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

const (
	hostPipeWaitDelay          = 5 * time.Second
	childExitObservationWindow = 25 * time.Millisecond
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
	DisconnectPersistenceFailure     DisconnectReason = "persistence_failure"
	DisconnectAuditFailure           DisconnectReason = "audit_failure"
	DisconnectUnsafeConfiguration    DisconnectReason = "unsafe_configuration"
	DisconnectTransportOwnership     DisconnectReason = "transport_ownership"
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
	for _, value := range []time.Duration{d.Startup, d.Shutdown, d.Kill, d.Liveness, d.Request} {
		if value > maxSupervisorDeadline {
			return errors.New("supervisor deadlines must be bounded")
		}
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
	validateLaunch() error
}

// HostLauncher starts one executable directly with inherited host placement.
// It deliberately does not invoke a shell and discards stderr rather than
// retaining provider output.
type HostLauncher struct {
	Executable string
	Args       []string
}

func (l HostLauncher) validateLaunch() error {
	if strings.TrimSpace(l.Executable) != l.Executable || l.Executable == "" || len(l.Executable) > maxLocalPathBytes {
		return errors.New("direct codex executable is required")
	}
	name := strings.ToLower(filepath.Base(l.Executable))
	if name != "codex" && name != "codex.exe" {
		return errors.New("only the direct codex app-server executable is permitted")
	}
	if isShell(name) || len(l.Args) != 2 || l.Args[0] != "app-server" || l.Args[1] != "--stdio" {
		return errors.New("only direct app-server stdio launch is permitted")
	}
	return nil
}

func (l HostLauncher) Start(ctx context.Context) (Child, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := l.validateLaunch(); err != nil {
		return nil, err
	}
	cmd := exec.Command(l.Executable, l.Args...)
	// Bound Wait when a command shim exits but leaves a descendant holding the
	// private stdio handles. This affects only post-exit pipe drainage.
	cmd.WaitDelay = hostPipeWaitDelay
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
	store          SnapshotStore
	audit          *auditJournal

	mu                 sync.RWMutex
	lifecycleMu        sync.Mutex
	started            bool
	initialized        bool
	state              providersession.State
	sequence           uint64
	child              Child
	stdin              io.WriteCloser
	stdout             io.ReadCloser
	client             *protocolClient
	initializationInfo InitializationInfo
	processRef         string
	connectionRef      string
	recoveryEvidence   string
	lifecycle          lifecycleState
	lastNotification   string
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
	return NewWithStores(session, launcher, deadlines, initialization, nil, nil)
}

// NewWithSnapshotStore keeps CAS-09 recovery state owned by this package. The
// supplied store receives only bounded snapshot bytes produced below; it never
// receives protocol frames or provider payloads.
func NewWithSnapshotStore(session providersession.SessionRef, launcher Launcher, deadlines Deadlines, initialization InitializationConfig, store SnapshotStore) (*Supervisor, error) {
	return NewWithStores(session, launcher, deadlines, initialization, store, nil)
}

// NewWithStores keeps both CAS-09 recovery and CAS-10 audit evidence inside
// this package. AuditStore receives only bounded safe projections.
func NewWithStores(session providersession.SessionRef, launcher Launcher, deadlines Deadlines, initialization InitializationConfig, store SnapshotStore, auditStore AuditStore) (*Supervisor, error) {
	if err := validateSupervisorSession(session); err != nil {
		return nil, err
	}
	if launcher == nil {
		return nil, errors.New("host launcher is required")
	}
	if err := launcher.validateLaunch(); err != nil {
		return nil, err
	}
	if err := deadlines.validate(); err != nil {
		return nil, err
	}
	if err := initialization.validate(); err != nil {
		return nil, err
	}
	reference := supervisorReferences.Add(1)
	recoveryEvidence := "recovery-" + strconv.FormatUint(reference, 10)
	return &Supervisor{
		session:          session,
		launcher:         launcher,
		deadlines:        deadlines,
		initialization:   initialization,
		store:            store,
		audit:            newAuditJournal(session, recoveryEvidence, auditStore),
		state:            providersession.StateReady,
		events:           make(chan providersession.Event, 16),
		processRef:       "process-" + strconv.FormatUint(reference, 10),
		connectionRef:    "connection-" + strconv.FormatUint(reference, 10),
		recoveryEvidence: recoveryEvidence,
		shutdownDone:     make(chan struct{}),
	}, nil
}

func (s *Supervisor) auditOperation(operation, outcome, lifecycle, summary string, correlation providersession.Correlation, started time.Time) bool {
	if s.audit == nil {
		s.auditFailure()
		return false
	}
	s.mu.RLock()
	record := AuditRecord{Version: auditSchemaVersion, Operation: operation, Outcome: outcome, Lifecycle: lifecycle, Summary: summary, Session: s.session, Correlation: correlation, Progress: auditProgress(s.sequence), Latency: auditLatency(started)}
	s.mu.RUnlock()
	if err := s.audit.append(context.Background(), record); err != nil {
		s.auditFailure()
		return false
	}
	return true
}

func (s *Supervisor) publish(event providersession.Event, operation, outcome, lifecycle string) bool {
	if !s.auditEvent(event, operation, outcome, lifecycle) {
		return false
	}
	s.events <- event
	return true
}

func (s *Supervisor) auditEvent(event providersession.Event, operation, outcome, lifecycle string) bool {
	if s.audit == nil {
		s.auditFailure()
		return false
	}
	if err := s.audit.append(context.Background(), AuditRecord{Version: auditSchemaVersion, EventSequence: event.Sequence, Operation: operation, Outcome: outcome, Lifecycle: lifecycle, Summary: event.Summary, Session: event.Session, Correlation: event.Correlation, Progress: auditProgress(event.Sequence), Latency: "none"}); err != nil {
		s.auditFailure()
		return false
	}
	return true
}

func (s *Supervisor) auditFailure() {
	s.mu.Lock()
	if s.state == providersession.StateDisconnected {
		s.mu.Unlock()
		return
	}
	s.state, s.sequence = providersession.StateDisconnected, s.sequence+1
	event := providersession.Event{ContractVersion: providersession.ContractVersion, Sequence: s.sequence, OccurredAt: nowUTC(), Session: s.session, Kind: providersession.EventStateChanged, State: providersession.StateDisconnected, Summary: string(DisconnectAuditFailure)}
	s.mu.Unlock()
	s.events <- event
	s.startShutdown()
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

// RecoveryEvidence is an opaque reference only. It is usable after a safe
// idle snapshot has been committed; it carries no session content itself.
func (s *Supervisor) RecoveryEvidence() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.recoveryEvidence
}

// Start launches one child under a bounded startup context. A later Start is
// rejected, including after failure, so no active work can be replayed.
func (s *Supervisor) Start(ctx context.Context) error {
	return s.start(ctx, true)
}

func (s *Supervisor) start(ctx context.Context, emitReady bool) error {
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
		s.child, s.stdin, s.stdout = result.child, stdin, stdout
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
		if emitReady {
			if !s.emit(providersession.StateReady, "initialized", "initialization", "completed") {
				return errors.New("app server audit journal failed during initialization")
			}
		}
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
	if reason == DisconnectTransportClosed && s.childExitedWithinObservationWindow() {
		reason = DisconnectChildExit
	}
	s.disconnect(reason)
	s.startShutdown()
}

// childExitedWithinObservationWindow distinguishes the owned process exiting
// from a still-live child whose stdout alone closed. It observes only the
// existing Wait completion signal and never reads child output or retries.
func (s *Supervisor) childExitedWithinObservationWindow() bool {
	s.mu.RLock()
	waitDone := s.waitDone
	s.mu.RUnlock()
	if waitDone == nil {
		return false
	}
	select {
	case <-waitDone:
		return true
	case <-time.After(childExitObservationWindow):
		return false
	}
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
		stdin, stdout := s.stdin, s.stdout
		s.stdin, s.stdout = nil, nil
		s.client = nil
		s.clearPrivateStateLocked()
		s.state = providersession.StateDisconnected
		s.sequence++
		event := providersession.Event{ContractVersion: providersession.ContractVersion, Sequence: s.sequence, OccurredAt: time.Now().UTC(), Session: s.session, Kind: providersession.EventStateChanged, State: providersession.StateDisconnected, Summary: string(reason)}
		s.mu.Unlock()
		if stdin != nil {
			_ = stdin.Close()
		}
		if stdout != nil {
			_ = stdout.Close()
		}
		s.publish(event, "disconnect", "failed", "disconnected")
	})
}

func (s *Supervisor) clearPrivateStateLocked() {
	if s.lifecycle.pending != nil && s.lifecycle.pending.timer != nil {
		s.lifecycle.pending.timer.Stop()
	}
	if s.lifecycle.cancellation != nil && s.lifecycle.cancellation.timer != nil {
		s.lifecycle.cancellation.timer.Stop()
	}
	s.lifecycle = lifecycleState{}
}

func (s *Supervisor) emit(state providersession.State, summary, operation, outcome string) bool {
	s.mu.Lock()
	if s.state == providersession.StateDisconnected {
		s.mu.Unlock()
		return false
	}
	s.state = state
	s.sequence++
	event := providersession.Event{ContractVersion: providersession.ContractVersion, Sequence: s.sequence, OccurredAt: time.Now().UTC(), Session: s.session, Kind: providersession.EventStateChanged, State: state, Summary: summary}
	s.mu.Unlock()
	return s.publish(event, operation, outcome, auditLifecycle(state))
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
	s.publish(event, "event", "completed", auditLifecycle(s.State()))
}

func auditLifecycle(state providersession.State) string {
	switch state {
	case providersession.StateReady:
		return "ready"
	case providersession.StateRunning:
		return "running"
	case providersession.StateWaitingForApproval, providersession.StateWaitingForUserInput:
		return "waiting"
	case providersession.StateCompleted, providersession.StateCancelled, providersession.StateFailed:
		return "terminal"
	case providersession.StateDisconnected:
		return "disconnected"
	default:
		return "idle"
	}
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
