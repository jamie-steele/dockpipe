// Package appserversupervisor owns the local lifecycle of one host-resident
// App Server child. It deliberately contains no provider protocol client.
package appserversupervisor

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"dorkpipe.orchestrator/providersession"
)

var (
	ErrAlreadyStarted  = errors.New("app server supervisor cannot restart a child")
	ErrNotStarted      = errors.New("app server supervisor has not started a child")
	ErrStartupDeadline = errors.New("app server startup deadline expired")
	ErrKillDeadline    = errors.New("app server child did not exit after kill deadline")
)

// DisconnectReason is a closed, safe classification. It never contains a
// child exit message, provider response, or transport payload.
type DisconnectReason string

const (
	DisconnectStartupFailure         DisconnectReason = "startup_failure"
	DisconnectStartupDeadline        DisconnectReason = "startup_deadline"
	DisconnectChildExit              DisconnectReason = "child_exit"
	DisconnectTransportClosed        DisconnectReason = "transport_closed"
	DisconnectMalformedInput         DisconnectReason = "malformed_transport"
	DisconnectLivenessDeadline       DisconnectReason = "liveness_deadline"
	DisconnectShutdown               DisconnectReason = "shutdown"
	DisconnectRequestDeadline        DisconnectReason = "request_deadline"
	DisconnectMalformedEnvelope      DisconnectReason = "malformed_envelope"
	DisconnectCorrelationMismatch    DisconnectReason = "correlation_mismatch"
	DisconnectProviderError          DisconnectReason = "provider_error"
	DisconnectInitializationRejected DisconnectReason = "initialization_rejected"
	DisconnectUnsupportedSchema      DisconnectReason = "unsupported_schema"
	DisconnectUnsupportedCapability  DisconnectReason = "unsupported_capability"
	DisconnectModelRerouted          DisconnectReason = "model_rerouted"
	DisconnectPolicyMismatch         DisconnectReason = "policy_mismatch"
	DisconnectLifecycleRejected      DisconnectReason = "lifecycle_rejected"
	DisconnectUnsupportedLifecycle   DisconnectReason = "unsupported_lifecycle_state"
	DisconnectUnsupportedEvent       DisconnectReason = "unsupported_event"
	DisconnectEventOrdering          DisconnectReason = "event_ordering"
	DisconnectDecisionRejected       DisconnectReason = "decision_rejected"
	DisconnectCancellationRejected   DisconnectReason = "cancellation_rejected"
)

// Deadlines bound the supervisor itself. They are not a substitute for future
// native workspace-write, writable-root, network, or review policy on turns.
type Deadlines struct {
	Startup  time.Duration
	Shutdown time.Duration
	Kill     time.Duration
	Liveness time.Duration
	Request  time.Duration
}

func (d Deadlines) validate() error {
	if d.Startup <= 0 || d.Shutdown <= 0 || d.Kill <= 0 || d.Liveness <= 0 || d.Request <= 0 {
		return errors.New("startup, shutdown, kill, liveness, and request deadlines must be positive")
	}
	return nil
}

// Child is the sole process instance owned by a Supervisor. Its streams remain
// private: CAS-03 only observes JSONL framing and never exposes or stores data.
type Child interface {
	Stdin() io.WriteCloser
	Stdout() io.ReadCloser
	Wait() error
	Kill() error
}

// Launcher starts a direct host child. It must honor ctx during startup and
// must not start a listener, shell, or fallback process.
type Launcher interface {
	Start(context.Context) (Child, error)
}

// HostLauncher starts one executable directly with inherited host placement.
// It deliberately does not invoke a shell and discards stderr rather than
// retaining provider output.
type HostLauncher struct {
	Executable string
	Args       []string
}

func (l HostLauncher) Start(ctx context.Context) (Child, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if strings.TrimSpace(l.Executable) == "" {
		return nil, errors.New("host executable is required")
	}
	if isShell(filepath.Base(l.Executable)) {
		return nil, errors.New("shell launch is forbidden")
	}
	if len(l.Args) != 2 || l.Args[0] != "app-server" || l.Args[1] != "--stdio" {
		return nil, errors.New("only direct app-server stdio launch is permitted")
	}
	cmd := exec.Command(l.Executable, l.Args...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		_ = stdin.Close()
		return nil, err
	}
	cmd.Stderr = io.Discard
	if err := cmd.Start(); err != nil {
		_ = stdin.Close()
		_ = stdout.Close()
		return nil, err
	}
	return commandChild{cmd: cmd, stdin: stdin, stdout: stdout}, nil
}

func isShell(executable string) bool {
	switch strings.ToLower(strings.TrimSpace(executable)) {
	case "bash", "bash.exe", "cmd", "cmd.exe", "powershell", "powershell.exe", "pwsh", "pwsh.exe", "sh", "sh.exe":
		return true
	default:
		return false
	}
}

type commandChild struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser
}

func (c commandChild) Stdin() io.WriteCloser { return c.stdin }
func (c commandChild) Stdout() io.ReadCloser { return c.stdout }
func (c commandChild) Wait() error           { return c.cmd.Wait() }
func (c commandChild) Kill() error           { return c.cmd.Process.Kill() }

// ShutdownRecord records bounded graceful shutdown and kill escalation without
// preserving child output or protocol data.
type ShutdownRecord struct {
	RequestedAt     time.Time
	GraceExpiredAt  time.Time
	KillRequestedAt time.Time
	ExitedAt        time.Time
	Forced          bool
}

type launchResult struct {
	child Child
	err   error
}

// Supervisor projects its process lifecycle as provider-neutral state events.
// It owns exactly one child and never retries, resumes, replays, or falls back.
type Supervisor struct {
	session        providersession.SessionRef
	launcher       Launcher
	deadlines      Deadlines
	initialization InitializationConfig
	events         chan providersession.Event

	mu                 sync.RWMutex
	lifecycleMu        sync.Mutex
	started            bool
	initialized        bool
	state              providersession.State
	sequence           uint64
	child              Child
	stdin              io.WriteCloser
	client             *protocolClient
	initializationInfo InitializationInfo
	processRef         string
	connectionRef      string
	lifecycle          lifecycleState
	waitDone           chan struct{}
	record             ShutdownRecord

	disconnectOnce sync.Once
	shutdownOnce   sync.Once
	shutdownDone   chan struct{}
	shutdownErr    error
}

var supervisorReferences atomic.Uint64

// New constructs a supervisor for an opaque session reference supplied by a
// future adapter. No provider session/thread lifecycle is started here.
func New(session providersession.SessionRef, launcher Launcher, deadlines Deadlines, initialization InitializationConfig) (*Supervisor, error) {
	if err := session.Validate(); err != nil {
		return nil, err
	}
	if launcher == nil {
		return nil, errors.New("host launcher is required")
	}
	if err := deadlines.validate(); err != nil {
		return nil, err
	}
	if err := initialization.validate(); err != nil {
		return nil, err
	}
	reference := supervisorReferences.Add(1)
	return &Supervisor{
		session:        session,
		launcher:       launcher,
		deadlines:      deadlines,
		initialization: initialization,
		state:          providersession.StateReady,
		events:         make(chan providersession.Event, 16),
		processRef:     "process-" + strconv.FormatUint(reference, 10),
		connectionRef:  "connection-" + strconv.FormatUint(reference, 10),
		shutdownDone:   make(chan struct{}),
	}, nil
}

// Events contains state projections only. It carries no raw child data.
func (s *Supervisor) Events() <-chan providersession.Event { return s.events }

func (s *Supervisor) State() providersession.State {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.state
}

func (s *Supervisor) ShutdownRecord() ShutdownRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.record
}

// Initialization returns the bounded, allow-listed initialization projection.
// It is empty until the supervisor has completed initialize/initialized.
func (s *Supervisor) Initialization() InitializationInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.initializationInfo
}

// Start launches one child under a bounded startup context. A later Start is
// rejected, including after failure, so no active work can be replayed.
func (s *Supervisor) Start(ctx context.Context) error {
	s.mu.Lock()
	if s.started || s.state == providersession.StateDisconnected {
		s.mu.Unlock()
		return ErrAlreadyStarted
	}
	s.started = true
	s.mu.Unlock()

	startupCtx, cancel := context.WithTimeout(ctx, s.deadlines.Startup)
	defer cancel()
	launched := make(chan launchResult, 1)
	go func() {
		child, err := s.launcher.Start(startupCtx)
		launched <- launchResult{child: child, err: err}
	}()

	select {
	case result := <-launched:
		if result.err != nil || result.child == nil {
			s.disconnect(DisconnectStartupFailure)
			if result.err != nil {
				return fmt.Errorf("start app server child: %w", result.err)
			}
			return errors.New("start app server child: launcher returned no child")
		}
		stdin, stdout := result.child.Stdin(), result.child.Stdout()
		if stdin == nil || stdout == nil {
			if stdin != nil {
				_ = stdin.Close()
			}
			_ = result.child.Kill()
			s.disconnect(DisconnectStartupFailure)
			return errors.New("start app server child: private stdio is required")
		}
		s.mu.Lock()
		s.child = result.child
		s.stdin = stdin
		s.waitDone = make(chan struct{})
		waitDone := s.waitDone
		s.mu.Unlock()
		client := newProtocolClientWithHandlers(stdin, stdout, s.deadlines.Liveness, s.fail, s.handleNotification, s.handleServerRequest)
		s.mu.Lock()
		s.client = client
		s.mu.Unlock()
		go s.waitForChild(result.child, waitDone)
		info, err := client.initialize(ctx, s.deadlines.Request, s.initialization)
		if err != nil {
			s.fail(client.failureReason())
			return errors.New("app server initialization did not complete")
		}
		s.mu.Lock()
		if s.state == providersession.StateDisconnected {
			s.mu.Unlock()
			return errors.New("app server disconnected during initialization")
		}
		s.initializationInfo = info
		s.initialized = true
		s.mu.Unlock()
		s.emit(providersession.StateReady, "initialized")
		return nil
	case <-startupCtx.Done():
		go reapLateLaunch(launched)
		s.disconnect(DisconnectStartupDeadline)
		return ErrStartupDeadline
	}
}

func reapLateLaunch(launched <-chan launchResult) {
	result := <-launched
	if result.child == nil {
		return
	}
	if stdin := result.child.Stdin(); stdin != nil {
		_ = stdin.Close()
	}
	_ = result.child.Kill()
	_ = result.child.Wait()
}

func (s *Supervisor) waitForChild(child Child, waitDone chan struct{}) {
	_ = child.Wait()
	s.mu.Lock()
	if s.record.ExitedAt.IsZero() {
		s.record.ExitedAt = time.Now().UTC()
	}
	s.mu.Unlock()
	close(waitDone)
	s.fail(DisconnectChildExit)
}

func (s *Supervisor) fail(reason DisconnectReason) {
	s.disconnect(reason)
	s.startShutdown()
}

// Shutdown immediately projects Disconnected, closes private stdin, then uses
// the configured grace and kill deadlines. It never sends a provider request.
func (s *Supervisor) Shutdown(ctx context.Context) error {
	s.mu.RLock()
	started := s.started
	s.mu.RUnlock()
	if !started {
		return ErrNotStarted
	}
	s.disconnect(DisconnectShutdown)
	s.startShutdown()
	select {
	case <-s.shutdownDone:
		s.mu.RLock()
		defer s.mu.RUnlock()
		return s.shutdownErr
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *Supervisor) disconnect(reason DisconnectReason) {
	s.disconnectOnce.Do(func() {
		s.mu.Lock()
		s.state = providersession.StateDisconnected
		s.sequence++
		event := providersession.Event{ContractVersion: providersession.ContractVersion, Sequence: s.sequence, OccurredAt: time.Now().UTC(), Session: s.session, Kind: providersession.EventStateChanged, State: providersession.StateDisconnected, Summary: string(reason)}
		s.mu.Unlock()
		s.events <- event
	})
}

func (s *Supervisor) emit(state providersession.State, summary string) {
	s.mu.Lock()
	if s.state == providersession.StateDisconnected {
		s.mu.Unlock()
		return
	}
	s.state = state
	s.sequence++
	event := providersession.Event{ContractVersion: providersession.ContractVersion, Sequence: s.sequence, OccurredAt: time.Now().UTC(), Session: s.session, Kind: providersession.EventStateChanged, State: state, Summary: summary}
	s.mu.Unlock()
	s.events <- event
}

// emitProgress projects a bounded, provider-neutral event. Its correlation is
// composed only from supervisor-owned process/connection references and the
// opaque lifecycle identifiers already held privately by the supervisor.
func (s *Supervisor) emitProgress(summary string, correlation providersession.Correlation) {
	s.mu.Lock()
	if s.state == providersession.StateDisconnected {
		s.mu.Unlock()
		return
	}
	s.sequence++
	event := providersession.Event{ContractVersion: providersession.ContractVersion, Sequence: s.sequence, OccurredAt: time.Now().UTC(), Session: s.session, Kind: providersession.EventProgress, Correlation: correlation, Summary: summary}
	s.mu.Unlock()
	s.events <- event
}

func (s *Supervisor) startShutdown() {
	s.shutdownOnce.Do(func() {
		go func() {
			s.mu.Lock()
			child, stdin, waitDone := s.child, s.stdin, s.waitDone
			if child == nil || waitDone == nil {
				s.shutdownErr = nil
				s.mu.Unlock()
				close(s.shutdownDone)
				return
			}
			s.record.RequestedAt = time.Now().UTC()
			s.mu.Unlock()

			if stdin != nil {
				_ = stdin.Close()
			}
			grace := time.NewTimer(s.deadlines.Shutdown)
			defer grace.Stop()
			select {
			case <-waitDone:
				s.finishShutdown(nil)
				return
			case <-grace.C:
			}

			s.mu.Lock()
			s.record.GraceExpiredAt = time.Now().UTC()
			s.record.KillRequestedAt = s.record.GraceExpiredAt
			s.record.Forced = true
			s.mu.Unlock()
			_ = child.Kill()
			kill := time.NewTimer(s.deadlines.Kill)
			defer kill.Stop()
			select {
			case <-waitDone:
				s.finishShutdown(nil)
			case <-kill.C:
				s.finishShutdown(ErrKillDeadline)
			}
		}()
	})
}

func (s *Supervisor) finishShutdown(err error) {
	s.mu.Lock()
	s.shutdownErr = err
	s.mu.Unlock()
	close(s.shutdownDone)
}
