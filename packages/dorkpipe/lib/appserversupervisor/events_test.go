package appserversupervisor

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"dorkpipe.orchestrator/providersession"
)

func sendNotification(t *testing.T, child *fakeChild, method, params string) {
	t.Helper()
	frame := `{"jsonrpc":"2.0","method":` + quoteJSON(method) + `,"params":` + params + "}\n"
	if _, err := child.stdoutW.Write([]byte(frame)); err != nil {
		t.Fatal(err)
	}
}

func quoteJSON(value string) string {
	data, _ := json.Marshal(value)
	return string(data)
}

func startEventTurn(t *testing.T) (*Supervisor, *fakeChild, *bufio.Scanner, LifecyclePolicy, LifecycleReference) {
	t.Helper()
	s, child, scanner, policy := initializedLifecycle(t)
	thread := startThreadForTest(t, s, child, scanner, policy)
	if event := nextEvent(t, s); event.State != providersession.StateReady {
		t.Fatalf("ready event = %+v", event)
	}
	sendNotification(t, child, "thread/started", `{"thread":{"id":"thread-1"}}`)
	if event := nextEvent(t, s); event.Summary != "thread_started" {
		t.Fatalf("thread event = %+v", event)
	}
	sendNotification(t, child, "thread/status/changed", `{"threadId":"thread-1","status":"active"}`)
	if event := nextEvent(t, s); event.Summary != "thread_active" {
		t.Fatalf("thread status event = %+v", event)
	}
	turnDone := make(chan struct {
		ref LifecycleReference
		err error
	}, 1)
	go func() {
		ref, err := s.StartTurn(context.Background(), thread, policy, "turn-input-1")
		turnDone <- struct {
			ref LifecycleReference
			err error
		}{ref, err}
	}()
	_ = lifecycleRequest(t, scanner, "turn/start", 3)
	_, _ = child.stdoutW.Write([]byte(response(3, `{"thread":{"id":"thread-1"},"turn":{"id":"turn-1","status":"inProgress"}}`)))
	turn := <-turnDone
	if turn.err != nil {
		t.Fatal(turn.err)
	}
	if event := nextEvent(t, s); event.State != providersession.StateRunning || event.Summary != "turn_started" {
		t.Fatalf("running event = %+v", event)
	}
	sendNotification(t, child, "turn/started", `{"threadId":"thread-1","turn":{"id":"turn-1","status":"inProgress"}}`)
	if event := nextEvent(t, s); event.Summary != "turn_started" || event.Kind != providersession.EventProgress {
		t.Fatalf("turn notification event = %+v", event)
	}
	return s, child, scanner, policy, turn.ref
}

func TestEventNormalizationProjectsCorrelatedLifecycleProgressAndTerminal(t *testing.T) {
	s, child, scanner, policy, threadTurn := startEventTurn(t)
	fixtures := []struct {
		method  string
		params  string
		summary string
		item    bool
	}{
		{"item/started", `{"threadId":"thread-1","turnId":"turn-1","item":{"id":"item-1","type":"agentMessage","status":"inProgress"}}`, "item_started", true},
		{"item/agentMessage/delta", `{"threadId":"thread-1","turnId":"turn-1","itemId":"item-1","delta":"do not expose this token text"}`, "item_progress", true},
		{"thread/tokenUsage/updated", `{"threadId":"thread-1","turnId":"turn-1","tokenUsage":{"totalTokens":12,"text":"private"}}`, "token_usage_total_12", false},
		{"warning", `{"threadId":"thread-1","turnId":"turn-1","warning":"config_deprecated","message":"private warning"}`, "warning_config_deprecated", false},
		{"item/completed", `{"threadId":"thread-1","turnId":"turn-1","item":{"id":"item-1","type":"agentMessage","status":"completed","text":"private"}}`, "item_completed", true},
		{"turn/completed", `{"threadId":"thread-1","turn":{"id":"turn-1","status":"completed"}}`, "turn_completed", false},
	}
	previous := uint64(5)
	for _, fixture := range fixtures {
		sendNotification(t, child, fixture.method, fixture.params)
		event := nextEvent(t, s)
		if event.Kind != providersession.EventProgress || event.Summary != fixture.summary || event.Sequence != previous+1 {
			t.Fatalf("%s event = %+v", fixture.method, event)
		}
		if event.Correlation.ProcessIncarnationID == "" || event.Correlation.ConnectionID == "" || event.Correlation.SessionID != "thread-1" || event.Correlation.InteractionID != "turn-1" {
			t.Fatalf("unsafe or incomplete correlation for %s: %+v", fixture.method, event.Correlation)
		}
		if fixture.item && event.Correlation.ActivityID != "item-1" {
			t.Fatalf("item correlation = %+v", event.Correlation)
		}
		if strings.Contains(event.Summary, "private") || strings.Contains(event.Summary, "token text") {
			t.Fatalf("raw content leaked in %s: %+v", fixture.method, event)
		}
		previous = event.Sequence
	}
	if s.State() != providersession.StateRunning {
		t.Fatalf("terminal turn changed session state: %s", s.State())
	}

	// A validated terminal notification releases only the private active-turn
	// invariant; it does not implement cancellation, recovery, or replay.
	done := make(chan error, 1)
	go func() {
		_, err := s.StartTurn(context.Background(), LifecycleReference{Session: threadTurn.Session, Correlation: providersession.Correlation{ProcessIncarnationID: threadTurn.Correlation.ProcessIncarnationID, ConnectionID: threadTurn.Correlation.ConnectionID, SessionID: threadTurn.Session.SessionID}}, policy, "turn-input-2")
		done <- err
	}()
	_ = lifecycleRequest(t, scanner, "turn/start", 4)
	_, _ = child.stdoutW.Write([]byte(response(4, `{"thread":{"id":"thread-1"},"turn":{"id":"turn-2","status":"inProgress"}}`)))
	if err := <-done; err != nil {
		t.Fatalf("subsequent turn was rejected: %v", err)
	}
	if event := nextEvent(t, s); event.Kind != providersession.EventProgress || event.Summary != "turn_started" || event.Sequence != previous+1 {
		t.Fatalf("subsequent turn event = %+v", event)
	}
}

func TestEventNormalizationRejectsCorrelationAndOrderingViolations(t *testing.T) {
	tests := []struct {
		name   string
		setup  func(*testing.T, *Supervisor, *fakeChild)
		method string
		params string
		reason DisconnectReason
	}{
		{"duplicate_turn", nil, "turn/started", `{"threadId":"thread-1","turn":{"id":"turn-1","status":"inProgress"}}`, DisconnectEventOrdering},
		{"cross_thread", nil, "warning", `{"threadId":"thread-2","turnId":"turn-1","warning":"config_deprecated"}`, DisconnectCorrelationMismatch},
		{"cross_turn", nil, "warning", `{"threadId":"thread-1","turnId":"turn-2","warning":"config_deprecated"}`, DisconnectCorrelationMismatch},
		{"uncorrelated", nil, "warning", `{"threadId":"thread-1","warning":"config_deprecated"}`, DisconnectCorrelationMismatch},
		{"reordered_item", nil, "item/completed", `{"threadId":"thread-1","turnId":"turn-1","item":{"id":"item-1","type":"agentMessage","status":"completed"}}`, DisconnectCorrelationMismatch},
		{"cross_item", func(t *testing.T, s *Supervisor, child *fakeChild) {
			sendNotification(t, child, "item/started", `{"threadId":"thread-1","turnId":"turn-1","item":{"id":"item-1","type":"agentMessage","status":"inProgress"}}`)
			_ = nextEvent(t, s)
		}, "item/updated", `{"threadId":"thread-1","turnId":"turn-1","item":{"id":"item-2","type":"agentMessage","status":"inProgress"}}`, DisconnectCorrelationMismatch},
		{"stale_token", func(t *testing.T, s *Supervisor, child *fakeChild) {
			sendNotification(t, child, "thread/tokenUsage/updated", `{"threadId":"thread-1","turnId":"turn-1","tokenUsage":{"totalTokens":12}}`)
			_ = nextEvent(t, s)
		}, "thread/tokenUsage/updated", `{"threadId":"thread-1","turnId":"turn-1","tokenUsage":{"totalTokens":11}}`, DisconnectEventOrdering},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			s, child, _, _, _ := startEventTurn(t)
			if test.setup != nil {
				test.setup(t, s, child)
			}
			sendNotification(t, child, test.method, test.params)
			event := nextEvent(t, s)
			if event.State != providersession.StateDisconnected || event.Summary != string(test.reason) || s.State() != providersession.StateDisconnected {
				t.Fatalf("event = %+v, state=%s", event, s.State())
			}
		})
	}
}

func TestEventNormalizationRedactsFailuresAndRejectsUnsafeFrames(t *testing.T) {
	s, child, _, _, _ := startEventTurn(t)
	sendNotification(t, child, "warning", `{"threadId":"thread-1","turnId":"turn-1","warning":"config_ignored","message":"private warning body"}`)
	if event := nextEvent(t, s); event.Summary != "warning_config_ignored" || strings.Contains(event.Summary, "private") {
		t.Fatalf("warning projection = %+v", event)
	}
	sendNotification(t, child, "error", `{"threadId":"thread-1","turnId":"turn-1","error":{"codexErrorInfo":"other","message":"private error body","stack":"private"}}`)
	if event := nextEvent(t, s); event.Summary != "error_other" || strings.Contains(event.Summary, "private") {
		t.Fatalf("error projection = %+v", event)
	}
	sendNotification(t, child, "turn/completed", `{"threadId":"thread-1","turn":{"id":"turn-1","status":"failed","error":{"codexErrorInfo":"other","message":"private terminal body"}}}`)
	if event := nextEvent(t, s); event.Summary != "turn_failed" || strings.Contains(event.Summary, "private") {
		t.Fatalf("terminal projection = %+v", event)
	}

	for name, frame := range map[string]string{
		"malformed_params":  `{"jsonrpc":"2.0","method":"turn/started"}` + "\n",
		"unsupported_event": `{"jsonrpc":"2.0","method":"turn/unknown","params":{}}` + "\n",
		"reroute":           `{"jsonrpc":"2.0","method":"model/rerouted","params":{"threadId":"thread-1","turnId":"turn-1"}}` + "\n",
	} {
		t.Run(name, func(t *testing.T) {
			s, child, _, _, _ := startEventTurn(t)
			if _, err := child.stdoutW.Write([]byte(frame)); err != nil {
				t.Fatal(err)
			}
			event := nextEvent(t, s)
			if event.State != providersession.StateDisconnected {
				t.Fatalf("event = %+v", event)
			}
		})
	}
}

func TestEventNormalizerAddsNoDeferredLifecycleOperations(t *testing.T) {
	contents, err := os.ReadFile("events.go")
	if err != nil {
		t.Fatal(err)
	}
	lower := strings.ToLower(string(contents))
	for _, forbidden := range []string{"retry", "replay", "reconnect", "fallback", "persist", "approval", "cancel"} {
		if strings.Contains(lower, forbidden) {
			t.Fatalf("deferred operation %q found in event normalizer", forbidden)
		}
	}
}
