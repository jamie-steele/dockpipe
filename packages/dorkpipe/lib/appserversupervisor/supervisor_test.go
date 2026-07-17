package appserversupervisor

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strconv"
	"sync"
	"testing"
	"time"

	"dorkpipe.orchestrator/providersession"
)

type fakeLauncher struct {
	start func(context.Context) (Child, error)
}

func (l fakeLauncher) Start(ctx context.Context) (Child, error) { return l.start(ctx) }
func (l fakeLauncher) validateLaunch() error                    { return nil }

type fakeChild struct {
	stdinR  *io.PipeReader
	stdinW  *io.PipeWriter
	stdoutR *io.PipeReader
	stdoutW *io.PipeWriter
	wait    chan struct{}

	mu        sync.Mutex
	waitErr   error
	killCalls int
}

func newFakeChild() *fakeChild {
	stdinR, stdinW := io.Pipe()
	stdoutR, stdoutW := io.Pipe()
	return &fakeChild{stdinR: stdinR, stdinW: stdinW, stdoutR: stdoutR, stdoutW: stdoutW, wait: make(chan struct{})}
}

func (c *fakeChild) Stdin() io.WriteCloser { return c.stdinW }
func (c *fakeChild) Stdout() io.ReadCloser { return c.stdoutR }
func (c *fakeChild) Wait() error {
	<-c.wait
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.waitErr
}
func (c *fakeChild) Kill() error {
	c.mu.Lock()
	c.killCalls++
	c.mu.Unlock()
	c.exit(errors.New("killed"))
	_ = c.stdoutW.Close()
	return nil
}
func (c *fakeChild) exit(err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	select {
	case <-c.wait:
		return
	default:
		c.waitErr = err
		close(c.wait)
	}
}
func (c *fakeChild) killed() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.killCalls
}

func testDeadlines() Deadlines {
	return Deadlines{Startup: time.Second, Shutdown: 30 * time.Millisecond, Kill: 30 * time.Millisecond, Liveness: time.Second, Request: time.Second}
}

func testInitialization() InitializationConfig {
	return InitializationConfig{SchemaVersion: "v2", RequiredCapabilities: []string{"stableV2"}, ClientName: "dockpipe-test", ClientVersion: "1.0.0", Model: PinnedModel, ReasoningEffort: PinnedReasoningEffort}
}

func newTestSupervisor(t *testing.T, launcher Launcher, deadlines Deadlines) *Supervisor {
	t.Helper()
	s, err := New(providersession.SessionRef{Provider: "test", SessionID: "session"}, launcher, deadlines, testInitialization())
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func startInitialized(t *testing.T, s *Supervisor, child *fakeChild, result string) error {
	t.Helper()
	started := make(chan error, 1)
	go func() { started <- s.Start(context.Background()) }()
	scanner := bufio.NewScanner(child.stdinR)
	if !scanner.Scan() {
		t.Fatal("expected initialize request")
	}
	var request struct {
		ID     uint64 `json:"id"`
		Method string `json:"method"`
		Params struct {
			Capabilities struct {
				OptOutNotificationMethods []string `json:"optOutNotificationMethods"`
			} `json:"capabilities"`
		} `json:"params"`
	}
	if err := json.Unmarshal(scanner.Bytes(), &request); err != nil || request.ID == 0 || request.Method != "initialize" {
		t.Fatalf("initialize request = %s, err=%v", scanner.Text(), err)
	}
	if len(request.Params.Capabilities.OptOutNotificationMethods) != 8 || request.Params.Capabilities.OptOutNotificationMethods[0] != remoteControlStatusNotification || request.Params.Capabilities.OptOutNotificationMethods[1] != threadStartedNotification || request.Params.Capabilities.OptOutNotificationMethods[2] != turnStartedNotification || request.Params.Capabilities.OptOutNotificationMethods[3] != threadSettingsNotification || request.Params.Capabilities.OptOutNotificationMethods[4] != mcpStartupNotification || request.Params.Capabilities.OptOutNotificationMethods[5] != mcpOAuthNotification || request.Params.Capabilities.OptOutNotificationMethods[6] != globalWarningNotification || request.Params.Capabilities.OptOutNotificationMethods[7] != accountRateLimitsNotification {
		t.Fatalf("initialize notification opt-out = %+v", request.Params.Capabilities.OptOutNotificationMethods)
	}
	if result == "" {
		result = `{"userAgent":"codex/0.144.1","codexHome":"C:/codex","platformFamily":"windows","platformOs":"windows"}`
	}
	if _, err := child.stdoutW.Write([]byte(`{"jsonrpc":"2.0","id":` + strconv.FormatUint(request.ID, 10) + `,"result":` + result + "}\n")); err != nil {
		t.Fatal(err)
	}
	if !scanner.Scan() {
		t.Fatal("expected initialized notification")
	}
	var notification struct {
		Method string `json:"method"`
	}
	if err := json.Unmarshal(scanner.Bytes(), &notification); err != nil || notification.Method != "initialized" {
		t.Fatalf("initialized notification = %s, err=%v", scanner.Text(), err)
	}
	return <-started
}

func nextEvent(t *testing.T, s *Supervisor) providersession.Event {
	t.Helper()
	select {
	case event := <-s.Events():
		if err := event.Validate(); err != nil {
			t.Fatalf("invalid projected event: %v", err)
		}
		return event
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for supervisor event")
		return providersession.Event{}
	}
}

func requireDisconnected(t *testing.T, s *Supervisor, reason DisconnectReason) {
	t.Helper()
	if ready := nextEvent(t, s); ready.State != providersession.StateReady {
		t.Fatalf("first event state = %s, want ready", ready.State)
	}
	event := nextEvent(t, s)
	if event.State != providersession.StateDisconnected || event.Summary != string(reason) {
		t.Fatalf("disconnect event = %+v, want %q", event, reason)
	}
	if s.State() != providersession.StateDisconnected {
		t.Fatalf("state = %s, want disconnected", s.State())
	}
}

func TestStartupFailureProjectsDisconnected(t *testing.T) {
	s := newTestSupervisor(t, fakeLauncher{start: func(context.Context) (Child, error) { return nil, errors.New("unavailable") }}, testDeadlines())
	if err := s.Start(context.Background()); err == nil {
		t.Fatal("expected startup error")
	}
	event := nextEvent(t, s)
	if event.State != providersession.StateDisconnected || event.Summary != string(DisconnectStartupFailure) {
		t.Fatalf("startup event = %+v", event)
	}
}

func TestHostLauncherRejectsShellAndNonStdioCommands(t *testing.T) {
	for _, launcher := range []HostLauncher{
		{Executable: "cmd.exe", Args: []string{"app-server", "--stdio"}},
		{Executable: "codex", Args: []string{"app-server"}},
		{Executable: "codex", Args: []string{"app-server", "--stdio", "--extra"}},
	} {
		if _, err := launcher.Start(context.Background()); err == nil {
			t.Fatalf("unsafe host launcher was accepted: %+v", launcher)
		}
	}
}

func TestStartupDeadlineProjectsDisconnected(t *testing.T) {
	release := make(chan struct{})
	child := newFakeChild()
	deadlines := testDeadlines()
	deadlines.Startup = 20 * time.Millisecond
	s := newTestSupervisor(t, fakeLauncher{start: func(context.Context) (Child, error) {
		<-release
		return child, nil
	}}, deadlines)
	if err := s.Start(context.Background()); !errors.Is(err, ErrStartupDeadline) {
		t.Fatalf("startup error = %v, want %v", err, ErrStartupDeadline)
	}
	event := nextEvent(t, s)
	if event.State != providersession.StateDisconnected || event.Summary != string(DisconnectStartupDeadline) {
		t.Fatalf("startup deadline event = %+v", event)
	}
	close(release)
}

func TestCleanShutdownClosesPrivateStdinWithoutKill(t *testing.T) {
	child := newFakeChild()
	s := newTestSupervisor(t, fakeLauncher{start: func(context.Context) (Child, error) { return child, nil }}, testDeadlines())
	if err := startInitialized(t, s, child, ""); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- s.Shutdown(context.Background()) }()
	if _, err := io.ReadAll(child.stdinR); err != nil {
		t.Fatal(err)
	}
	child.exit(nil)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	requireDisconnected(t, s, DisconnectShutdown)
	record := s.ShutdownRecord()
	if record.RequestedAt.IsZero() || record.ExitedAt.IsZero() || record.Forced || child.killed() != 0 {
		t.Fatalf("unexpected clean shutdown record: %+v, kills=%d", record, child.killed())
	}
}

func TestForcedChildDeathProjectsDisconnected(t *testing.T) {
	child := newFakeChild()
	s := newTestSupervisor(t, fakeLauncher{start: func(context.Context) (Child, error) { return child, nil }}, testDeadlines())
	if err := startInitialized(t, s, child, ""); err != nil {
		t.Fatal(err)
	}
	child.exit(errors.New("died"))
	requireDisconnected(t, s, DisconnectChildExit)
}

func TestClosedStdoutProjectsDisconnected(t *testing.T) {
	child := newFakeChild()
	s := newTestSupervisor(t, fakeLauncher{start: func(context.Context) (Child, error) { return child, nil }}, testDeadlines())
	if err := startInitialized(t, s, child, ""); err != nil {
		t.Fatal(err)
	}
	_ = child.stdoutW.Close()
	requireDisconnected(t, s, DisconnectTransportClosed)
}

func TestLivenessDeadlineProjectsDisconnectedAndKillsChild(t *testing.T) {
	child := newFakeChild()
	deadlines := testDeadlines()
	deadlines.Liveness = 20 * time.Millisecond
	s := newTestSupervisor(t, fakeLauncher{start: func(context.Context) (Child, error) { return child, nil }}, deadlines)
	if err := startInitialized(t, s, child, ""); err != nil {
		t.Fatal(err)
	}
	requireDisconnected(t, s, DisconnectLivenessDeadline)
	deadline := time.After(time.Second)
	for child.killed() == 0 {
		select {
		case <-deadline:
			t.Fatal("liveness failure did not stop child")
		case <-time.After(time.Millisecond):
		}
	}
	record := s.ShutdownRecord()
	if !record.Forced || record.KillRequestedAt.IsZero() {
		t.Fatalf("kill escalation was not recorded: %+v", record)
	}
}

func TestMalformedStdoutProjectsDisconnected(t *testing.T) {
	child := newFakeChild()
	s := newTestSupervisor(t, fakeLauncher{start: func(context.Context) (Child, error) { return child, nil }}, testDeadlines())
	if err := startInitialized(t, s, child, ""); err != nil {
		t.Fatal(err)
	}
	_, _ = child.stdoutW.Write([]byte("not-json\n"))
	requireDisconnected(t, s, DisconnectMalformedEnvelope)
}

func TestNoReplayOrResumeAfterDisconnect(t *testing.T) {
	child := newFakeChild()
	s := newTestSupervisor(t, fakeLauncher{start: func(context.Context) (Child, error) { return child, nil }}, testDeadlines())
	if err := startInitialized(t, s, child, ""); err != nil {
		t.Fatal(err)
	}
	child.exit(errors.New("died"))
	requireDisconnected(t, s, DisconnectChildExit)
	if err := s.Start(context.Background()); !errors.Is(err, ErrAlreadyStarted) {
		t.Fatalf("restart error = %v, want %v", err, ErrAlreadyStarted)
	}
	select {
	case event := <-s.Events():
		t.Fatalf("unexpected replay event: %+v", event)
	case <-time.After(30 * time.Millisecond):
	}
}
