package appserversupervisor

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"dorkpipe.orchestrator/providersession"
)

func TestCAS11RejectsUnsafeConstructorAndLauncherConfiguration(t *testing.T) {
	for name, launcher := range map[string]HostLauncher{
		"shell":        {Executable: "pwsh.exe", Args: []string{"app-server", "--stdio"}},
		"batch":        {Executable: "codex.cmd", Args: []string{"app-server", "--stdio"}},
		"extra_arg":    {Executable: "codex", Args: []string{"app-server", "--stdio", "--unsafe"}},
		"wrong_binary": {Executable: "other", Args: []string{"app-server", "--stdio"}},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := New(providersession.SessionRef{Provider: "test", SessionID: "session"}, launcher, testDeadlines(), testInitialization()); err == nil {
				t.Fatal("unsafe direct-child launcher was accepted")
			}
		})
	}
	child := newFakeChild()
	for name, mutate := range map[string]func(*Deadlines, *InitializationConfig, *providersession.SessionRef){
		"unbounded_deadline": func(d *Deadlines, _ *InitializationConfig, _ *providersession.SessionRef) {
			d.Request = maxSupervisorDeadline + time.Second
		},
		"unsafe_client": func(_ *Deadlines, c *InitializationConfig, _ *providersession.SessionRef) {
			c.ClientName = "client name"
		},
		"oversized_session": func(_ *Deadlines, _ *InitializationConfig, s *providersession.SessionRef) {
			s.SessionID = strings.Repeat("x", 129)
		},
	} {
		t.Run(name, func(t *testing.T) {
			deadlines, initialization := testDeadlines(), testInitialization()
			session := providersession.SessionRef{Provider: "test", SessionID: "session"}
			mutate(&deadlines, &initialization, &session)
			if _, err := New(session, fakeLauncher{start: func(context.Context) (Child, error) { return child, nil }}, deadlines, initialization); err == nil {
				t.Fatal("unsafe bounded configuration was accepted")
			}
		})
	}
}

func TestCAS11RejectsExtendedProtocolShapesWithoutPrivateRetention(t *testing.T) {
	t.Run("initialization_extension", func(t *testing.T) {
		child := newFakeChild()
		s := newTestSupervisor(t, fakeLauncher{start: func(context.Context) (Child, error) { return child, nil }}, testDeadlines())
		done, id := beginInitialize(t, s, child)
		_, _ = child.stdoutW.Write([]byte(response(id, `{"protocolVersion":"v2","serverInfo":{"name":"codex","version":"0.144.1"},"capabilities":{"stableV2":true},"unexpected":"private"}`)))
		if err := <-done; err == nil {
			t.Fatal("extended initialization result was accepted")
		}
		event := nextEvent(t, s)
		if event.State != providersession.StateDisconnected || event.Summary != string(DisconnectUnsupportedSchema) || strings.Contains(event.Summary, "private") {
			t.Fatalf("initialization rejection = %+v", event)
		}
	})
	t.Run("event_extension", func(t *testing.T) {
		s, child, _, _, _ := startEventTurn(t)
		sendNotification(t, child, "warning", `{"threadId":"thread-1","turnId":"turn-1","warning":"config_deprecated","extended":"private"}`)
		event := nextEvent(t, s)
		if event.State != providersession.StateDisconnected || event.Summary != string(DisconnectUnsupportedEvent) || strings.Contains(event.Summary, "private") {
			t.Fatalf("event rejection = %+v", event)
		}
	})
	t.Run("duplicate_event_field", func(t *testing.T) {
		s, child, _, _, _ := startEventTurn(t)
		sendNotification(t, child, "warning", `{"threadId":"thread-1","threadId":"thread-1","turnId":"turn-1","warning":"config_deprecated"}`)
		event := nextEvent(t, s)
		if event.State != providersession.StateDisconnected || event.Summary != string(DisconnectUnsupportedEvent) {
			t.Fatalf("duplicate event rejection = %+v", event)
		}
	})
	t.Run("request_extension", func(t *testing.T) {
		s, child, _, _, _ := startApprovalTurn(t)
		sendServerRequest(t, child, 71, "item/commandExecution/requestApproval", `{"threadId":"thread-1","turnId":"turn-1","itemId":"item-1","command":"private","extended":"private"}`)
		event := nextEvent(t, s)
		if event.State != providersession.StateDisconnected || event.Summary != string(DisconnectUnsupportedEvent) || strings.Contains(event.Summary, "private") {
			t.Fatalf("request rejection = %+v", event)
		}
	})
}

func TestCAS11DisconnectClearsPrivatePendingStateAndPreventsReuse(t *testing.T) {
	s, child, _, _, _ := startApprovalTurn(t)
	event := approvalRequest(t, s, child, 72, "item/commandExecution/requestApproval", `{"threadId":"thread-1","turnId":"turn-1","itemId":"item-1"}`)
	s.disconnect(DisconnectTransportOwnership)
	disconnected := nextEvent(t, s)
	if disconnected.State != providersession.StateDisconnected || disconnected.Summary != string(DisconnectTransportOwnership) {
		t.Fatalf("disconnect = %+v", disconnected)
	}
	s.mu.RLock()
	pending, stdin, stdout := s.lifecycle.pending, s.stdin, s.stdout
	s.mu.RUnlock()
	if pending != nil || stdin != nil || stdout != nil {
		t.Fatal("disconnect retained reusable private transport or approval state")
	}
	if err := s.Decide(context.Background(), providersession.ApprovalDecision{Correlation: event.Approval.Correlation, Decision: providersession.DecisionDeny}); !errors.Is(err, ErrDecisionRejected) {
		t.Fatalf("stale decision error = %v", err)
	}
}
