package appserversupervisor

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"dorkpipe.orchestrator/providersession"
)

func startApprovalTurn(t *testing.T) (*Supervisor, *fakeChild, *bufio.Scanner, LifecyclePolicy, LifecycleReference) {
	t.Helper()
	s, child, scanner, policy, turn := startEventTurn(t)
	sendNotification(t, child, "item/started", `{"threadId":"thread-1","turnId":"turn-1","item":{"id":"item-1","type":"commandExecution","status":"inProgress"}}`)
	if event := nextEvent(t, s); event.Summary != "item_started" {
		t.Fatalf("item event = %+v", event)
	}
	return s, child, scanner, policy, turn
}

func sendServerRequest(t *testing.T, child *fakeChild, id uint64, method, params string) {
	t.Helper()
	// App Server request identifiers are numeric; avoid retaining a provider
	// identifier in any projected test value.
	frame := `{"jsonrpc":"2.0","id":` + jsonNumber(id) + `,"method":` + quoteJSON(method) + `,"params":` + params + "}\n"
	if _, err := child.stdoutW.Write([]byte(frame)); err != nil {
		t.Fatal(err)
	}
}

func jsonNumber(value uint64) string {
	data, _ := json.Marshal(value)
	return string(data)
}

func approvalRequest(t *testing.T, s *Supervisor, child *fakeChild, id uint64, method, params string) providersession.Event {
	t.Helper()
	sendServerRequest(t, child, id, method, params)
	event := nextEvent(t, s)
	if event.Kind != providersession.EventApprovalRequested || event.State != providersession.StateWaitingForApproval || event.Approval == nil {
		t.Fatalf("approval event = %+v", event)
	}
	return event
}

func TestApprovalRelayProjectsSafeCorrelatedRequestsAndOneTimeDecision(t *testing.T) {
	for _, fixture := range []struct {
		name        string
		method      string
		params      string
		actionClass string
		decision    string
		wantResult  map[string]any
	}{
		{"command", "item/commandExecution/requestApproval", `{"threadId":"thread-1","turnId":"turn-1","itemId":"item-1","command":"private command","cwd":"private path","reason":"private reason"}`, "command_execution", providersession.DecisionApprove, map[string]any{"decision": "accept"}},
		{"file", "item/fileChange/requestApproval", `{"threadId":"thread-1","turnId":"turn-1","itemId":"item-1","patch":"private patch","reason":"private reason"}`, "workspace_change", providersession.DecisionDeny, map[string]any{"decision": "decline"}},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			s, child, scanner, _, _ := startApprovalTurn(t)
			event := approvalRequest(t, s, child, 41, fixture.method, fixture.params)
			if event.Approval.ActionClass != fixture.actionClass || len(event.Approval.Scope) != 1 || event.Approval.Scope[0] != "turn" || event.Approval.Correlation.ProcessIncarnationID == "" || event.Approval.Correlation.ConnectionID == "" || event.Approval.Correlation.SessionID != "thread-1" || event.Approval.Correlation.InteractionID != "turn-1" || event.Approval.Correlation.ActivityID != "item-1" || event.Approval.Correlation.RequestID == "" || event.Approval.Correlation.DecisionID == "" {
				t.Fatalf("unsafe approval projection = %+v", event)
			}
			data, _ := json.Marshal(event)
			if strings.Contains(string(data), "private") || strings.Contains(string(data), "command") && fixture.name != "command" {
				t.Fatalf("raw approval content leaked: %s", data)
			}
			done := make(chan error, 1)
			go func() {
				done <- s.Decide(context.Background(), providersession.ApprovalDecision{Correlation: event.Approval.Correlation, Decision: fixture.decision})
			}()
			if !scanner.Scan() {
				t.Fatal("expected private decision response")
			}
			var response struct {
				ID     uint64         `json:"id"`
				Result map[string]any `json:"result"`
			}
			if err := json.Unmarshal(scanner.Bytes(), &response); err != nil || response.ID != 41 || !sameResult(response.Result, fixture.wantResult) {
				t.Fatalf("decision response = %s, err=%v", scanner.Text(), err)
			}
			if err := <-done; err != nil {
				t.Fatal(err)
			}
			sendNotification(t, child, "serverRequest/resolved", `{"threadId":"thread-1","requestId":41}`)
			if resolved := nextEvent(t, s); resolved.State != providersession.StateRunning || resolved.Summary != "approval_resolved" || resolved.Correlation != event.Approval.Correlation {
				t.Fatalf("resolution event = %+v", resolved)
			}
		})
	}
}

func TestApprovalRelayPermissionIsDeclaredAndDenyOnly(t *testing.T) {
	s, child, scanner, policy, _ := startApprovalTurn(t)
	params := `{"threadId":"thread-1","turnId":"turn-1","itemId":"item-1","permissions":{"fileSystem":{"write":[` + quoteJSON(policy.Workspace) + `]}}}`
	event := approvalRequest(t, s, child, 42, "item/permissions/requestApproval", params)
	if event.Approval.ActionClass != "declared_permission" || len(event.Approval.Scope) != 1 || event.Approval.Scope[0] != "declared_writable_roots" {
		t.Fatalf("permission projection = %+v", event)
	}
	approve := providersession.ApprovalDecision{Correlation: event.Approval.Correlation, Decision: providersession.DecisionApprove}
	if err := s.Decide(context.Background(), approve); err == nil {
		t.Fatal("permission approval without a neutral granted subset must fail")
	}
	if disconnected := nextEvent(t, s); disconnected.State != providersession.StateDisconnected || disconnected.Summary != string(DisconnectDecisionRejected) {
		t.Fatalf("permission decision event = %+v", disconnected)
	}
	_ = scanner
}

func TestUserInputRelayIsOpaqueAndHasNoAnswerOperation(t *testing.T) {
	s, child, _, _, _ := startApprovalTurn(t)
	sendServerRequest(t, child, 43, "item/tool/requestUserInput", `{"threadId":"thread-1","turnId":"turn-1","itemId":"item-1","questions":[{"id":"question-1","header":"private header","question":"private prompt","options":["private option"]}]}`)
	event := nextEvent(t, s)
	if event.Kind != providersession.EventUserInputRequested || event.State != providersession.StateWaitingForUserInput || event.UserInput == nil || event.UserInput.PromptRef == "" || event.UserInput.Correlation.ActivityID != "item-1" {
		t.Fatalf("user input event = %+v", event)
	}
	data, _ := json.Marshal(event)
	if strings.Contains(string(data), "private") || strings.Contains(string(data), "question-1") {
		t.Fatalf("user-input content leaked: %s", data)
	}
	if err := s.Decide(context.Background(), providersession.ApprovalDecision{Correlation: event.UserInput.Correlation, Decision: providersession.DecisionDeny}); err == nil {
		t.Fatal("user-input answer must not be encoded as an approval decision")
	}
	if disconnected := nextEvent(t, s); disconnected.State != providersession.StateDisconnected || disconnected.Summary != string(DisconnectDecisionRejected) {
		t.Fatalf("unsupported input decision = %+v", disconnected)
	}
}

func TestApprovalRelayRejectsDuplicateStaleReorderedAndCrossCorrelatedMessages(t *testing.T) {
	tests := []struct {
		name   string
		apply  func(*testing.T, *Supervisor, *fakeChild, *bufio.Scanner, providersession.Event)
		reason DisconnectReason
	}{
		{"duplicate", func(t *testing.T, _ *Supervisor, child *fakeChild, _ *bufio.Scanner, _ providersession.Event) {
			sendServerRequest(t, child, 44, "item/commandExecution/requestApproval", `{"threadId":"thread-1","turnId":"turn-1","itemId":"item-1"}`)
		}, DisconnectEventOrdering},
		{"malformed", func(t *testing.T, _ *Supervisor, child *fakeChild, _ *bufio.Scanner, _ providersession.Event) {
			sendServerRequest(t, child, 44, "item/commandExecution/requestApproval", `[]`)
		}, DisconnectMalformedEnvelope},
		{"uncorrelated", func(t *testing.T, _ *Supervisor, child *fakeChild, _ *bufio.Scanner, _ providersession.Event) {
			sendServerRequest(t, child, 44, "item/commandExecution/requestApproval", `{"threadId":"thread-1","turnId":"turn-1"}`)
		}, DisconnectMalformedEnvelope},
		{"cross_thread", func(t *testing.T, _ *Supervisor, child *fakeChild, _ *bufio.Scanner, _ providersession.Event) {
			sendServerRequest(t, child, 44, "item/commandExecution/requestApproval", `{"threadId":"thread-2","turnId":"turn-1","itemId":"item-1"}`)
		}, DisconnectCorrelationMismatch},
		{"cross_turn", func(t *testing.T, _ *Supervisor, child *fakeChild, _ *bufio.Scanner, _ providersession.Event) {
			sendServerRequest(t, child, 44, "item/commandExecution/requestApproval", `{"threadId":"thread-1","turnId":"turn-2","itemId":"item-1"}`)
		}, DisconnectCorrelationMismatch},
		{"cross_item", func(t *testing.T, _ *Supervisor, child *fakeChild, _ *bufio.Scanner, _ providersession.Event) {
			sendServerRequest(t, child, 44, "item/commandExecution/requestApproval", `{"threadId":"thread-1","turnId":"turn-1","itemId":"item-2"}`)
		}, DisconnectCorrelationMismatch},
		{"reordered_resolution", func(t *testing.T, _ *Supervisor, child *fakeChild, _ *bufio.Scanner, _ providersession.Event) {
			sendNotification(t, child, "serverRequest/resolved", `{"threadId":"thread-1","requestId":44}`)
		}, DisconnectEventOrdering},
		{"unsupported_network", func(t *testing.T, _ *Supervisor, child *fakeChild, _ *bufio.Scanner, _ providersession.Event) {
			sendServerRequest(t, child, 44, "item/commandExecution/requestApproval", `{"threadId":"thread-1","turnId":"turn-1","itemId":"item-1","networkApprovalContext":{}}`)
		}, DisconnectUnsupportedEvent},
		{"stale_decision", func(t *testing.T, s *Supervisor, _ *fakeChild, _ *bufio.Scanner, event providersession.Event) {
			decision := providersession.ApprovalDecision{Correlation: event.Approval.Correlation, Decision: providersession.DecisionDeny}
			decision.Correlation.DecisionID = "stale"
			_ = s.Decide(context.Background(), decision)
		}, DisconnectCorrelationMismatch},
	}
	for _, fixture := range tests {
		t.Run(fixture.name, func(t *testing.T) {
			s, child, scanner, _, _ := startApprovalTurn(t)
			event := approvalRequest(t, s, child, 44, "item/commandExecution/requestApproval", `{"threadId":"thread-1","turnId":"turn-1","itemId":"item-1"}`)
			fixture.apply(t, s, child, scanner, event)
			if disconnected := nextEvent(t, s); disconnected.State != providersession.StateDisconnected || disconnected.Summary != string(fixture.reason) {
				t.Fatalf("disconnect event = %+v", disconnected)
			}
		})
	}
}

func TestApprovalRelayRejectsDuplicateDecisionIdentity(t *testing.T) {
	s, child, scanner, _, _ := startApprovalTurn(t)
	event := approvalRequest(t, s, child, 47, "item/commandExecution/requestApproval", `{"threadId":"thread-1","turnId":"turn-1","itemId":"item-1"}`)
	decision := providersession.ApprovalDecision{Correlation: event.Approval.Correlation, Decision: providersession.DecisionDeny}
	done := make(chan error, 1)
	go func() { done <- s.Decide(context.Background(), decision) }()
	if !scanner.Scan() {
		t.Fatal("expected first decision response")
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if err := s.Decide(context.Background(), decision); err == nil {
		t.Fatal("duplicate decision identity must be rejected")
	}
	if disconnected := nextEvent(t, s); disconnected.State != providersession.StateDisconnected || disconnected.Summary != string(DisconnectDecisionRejected) {
		t.Fatalf("duplicate decision event = %+v", disconnected)
	}
}

func TestApprovalRelayFailsClosedOnExpiryTransportChildExitProviderErrorAndReroute(t *testing.T) {
	t.Run("expiry", func(t *testing.T) {
		s, child, scanner, _, _ := startApprovalTurn(t)
		s.deadlines.Request = 20 * time.Millisecond
		_ = approvalRequest(t, s, child, 45, "item/commandExecution/requestApproval", `{"threadId":"thread-1","turnId":"turn-1","itemId":"item-1"}`)
		if !scanner.Scan() {
			t.Fatal("expiry must send a private default-deny response")
		}
		var response struct {
			Result map[string]any `json:"result"`
		}
		if err := json.Unmarshal(scanner.Bytes(), &response); err != nil || !sameResult(response.Result, map[string]any{"decision": "decline"}) {
			t.Fatalf("expiry response = %s, err=%v", scanner.Text(), err)
		}
		if disconnected := nextEvent(t, s); disconnected.State != providersession.StateDisconnected || disconnected.Summary != string(DisconnectRequestDeadline) {
			t.Fatalf("expiry event = %+v", disconnected)
		}
	})
	for _, fixture := range []struct {
		name   string
		apply  func(*testing.T, *fakeChild)
		reason DisconnectReason
	}{
		{"transport", func(_ *testing.T, child *fakeChild) { _ = child.stdoutW.Close() }, DisconnectTransportClosed},
		{"child_exit", func(_ *testing.T, child *fakeChild) { child.exit(errors.New("died")) }, DisconnectChildExit},
		{"provider_error", func(t *testing.T, child *fakeChild) { sendServerRequestError(t, child, 46) }, DisconnectProviderError},
		{"reroute", func(t *testing.T, child *fakeChild) {
			sendNotification(t, child, "model/rerouted", `{"threadId":"thread-1","turnId":"turn-1","itemId":"item-1"}`)
		}, DisconnectModelRerouted},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			s, child, _, _, _ := startApprovalTurn(t)
			_ = approvalRequest(t, s, child, 46, "item/commandExecution/requestApproval", `{"threadId":"thread-1","turnId":"turn-1","itemId":"item-1"}`)
			fixture.apply(t, child)
			if disconnected := nextEvent(t, s); disconnected.State != providersession.StateDisconnected || disconnected.Summary != string(fixture.reason) {
				t.Fatalf("disconnect event = %+v", disconnected)
			}
		})
	}
}

func sendServerRequestError(t *testing.T, child *fakeChild, id uint64) {
	t.Helper()
	if _, err := child.stdoutW.Write([]byte(`{"jsonrpc":"2.0","id":` + jsonNumber(id) + `,"error":{"message":"private provider error"}}` + "\n")); err != nil {
		t.Fatal(err)
	}
}

func sameResult(got, want map[string]any) bool {
	gotJSON, _ := json.Marshal(got)
	wantJSON, _ := json.Marshal(want)
	return string(gotJSON) == string(wantJSON)
}
