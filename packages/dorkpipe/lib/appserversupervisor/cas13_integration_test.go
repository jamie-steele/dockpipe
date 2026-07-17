//go:build cas13

package appserversupervisor

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"dorkpipe.orchestrator/providersession"
)

// cas13Launcher uses the existing authenticated direct App Server command.
func cas13Launcher() HostLauncher {
	executable := os.Getenv("DOCKPIPE_CAS13_EXECUTABLE")
	if executable == "" {
		executable = "codex"
	}
	return HostLauncher{Executable: executable, Args: []string{"app-server", "--stdio"}}
}

// TestCAS13ControlledCodex is deliberately excluded from ordinary tests. It
// accepts only the CAS-11 direct child and writes no evidence: callers may
// retain only their redacted outcome classification outside this process.
func TestCAS13ControlledCodex(t *testing.T) {
	if os.Getenv("DOCKPIPE_CAS13") != "run" {
		t.Skip("set DOCKPIPE_CAS13=run for the reviewed controlled integration")
	}
	workspace := os.Getenv("DOCKPIPE_CAS13_WORKSPACE")
	if !filepath.IsAbs(workspace) {
		t.Fatal("CAS-13 requires an absolute declared workspace")
	}
	if filepath.Clean(workspace) != workspace {
		t.Fatal("CAS-13 workspace must be clean")
	}
	deadlines := Deadlines{Startup: time.Minute, Shutdown: 20 * time.Second, Kill: 10 * time.Second, Liveness: time.Minute, Request: time.Minute}
	initialization := InitializationConfig{SchemaVersion: "v2", RequiredCapabilities: []string{"stableV2"}, ClientName: "dockpipe-cas13", ClientVersion: "0.1.0", Model: PinnedModel, ReasoningEffort: PinnedReasoningEffort}
	s, err := New(providersession.SessionRef{Provider: "codex", SessionID: "cas13"}, cas13Launcher(), deadlines, initialization)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	if err := s.Start(ctx); err != nil {
		t.Fatalf("CAS-13 initialization failed: %s", cas13FailureClass(s))
	}
	defer func() {
		if err := s.Shutdown(context.Background()); err != nil {
			t.Errorf("CAS-13 clean shutdown failed: %v", err)
		}
		if s.ShutdownRecord().Forced {
			t.Error("CAS-13 clean shutdown required a forced kill")
		}
	}()
	if err := verifyCAS13Catalog(ctx, s); err != nil {
		t.Fatal(err)
	}
	policy := LifecyclePolicy{Workspace: workspace, WritableRoots: []string{workspace}, Sandbox: "workspace-write", ApprovalPolicy: "untrusted", Reviewer: "user", Model: PinnedModel, ReasoningEffort: PinnedReasoningEffort, ModelProvider: "openai"}
	thread, err := s.StartThread(ctx, policy)
	if err != nil {
		t.Fatalf("CAS-13 thread lifecycle failed: %v", err)
	}
	if _, err := s.startTurn(ctx, thread, policy, cas13Prompt(t)); err != nil {
		t.Fatalf("CAS-13 turn lifecycle failed: %s", cas13FailureClass(s))
	}
	if terminal := waitCAS13Terminal(ctx, s); terminal != "turn_completed" {
		t.Fatalf("CAS-13 normalized terminal = %s", terminal)
	}
}

func TestCAS13FailedTurnReconcilesThread(t *testing.T) {
	if os.Getenv("DOCKPIPE_CAS13") != "run" {
		t.Skip("set DOCKPIPE_CAS13=run for the reviewed controlled integration")
	}
	workspace := os.Getenv("DOCKPIPE_CAS13_WORKSPACE")
	if !filepath.IsAbs(workspace) || filepath.Clean(workspace) != workspace {
		t.Fatal("CAS-13 requires a clean absolute declared workspace")
	}
	deadlines := Deadlines{Startup: time.Minute, Shutdown: 20 * time.Second, Kill: 10 * time.Second, Liveness: time.Minute, Request: time.Minute}
	initialization := InitializationConfig{SchemaVersion: "v2", RequiredCapabilities: []string{"stableV2"}, ClientName: "dockpipe-cas13", ClientVersion: "0.1.0", Model: PinnedModel, ReasoningEffort: PinnedReasoningEffort}
	s, err := New(providersession.SessionRef{Provider: "codex", SessionID: "cas13-reconcile"}, cas13Launcher(), deadlines, initialization)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	if err := s.Start(ctx); err != nil {
		t.Fatalf("CAS-13 reconciliation initialization failed: %s", cas13FailureClass(s))
	}
	defer func() {
		if err := s.Shutdown(context.Background()); err != nil {
			t.Errorf("CAS-13 reconciliation shutdown failed: %v", err)
		}
		if s.ShutdownRecord().Forced {
			t.Error("CAS-13 reconciliation shutdown required a forced kill")
		}
	}()
	if err := verifyCAS13Catalog(ctx, s); err != nil {
		t.Fatal(err)
	}
	policy := LifecyclePolicy{Workspace: workspace, WritableRoots: []string{workspace}, Sandbox: "workspace-write", ApprovalPolicy: "untrusted", Reviewer: "user", Model: PinnedModel, ReasoningEffort: PinnedReasoningEffort, ModelProvider: "openai"}
	thread, err := s.StartThread(ctx, policy)
	if err != nil {
		t.Fatalf("CAS-13 reconciliation thread lifecycle failed: %v", err)
	}
	if _, err := s.startTurn(ctx, thread, policy, cas13Prompt(t)); err != nil {
		t.Fatalf("CAS-13 reconciliation turn lifecycle failed: %s", cas13FailureClass(s))
	}
	if terminal := waitCAS13Terminal(ctx, s); terminal != "turn_failed_other" {
		t.Fatalf("CAS-13 reconciliation terminal = %s", terminal)
	}
	if _, err := s.ReadThread(ctx, thread, policy); err != nil {
		t.Fatalf("CAS-13 failed-turn thread read = %s", cas13FailureClass(s))
	}
}

// TestCAS13ControlledCancellation uses the same constrained no-tool turn as
// the completion probe, then requests the existing neutral user cancellation.
// It is successful only on the exact correlated interrupted terminal state.
func TestCAS13ControlledCancellation(t *testing.T) {
	if os.Getenv("DOCKPIPE_CAS13") != "run" {
		t.Skip("set DOCKPIPE_CAS13=run for the reviewed controlled integration")
	}
	workspace := os.Getenv("DOCKPIPE_CAS13_WORKSPACE")
	if !filepath.IsAbs(workspace) || filepath.Clean(workspace) != workspace {
		t.Fatal("CAS-13 requires a clean absolute declared workspace")
	}
	deadlines := Deadlines{Startup: time.Minute, Shutdown: 20 * time.Second, Kill: 10 * time.Second, Liveness: time.Minute, Request: time.Minute}
	initialization := InitializationConfig{SchemaVersion: "v2", RequiredCapabilities: []string{"stableV2"}, ClientName: "dockpipe-cas13", ClientVersion: "0.1.0", Model: PinnedModel, ReasoningEffort: PinnedReasoningEffort}
	s, err := New(providersession.SessionRef{Provider: "codex", SessionID: "cas13-cancel"}, cas13Launcher(), deadlines, initialization)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	if err := s.Start(ctx); err != nil {
		t.Fatalf("CAS-13 cancellation initialization failed: %s", cas13FailureClass(s))
	}
	defer func() {
		if err := s.Shutdown(context.Background()); err != nil {
			t.Errorf("CAS-13 cancellation shutdown failed: %v", err)
		}
		if s.ShutdownRecord().Forced {
			t.Error("CAS-13 cancellation shutdown required a forced kill")
		}
	}()
	if err := verifyCAS13Catalog(ctx, s); err != nil {
		t.Fatal(err)
	}
	policy := LifecyclePolicy{Workspace: workspace, WritableRoots: []string{workspace}, Sandbox: "workspace-write", ApprovalPolicy: "untrusted", Reviewer: "user", Model: PinnedModel, ReasoningEffort: PinnedReasoningEffort, ModelProvider: "openai"}
	thread, err := s.StartThread(ctx, policy)
	if err != nil {
		t.Fatalf("CAS-13 cancellation thread lifecycle failed: %v", err)
	}
	turn, err := s.startTurn(ctx, thread, policy, cas13Prompt(t))
	if err != nil {
		t.Fatalf("CAS-13 cancellation turn lifecycle failed: %s", cas13FailureClass(s))
	}
	if outcome := waitCAS13ActiveItem(ctx, s); outcome != "item_started" {
		t.Fatalf("CAS-13 cancellation active item = %s", outcome)
	}
	if err := s.Cancel(ctx, cancellationIntent(turn)); err != nil {
		t.Fatalf("CAS-13 cancellation delivery failed: %s", cas13FailureClass(s))
	}
	if terminal := waitCAS13Cancellation(ctx, s); terminal != "cancelled" {
		t.Fatalf("CAS-13 cancellation terminal = %s", terminal)
	}
}

func TestCAS13ControlledApprovalDeny(t *testing.T) {
	if os.Getenv("DOCKPIPE_CAS13") != "run" {
		t.Skip("set DOCKPIPE_CAS13=run for the reviewed controlled integration")
	}
	workspace := os.Getenv("DOCKPIPE_CAS13_WORKSPACE")
	if !filepath.IsAbs(workspace) || filepath.Clean(workspace) != workspace {
		t.Fatal("CAS-13 requires a clean absolute declared workspace")
	}
	deadlines := Deadlines{Startup: time.Minute, Shutdown: 20 * time.Second, Kill: 10 * time.Second, Liveness: time.Minute, Request: time.Minute}
	initialization := InitializationConfig{SchemaVersion: "v2", RequiredCapabilities: []string{"stableV2"}, ClientName: "dockpipe-cas13", ClientVersion: "0.1.0", Model: PinnedModel, ReasoningEffort: PinnedReasoningEffort}
	s, err := New(providersession.SessionRef{Provider: "codex", SessionID: "cas13-approval"}, cas13Launcher(), deadlines, initialization)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	if err := s.Start(ctx); err != nil {
		t.Fatalf("CAS-13 approval initialization failed: %s", cas13FailureClass(s))
	}
	defer func() {
		if err := s.Shutdown(context.Background()); err != nil {
			t.Errorf("CAS-13 approval shutdown failed: %v", err)
		}
		if s.ShutdownRecord().Forced {
			t.Error("CAS-13 approval shutdown required a forced kill")
		}
	}()
	if err := verifyCAS13Catalog(ctx, s); err != nil {
		t.Fatal(err)
	}
	policy := LifecyclePolicy{Workspace: workspace, WritableRoots: []string{workspace}, Sandbox: "workspace-write", ApprovalPolicy: "untrusted", Reviewer: "user", Model: PinnedModel, ReasoningEffort: PinnedReasoningEffort, ModelProvider: "openai"}
	thread, err := s.StartThread(ctx, policy)
	if err != nil {
		t.Fatalf("CAS-13 approval thread lifecycle failed: %v", err)
	}
	if _, err := s.startTurn(ctx, thread, policy, cas13Prompt(t)); err != nil {
		t.Fatalf("CAS-13 approval turn lifecycle failed: %s", cas13FailureClass(s))
	}
	for {
		select {
		case event := <-s.Events():
			if event.Kind == providersession.EventApprovalRequested && event.Approval != nil {
				if err := s.Decide(ctx, providersession.ApprovalDecision{Correlation: event.Approval.Correlation, Decision: providersession.DecisionDeny}); err != nil {
					t.Fatalf("CAS-13 denial delivery failed: %s", cas13FailureClass(s))
				}
				if outcome := waitCAS13ApprovalResolution(ctx, s); outcome != "approval_resolved" {
					t.Fatalf("CAS-13 denial resolution = %s", outcome)
				}
				return
			}
			if event.Kind == providersession.EventProgress && event.Summary == "turn_failed" {
				t.Fatal("CAS-13 approval turn failed before an approval request")
			}
			if event.State == providersession.StateDisconnected {
				s.mu.RLock()
				last := s.lastNotification
				s.mu.RUnlock()
				if last != "" && last != "other" {
					t.Fatalf("CAS-13 approval request = %s_%s", event.Summary, last)
				}
				t.Fatalf("CAS-13 approval request = %s", event.Summary)
			}
		case <-ctx.Done():
			t.Fatal("CAS-13 approval request did not arrive")
		}
	}
}

func TestCAS13ControlledTransportLoss(t *testing.T) {
	if os.Getenv("DOCKPIPE_CAS13") != "run" {
		t.Skip("set DOCKPIPE_CAS13=run for the reviewed controlled integration")
	}
	deadlines := Deadlines{Startup: time.Minute, Shutdown: 20 * time.Second, Kill: 10 * time.Second, Liveness: time.Minute, Request: time.Minute}
	initialization := InitializationConfig{SchemaVersion: "v2", RequiredCapabilities: []string{"stableV2"}, ClientName: "dockpipe-cas13", ClientVersion: "0.1.0", Model: PinnedModel, ReasoningEffort: PinnedReasoningEffort}
	s, err := New(providersession.SessionRef{Provider: "codex", SessionID: "cas13-loss"}, cas13Launcher(), deadlines, initialization)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := s.Start(ctx); err != nil {
		t.Fatalf("CAS-13 transport-loss initialization failed: %s", cas13FailureClass(s))
	}
	s.mu.RLock()
	stdout := s.stdout
	s.mu.RUnlock()
	if stdout == nil || stdout.Close() != nil {
		t.Fatal("CAS-13 transport-loss close could not be delivered")
	}
	for {
		select {
		case event := <-s.Events():
			if event.State != providersession.StateDisconnected {
				continue
			}
			if event.Summary != string(DisconnectTransportClosed) {
				t.Fatalf("CAS-13 transport-loss class = %s", event.Summary)
			}
			return
		case <-ctx.Done():
			s.mu.RLock()
			state, waitDone := s.state, s.waitDone
			s.mu.RUnlock()
			waited := false
			if waitDone != nil {
				select {
				case <-waitDone:
					waited = true
				default:
				}
			}
			t.Fatalf("CAS-13 transport-loss had no bounded disconnect (state=%s, wait=%t)", state, waited)
		}
	}
}

// TestCAS13ControlledChildDeath kills only the direct child this test started
// and requires the process-exit path—not a synthetic stream close—to produce
// the fail-closed normalized disconnect.
func TestCAS13ControlledChildDeath(t *testing.T) {
	if os.Getenv("DOCKPIPE_CAS13") != "run" {
		t.Skip("set DOCKPIPE_CAS13=run for the reviewed controlled integration")
	}
	deadlines := Deadlines{Startup: time.Minute, Shutdown: 20 * time.Second, Kill: 10 * time.Second, Liveness: time.Minute, Request: time.Minute}
	initialization := InitializationConfig{SchemaVersion: "v2", RequiredCapabilities: []string{"stableV2"}, ClientName: "dockpipe-cas13", ClientVersion: "0.1.0", Model: PinnedModel, ReasoningEffort: PinnedReasoningEffort}
	s, err := New(providersession.SessionRef{Provider: "codex", SessionID: "cas13-child-death"}, cas13Launcher(), deadlines, initialization)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	if err := s.Start(ctx); err != nil {
		t.Fatalf("CAS-13 child-death initialization failed: %s", cas13FailureClass(s))
	}
	s.mu.RLock()
	child := s.child
	s.mu.RUnlock()
	if child == nil || child.Kill() != nil {
		t.Fatal("CAS-13 direct child could not be terminated")
	}
	for {
		select {
		case event := <-s.Events():
			if event.State != providersession.StateDisconnected {
				continue
			}
			if event.Summary != string(DisconnectChildExit) {
				t.Fatalf("CAS-13 child-death class = %s", event.Summary)
			}
			return
		case <-ctx.Done():
			t.Fatal("CAS-13 child death did not produce a bounded disconnect")
		}
	}
}

func waitCAS13Terminal(ctx context.Context, s *Supervisor) string {
	for {
		select {
		case event := <-s.Events():
			if event.State == providersession.StateDisconnected {
				s.mu.RLock()
				last := s.lastNotification
				s.mu.RUnlock()
				if last != "" && last != "other" {
					return event.Summary + "_" + last
				}
				return event.Summary
			}
			if event.Kind == providersession.EventProgress && event.Summary == "turn_failed" {
				s.mu.RLock()
				terminal := s.lastNotification
				s.mu.RUnlock()
				return terminal
			}
			if event.Kind == providersession.EventProgress && event.Summary == "turn_completed" {
				return event.Summary
			}
		case <-ctx.Done():
			s.mu.RLock()
			last := s.lastNotification
			s.mu.RUnlock()
			if last != "" && last != "other" {
				return "request_deadline_" + last
			}
			return "request_deadline"
		}
	}
}

func cas13Prompt(t *testing.T) []any {
	t.Helper()
	prompt := os.Getenv("DOCKPIPE_CAS13_PROMPT")
	if len(prompt) == 0 || len(prompt) > 256 || strings.TrimSpace(prompt) != prompt || strings.ContainsAny(prompt, "\r\n") {
		t.Fatal("CAS-13 requires one bounded single-line ephemeral prompt")
	}
	return []any{map[string]any{"type": "text", "text": prompt}}
}

func waitCAS13Cancellation(ctx context.Context, s *Supervisor) string {
	for {
		select {
		case event := <-s.Events():
			if event.State == providersession.StateCancelled && event.Summary == "cancelled" {
				return "cancelled"
			}
			if event.State == providersession.StateDisconnected {
				return event.Summary
			}
		case <-ctx.Done():
			return "request_deadline"
		}
	}
}

func waitCAS13ActiveItem(ctx context.Context, s *Supervisor) string {
	for {
		select {
		case event := <-s.Events():
			if event.Kind == providersession.EventProgress && event.Summary == "item_started" && event.Correlation.ActivityID != "" {
				s.mu.RLock()
				kind := s.lastNotification
				s.mu.RUnlock()
				if kind == "item_agent" || kind == "item_reasoning" {
					return "item_started"
				}
			}
			if event.State == providersession.StateDisconnected {
				return event.Summary
			}
		case <-ctx.Done():
			return "request_deadline"
		}
	}
}

func waitCAS13ApprovalResolution(ctx context.Context, s *Supervisor) string {
	for {
		select {
		case event := <-s.Events():
			if event.Summary == "approval_resolved" && event.State == providersession.StateRunning {
				return "approval_resolved"
			}
			if event.State == providersession.StateDisconnected {
				return event.Summary
			}
		case <-ctx.Done():
			return "request_deadline"
		}
	}
}

func cas13FailureClass(s *Supervisor) string {
	timer := time.NewTimer(time.Second)
	defer timer.Stop()
	for {
		select {
		case event := <-s.Events():
			if event.State == providersession.StateDisconnected && validAuditDisconnectReason(event.Summary) {
				s.mu.RLock()
				last := s.lastNotification
				s.mu.RUnlock()
				if last != "" && last != "other" {
					return event.Summary + "_" + last
				}
				return event.Summary
			}
		case <-timer.C:
			s.mu.RLock()
			last := s.lastNotification
			s.mu.RUnlock()
			if last != "" {
				return "initialization_rejected_" + last
			}
			return "initialization_rejected"
		}
	}
}

// TestCAS13InitializationEnvelopeShape is a redacted diagnostic for the
// existing direct-child gate. It reports only the first JSON-RPC envelope's
// structural class and discards its content immediately.
func TestCAS13InitializationEnvelopeShape(t *testing.T) {
	if os.Getenv("DOCKPIPE_CAS13_DIAGNOSTIC") != "run" {
		t.Skip("set DOCKPIPE_CAS13_DIAGNOSTIC=run for the reviewed shape diagnostic")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	child, err := cas13Launcher().Start(ctx)
	if err != nil {
		t.Fatal("CAS-13 direct child could not start")
	}
	defer func() {
		_ = child.Stdin().Close()
		_ = child.Kill()
		_ = child.Wait()
	}()
	if err := json.NewEncoder(child.Stdin()).Encode(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": map[string]any{"clientInfo": map[string]any{"name": "dockpipe-cas13", "version": "0.1.0"}, "capabilities": map[string]any{}}}); err != nil {
		t.Fatal("CAS-13 initialization request could not be sent")
	}
	scanner := bufio.NewScanner(child.Stdout())
	scanner.Buffer(make([]byte, 4096), 1<<20)
	if !scanner.Scan() {
		t.Fatal("CAS-13 initialization produced no envelope")
	}
	t.Logf("CAS-13 initialization envelope shape: %s", cas13EnvelopeShape(scanner.Bytes()))
	t.Logf("CAS-13 initialization result shape: %s", cas13InitializationResultShape(scanner.Bytes()))
}

func TestCAS13PostInitializationNotificationClass(t *testing.T) {
	if os.Getenv("DOCKPIPE_CAS13_DIAGNOSTIC") != "run" {
		t.Skip("set DOCKPIPE_CAS13_DIAGNOSTIC=run for the reviewed shape diagnostic")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	child, err := cas13Launcher().Start(ctx)
	if err != nil {
		t.Fatal("CAS-13 direct child could not start")
	}
	defer func() {
		_ = child.Stdin().Close()
		_ = child.Kill()
		_ = child.Wait()
	}()
	encoder := json.NewEncoder(child.Stdin())
	if err := encoder.Encode(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": map[string]any{"clientInfo": map[string]any{"name": "dockpipe-cas13", "version": "0.1.0"}, "capabilities": map[string]any{}}}); err != nil {
		t.Fatal("CAS-13 initialization request could not be sent")
	}
	scanner := bufio.NewScanner(child.Stdout())
	scanner.Buffer(make([]byte, 4096), 1<<20)
	if !scanner.Scan() {
		t.Fatal("CAS-13 initialization produced no envelope")
	}
	if err := encoder.Encode(map[string]any{"jsonrpc": "2.0", "method": "initialized", "params": map[string]any{}}); err != nil {
		t.Fatal("CAS-13 initialized notification could not be sent")
	}
	next := make(chan []byte, 1)
	go func() {
		if scanner.Scan() {
			next <- append([]byte(nil), scanner.Bytes()...)
		}
	}()
	select {
	case frame := <-next:
		t.Logf("CAS-13 post-initialization notification class: %s", cas13NotificationClass(frame))
	case <-time.After(5 * time.Second):
		t.Log("CAS-13 post-initialization notification class: none")
	}
}

func TestCAS13ThreadStartedShape(t *testing.T) {
	if os.Getenv("DOCKPIPE_CAS13_DIAGNOSTIC") != "run" {
		t.Skip("set DOCKPIPE_CAS13_DIAGNOSTIC=run for the reviewed shape diagnostic")
	}
	workspace := os.Getenv("DOCKPIPE_CAS13_WORKSPACE")
	if !filepath.IsAbs(workspace) {
		t.Fatal("CAS-13 requires an absolute declared workspace")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	child, err := cas13Launcher().Start(ctx)
	if err != nil {
		t.Fatal("CAS-13 direct child could not start")
	}
	defer func() { _ = child.Stdin().Close(); _ = child.Kill(); _ = child.Wait() }()
	encoder := json.NewEncoder(child.Stdin())
	if err := encoder.Encode(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": map[string]any{"clientInfo": map[string]any{"name": "dockpipe-cas13", "version": "0.1.0"}, "capabilities": map[string]any{"optOutNotificationMethods": []string{remoteControlStatusNotification}}}}); err != nil {
		t.Fatal("CAS-13 initialization request could not be sent")
	}
	scanner := bufio.NewScanner(child.Stdout())
	scanner.Buffer(make([]byte, 4096), 1<<20)
	if !scanner.Scan() {
		t.Fatal("CAS-13 initialization produced no envelope")
	}
	if err := encoder.Encode(map[string]any{"jsonrpc": "2.0", "method": "initialized", "params": map[string]any{}}); err != nil {
		t.Fatal("CAS-13 initialized notification could not be sent")
	}
	params := LifecyclePolicy{Workspace: workspace, WritableRoots: []string{workspace}, Sandbox: "workspace-write", ApprovalPolicy: "untrusted", Reviewer: "user", Model: PinnedModel, ReasoningEffort: PinnedReasoningEffort, ModelProvider: "openai"}.params()
	if err := encoder.Encode(map[string]any{"jsonrpc": "2.0", "id": 2, "method": "thread/start", "params": params}); err != nil {
		t.Fatal("CAS-13 thread request could not be sent")
	}
	for deadline := time.NewTimer(10 * time.Second); ; {
		select {
		case <-deadline.C:
			t.Fatal("CAS-13 thread start notification was not observed")
		default:
		}
		if !scanner.Scan() {
			t.Fatal("CAS-13 thread start transport closed")
		}
		if shape, found := cas13ThreadStartedShape(scanner.Bytes()); found {
			t.Logf("CAS-13 thread started shape: %s", shape)
			return
		}
	}
}

// TestCAS13ThreadStatusShape retains no provider frame. It classifies only
// the field names and JSON value kinds of the first status notification that
// follows a constrained untrusted workspace-change request.
func TestCAS13ThreadStatusShape(t *testing.T) {
	if os.Getenv("DOCKPIPE_CAS13_DIAGNOSTIC") != "run" {
		t.Skip("set DOCKPIPE_CAS13_DIAGNOSTIC=run for the reviewed shape diagnostic")
	}
	workspace := os.Getenv("DOCKPIPE_CAS13_WORKSPACE")
	if !filepath.IsAbs(workspace) || filepath.Clean(workspace) != workspace {
		t.Fatal("CAS-13 requires a clean absolute declared workspace")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	child, err := cas13Launcher().Start(ctx)
	if err != nil {
		t.Fatal("CAS-13 direct child could not start")
	}
	defer func() { _ = child.Stdin().Close(); _ = child.Kill(); _ = child.Wait() }()
	encoder := json.NewEncoder(child.Stdin())
	scanner := bufio.NewScanner(child.Stdout())
	scanner.Buffer(make([]byte, 4096), 1<<20)
	if err := encoder.Encode(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": map[string]any{"clientInfo": map[string]any{"name": "dockpipe-cas13", "version": "0.1.0"}, "capabilities": map[string]any{"optOutNotificationMethods": []string{remoteControlStatusNotification}}}}); err != nil {
		t.Fatal("CAS-13 initialization request could not be sent")
	}
	if _, ok := cas13DiagnosticResponse(t, scanner, 1); !ok {
		t.Fatal("CAS-13 initialization response was not observed")
	}
	if err := encoder.Encode(map[string]any{"jsonrpc": "2.0", "method": "initialized", "params": map[string]any{}}); err != nil {
		t.Fatal("CAS-13 initialized notification could not be sent")
	}
	policy := LifecyclePolicy{Workspace: workspace, WritableRoots: []string{workspace}, Sandbox: "workspace-write", ApprovalPolicy: "untrusted", Reviewer: "user", Model: PinnedModel, ReasoningEffort: PinnedReasoningEffort, ModelProvider: "openai"}
	threadResult := cas13DiagnosticRequest(t, encoder, scanner, 2, "thread/start", policy.params())
	threadID, reason := projectThread(threadResult)
	if reason != "" {
		t.Fatal("CAS-13 diagnostic thread response was invalid")
	}
	turnParams := policy.params()
	turnParams["threadId"] = threadID
	turnParams["input"] = cas13Prompt(t)
	if err := encoder.Encode(map[string]any{"jsonrpc": "2.0", "id": 3, "method": "turn/start", "params": turnParams}); err != nil {
		t.Fatal("CAS-13 diagnostic turn request could not be sent")
	}
	statuses := 0
	for scanner.Scan() {
		var envelope struct {
			ID     json.RawMessage `json:"id"`
			Method string          `json:"method"`
			Params json.RawMessage `json:"params"`
		}
		if json.Unmarshal(scanner.Bytes(), &envelope) != nil {
			continue
		}
		if envelope.Method == "thread/status/changed" {
			statuses++
			t.Logf("CAS-13 thread status %d shape: %s", statuses, cas13ObjectShape(envelope.Params))
			if statuses > 8 {
				t.Fatal("CAS-13 thread status diagnostic exceeded bounded sequence")
			}
			continue
		}
		if len(envelope.ID) != 0 && envelope.Method != "" {
			t.Logf("CAS-13 approval request shape: id=%s,method=%s,params=%s", cas13ValueShape(envelope.ID), cas13RequestMethodClass(envelope.Method), cas13ObjectShape(envelope.Params))
			return
		}
	}
	t.Fatal("CAS-13 approval request was not observed")
}

func cas13DiagnosticRequest(t *testing.T, encoder *json.Encoder, scanner *bufio.Scanner, id uint64, method string, params map[string]any) json.RawMessage {
	t.Helper()
	if err := encoder.Encode(map[string]any{"jsonrpc": "2.0", "id": id, "method": method, "params": params}); err != nil {
		t.Fatalf("CAS-13 %s request could not be sent", method)
	}
	result, ok := cas13DiagnosticResponse(t, scanner, id)
	if !ok {
		t.Fatalf("CAS-13 %s response was not observed", method)
	}
	return result
}

func cas13DiagnosticResponse(t *testing.T, scanner *bufio.Scanner, id uint64) (json.RawMessage, bool) {
	t.Helper()
	for scanner.Scan() {
		var envelope struct {
			ID     json.RawMessage `json:"id"`
			Result json.RawMessage `json:"result"`
			Error  json.RawMessage `json:"error"`
		}
		if json.Unmarshal(scanner.Bytes(), &envelope) != nil || len(envelope.ID) == 0 {
			continue
		}
		var received uint64
		if json.Unmarshal(envelope.ID, &received) != nil || received != id || len(envelope.Error) != 0 || len(envelope.Result) == 0 {
			continue
		}
		return append(json.RawMessage(nil), envelope.Result...), true
	}
	return nil, false
}

func cas13ObjectShape(raw json.RawMessage) string {
	var fields map[string]json.RawMessage
	if json.Unmarshal(raw, &fields) != nil || fields == nil {
		return "not_object"
	}
	parts := make([]string, 0, len(fields))
	for name, value := range fields {
		parts = append(parts, name+":"+cas13ValueShape(value))
	}
	sort.Strings(parts)
	return strings.Join(parts, ",")
}

func cas13ValueShape(raw json.RawMessage) string {
	var stringValue string
	if json.Unmarshal(raw, &stringValue) == nil {
		return "string(len=" + strconv.Itoa(len(stringValue)) + ",id=" + strconv.FormatBool(validID(stringValue)) + ")"
	}
	var unsigned uint64
	if json.Unmarshal(raw, &unsigned) == nil {
		return "uint(" + strconv.FormatBool(unsigned > 0) + ")"
	}
	var signed int64
	if json.Unmarshal(raw, &signed) == nil {
		return "int"
	}
	var decimal float64
	if json.Unmarshal(raw, &decimal) == nil {
		return "decimal"
	}
	var object map[string]json.RawMessage
	if json.Unmarshal(raw, &object) == nil && object != nil {
		keys := make([]string, 0, len(object))
		for key := range object {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		return "object(" + strings.Join(keys, "+") + ")"
	}
	if string(raw) == "null" {
		return "null"
	}
	return "other"
}

func cas13RequestMethodClass(method string) string {
	switch method {
	case "item/commandExecution/requestApproval":
		return "command_approval"
	case "item/fileChange/requestApproval":
		return "file_approval"
	case "item/permissions/requestApproval":
		return "permission_approval"
	case "item/tool/requestUserInput":
		return "user_input"
	default:
		return "other"
	}
}

func cas13EnvelopeShape(frame []byte) string {
	var envelope map[string]json.RawMessage
	if json.Unmarshal(frame, &envelope) != nil || envelope == nil {
		return "not_json_object"
	}
	jsonRPC := "absent"
	if raw, found := envelope["jsonrpc"]; found {
		var version string
		if json.Unmarshal(raw, &version) == nil && version == "2.0" {
			jsonRPC = "2.0"
		} else {
			jsonRPC = "other"
		}
	}
	id := "absent"
	if raw, found := envelope["id"]; found {
		var number uint64
		var text string
		switch {
		case json.Unmarshal(raw, &number) == nil && number > 0:
			id = "number"
		case json.Unmarshal(raw, &text) == nil && text != "":
			id = "string"
		default:
			id = "other"
		}
	}
	method := "absent"
	if raw, found := envelope["method"]; found {
		var value string
		if json.Unmarshal(raw, &value) == nil && value != "" {
			method = "present"
		} else {
			method = "other"
		}
	}
	result, failure, params := "absent", "absent", "absent"
	if len(envelope["result"]) != 0 {
		result = "present"
	}
	if len(envelope["error"]) != 0 {
		failure = "present"
	}
	if len(envelope["params"]) != 0 {
		params = "present"
	}
	return "jsonrpc=" + jsonRPC + ",id=" + id + ",method=" + method + ",result=" + result + ",error=" + failure + ",params=" + params
}

func cas13NotificationClass(frame []byte) string {
	var envelope struct {
		Method string `json:"method"`
	}
	if json.Unmarshal(frame, &envelope) != nil || envelope.Method == "" {
		return "not_notification"
	}
	switch envelope.Method {
	case "account/updated":
		return "account_update"
	case "remoteControl/status/changed":
		return "remote_control_status"
	case "configWarning", "config/updated":
		return "configuration"
	case "thread/started", "thread/status/changed", "turn/started", "item/started":
		return "lifecycle"
	case "error", "warning":
		return "diagnostic"
	default:
		return "unrecognized"
	}
}

func cas13InitializationResultShape(frame []byte) string {
	var envelope struct {
		Result json.RawMessage `json:"result"`
	}
	if json.Unmarshal(frame, &envelope) != nil {
		return "unavailable"
	}
	var result map[string]json.RawMessage
	if json.Unmarshal(envelope.Result, &result) != nil || result == nil {
		return "not_object"
	}
	fields := []string{"protocolVersion", "serverInfo", "capabilities", "configWarnings", "userAgent", "codexHome", "platformFamily", "platformOs"}
	shape := ""
	for _, field := range fields {
		if len(result[field]) == 0 {
			shape += "0"
		} else {
			shape += "1"
		}
	}
	return shape
}

func cas13ThreadStartedShape(frame []byte) (string, bool) {
	var envelope struct {
		Method string          `json:"method"`
		Params json.RawMessage `json:"params"`
	}
	if json.Unmarshal(frame, &envelope) != nil || envelope.Method != "thread/started" {
		return "", false
	}
	var params struct {
		Thread map[string]json.RawMessage `json:"thread"`
	}
	if json.Unmarshal(envelope.Params, &params) != nil || params.Thread == nil {
		return "not_object", true
	}
	known := []string{"id", "name", "preview", "createdAt", "updatedAt", "status", "cwd", "model", "modelProvider", "sandbox", "approvalPolicy"}
	shape, extras := "", 0
	for _, field := range known {
		if len(params.Thread[field]) == 0 {
			shape += "0"
		} else {
			shape += "1"
		}
	}
	for field := range params.Thread {
		found := false
		for _, knownField := range known {
			if field == knownField {
				found = true
				break
			}
		}
		if !found {
			extras++
		}
	}
	return shape + ":extra=" + string(rune('0'+extras)), true
}

func verifyCAS13Catalog(ctx context.Context, s *Supervisor) error {
	s.mu.RLock()
	client := s.client
	s.mu.RUnlock()
	if client == nil {
		return errors.New("CAS-13 catalog route is unavailable")
	}
	result, err := s.lifecycleRequest(ctx, client, "model/list", map[string]any{})
	if err != nil || containsModelReroute(result) {
		return errors.New("CAS-13 pinned model policy could not be verified")
	}
	var catalog struct {
		Data []struct {
			ID                        string `json:"id"`
			SupportedReasoningEfforts []struct {
				ReasoningEffort string `json:"reasoningEffort"`
			} `json:"supportedReasoningEfforts"`
		} `json:"data"`
	}
	if json.Unmarshal(result, &catalog) != nil {
		return errors.New("CAS-13 model catalog is malformed")
	}
	for _, model := range catalog.Data {
		if model.ID != PinnedModel {
			continue
		}
		for _, effort := range model.SupportedReasoningEfforts {
			if effort.ReasoningEffort == PinnedReasoningEffort {
				return nil
			}
		}
	}
	return errors.New("CAS-13 pinned model and reasoning effort are unavailable")
}
