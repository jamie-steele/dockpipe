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

func cancellationIntent(turn LifecycleReference) providersession.CancellationIntent {
	return providersession.CancellationIntent{Session: turn.Session, Correlation: turn.Correlation, Reason: providersession.CancellationReasonUserRequested}
}

func beginCancellation(t *testing.T, s *Supervisor, scanner *bufio.Scanner, intent providersession.CancellationIntent) <-chan error {
	t.Helper()
	done := make(chan error, 1)
	go func() { done <- s.Cancel(context.Background(), intent) }()
	if event := nextEvent(t, s); event.Kind != providersession.EventCancellationRequested || event.Cancellation == nil || event.Cancellation.Session != intent.Session || event.Cancellation.Correlation != intent.Correlation || event.Cancellation.Reason != intent.Reason || event.State != "" || event.Summary != "cancellation_requested" {
		t.Fatalf("cancellation projection = %+v", event)
	}
	request := lifecycleRequest(t, scanner, "turn/interrupt", 4)
	params := requestParams(t, request)
	if len(params) != 2 || params["threadId"] != intent.Session.SessionID || params["turnId"] != intent.Correlation.InteractionID {
		t.Fatalf("interrupt request must contain only the exact private turn: %#v", request)
	}
	return done
}

func acknowledgeInterrupt(t *testing.T, child *fakeChild, id uint64, result string) {
	t.Helper()
	if _, err := child.stdoutW.Write([]byte(response(id, result))); err != nil {
		t.Fatal(err)
	}
}

func TestCancellationProjectsIntentDeliversExactInterruptAndWaitsForTerminal(t *testing.T) {
	s, child, scanner, _, turn := startEventTurn(t)
	intent := cancellationIntent(turn)
	done := beginCancellation(t, s, scanner, intent)
	acknowledgeInterrupt(t, child, 4, `{"thread":{"id":"thread-1"},"turn":{"id":"turn-1","status":"inProgress"}}`)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if s.State() != providersession.StateRunning {
		t.Fatalf("interrupt acknowledgement claimed terminal state: %s", s.State())
	}
	sendNotification(t, child, "turn/completed", `{"threadId":"thread-1","turn":{"id":"turn-1","status":"interrupted"}}`)
	event := nextEvent(t, s)
	if event.Kind != providersession.EventStateChanged || event.State != providersession.StateCancelled || event.Summary != "cancelled" || event.Correlation != intent.Correlation {
		t.Fatalf("cancelled terminal projection = %+v", event)
	}
}

func TestCancellationAcceptsCurrentEmptyInterruptAcknowledgement(t *testing.T) {
	s, child, scanner, _, turn := startEventTurn(t)
	intent := cancellationIntent(turn)
	done := beginCancellation(t, s, scanner, intent)
	acknowledgeInterrupt(t, child, 4, `{}`)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if s.State() != providersession.StateRunning {
		t.Fatalf("empty acknowledgement claimed terminal state: %s", s.State())
	}
	sendNotification(t, child, "turn/completed", `{"threadId":"thread-1","turn":{"id":"turn-1","status":"interrupted"}}`)
	if event := nextEvent(t, s); event.State != providersession.StateCancelled || event.Summary != "cancelled" {
		t.Fatalf("empty acknowledgement cancellation terminal = %+v", event)
	}
}

func TestCancellationAcceptsInterruptedTerminalWithActiveItem(t *testing.T) {
	s, child, scanner, _, turn := startEventTurn(t)
	sendNotification(t, child, "item/started", `{"threadId":"thread-1","turnId":"turn-1","item":{"id":"item-1","type":"agentMessage","status":"inProgress"}}`)
	_ = nextEvent(t, s)
	done := beginCancellation(t, s, scanner, cancellationIntent(turn))
	acknowledgeInterrupt(t, child, 4, `{}`)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	sendNotification(t, child, "turn/completed", `{"threadId":"thread-1","turn":{"id":"turn-1","status":"interrupted"}}`)
	if event := nextEvent(t, s); event.State != providersession.StateCancelled || event.Summary != "cancelled" {
		t.Fatalf("active-item cancellation terminal = %+v", event)
	}
}

func TestCancellationRejectsDuplicateStaleReorderedAndCrossCorrelatedMessages(t *testing.T) {
	for _, fixture := range []struct {
		name   string
		apply  func(*testing.T, *Supervisor, *fakeChild, *bufio.Scanner, LifecycleReference)
		reason DisconnectReason
	}{
		{"duplicate", func(t *testing.T, s *Supervisor, child *fakeChild, scanner *bufio.Scanner, turn LifecycleReference) {
			done := beginCancellation(t, s, scanner, cancellationIntent(turn))
			acknowledgeInterrupt(t, child, 4, `{"thread":{"id":"thread-1"},"turn":{"id":"turn-1","status":"inProgress"}}`)
			if err := <-done; err != nil {
				t.Fatal(err)
			}
			if err := s.Cancel(context.Background(), cancellationIntent(turn)); !errors.Is(err, ErrCancellationRejected) {
				t.Fatalf("duplicate cancellation error = %v", err)
			}
		}, DisconnectEventOrdering},
		{"cross_process", func(t *testing.T, s *Supervisor, _ *fakeChild, _ *bufio.Scanner, turn LifecycleReference) {
			intent := cancellationIntent(turn)
			intent.Correlation.ProcessIncarnationID = "other-process"
			_ = s.Cancel(context.Background(), intent)
		}, DisconnectCorrelationMismatch},
		{"cross_connection", func(t *testing.T, s *Supervisor, _ *fakeChild, _ *bufio.Scanner, turn LifecycleReference) {
			intent := cancellationIntent(turn)
			intent.Correlation.ConnectionID = "other-connection"
			_ = s.Cancel(context.Background(), intent)
		}, DisconnectCorrelationMismatch},
		{"cross_thread", func(t *testing.T, s *Supervisor, _ *fakeChild, _ *bufio.Scanner, turn LifecycleReference) {
			intent := cancellationIntent(turn)
			intent.Session.SessionID, intent.Correlation.SessionID = "thread-2", "thread-2"
			_ = s.Cancel(context.Background(), intent)
		}, DisconnectCorrelationMismatch},
		{"cross_turn", func(t *testing.T, s *Supervisor, _ *fakeChild, _ *bufio.Scanner, turn LifecycleReference) {
			intent := cancellationIntent(turn)
			intent.Correlation.InteractionID = "turn-2"
			_ = s.Cancel(context.Background(), intent)
		}, DisconnectCorrelationMismatch},
		{"stale_reference", func(t *testing.T, s *Supervisor, _ *fakeChild, _ *bufio.Scanner, turn LifecycleReference) {
			intent := cancellationIntent(turn)
			intent.Correlation.ActivityID = "stale-item"
			_ = s.Cancel(context.Background(), intent)
		}, DisconnectCorrelationMismatch},
		{"uncorrelated", func(t *testing.T, s *Supervisor, _ *fakeChild, _ *bufio.Scanner, turn LifecycleReference) {
			intent := cancellationIntent(turn)
			intent.Correlation.ProcessIncarnationID = ""
			_ = s.Cancel(context.Background(), intent)
		}, DisconnectCorrelationMismatch},
		{"cross_item", func(t *testing.T, s *Supervisor, child *fakeChild, scanner *bufio.Scanner, turn LifecycleReference) {
			_ = beginCancellation(t, s, scanner, cancellationIntent(turn))
			sendNotification(t, child, "item/updated", `{"threadId":"thread-1","turnId":"turn-1","item":{"id":"item-2","type":"agentMessage","status":"inProgress"}}`)
		}, DisconnectCorrelationMismatch},
		{"reordered_terminal", func(t *testing.T, s *Supervisor, child *fakeChild, scanner *bufio.Scanner, turn LifecycleReference) {
			_ = beginCancellation(t, s, scanner, cancellationIntent(turn))
			sendNotification(t, child, "turn/completed", `{"threadId":"thread-1","turn":{"id":"turn-1","status":"interrupted"}}`)
		}, DisconnectEventOrdering},
		{"malformed", func(t *testing.T, s *Supervisor, _ *fakeChild, _ *bufio.Scanner, turn LifecycleReference) {
			intent := cancellationIntent(turn)
			intent.Reason = "private reason"
			_ = s.Cancel(context.Background(), intent)
		}, DisconnectCancellationRejected},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			s, child, scanner, _, turn := startEventTurn(t)
			fixture.apply(t, s, child, scanner, turn)
			if event := nextEvent(t, s); event.State != providersession.StateDisconnected || event.Summary != string(fixture.reason) {
				t.Fatalf("disconnect event = %+v", event)
			}
		})
	}
}

func TestCancellationRejectsInterruptMismatchAndNonInterruptedTerminal(t *testing.T) {
	t.Run("response_mismatch", func(t *testing.T) {
		s, child, scanner, _, turn := startEventTurn(t)
		done := beginCancellation(t, s, scanner, cancellationIntent(turn))
		acknowledgeInterrupt(t, child, 4, `{"thread":{"id":"thread-1"},"turn":{"id":"turn-2","status":"inProgress"}}`)
		if err := <-done; !errors.Is(err, ErrCancellationRejected) {
			t.Fatalf("mismatch error = %v", err)
		}
		if event := nextEvent(t, s); event.State != providersession.StateDisconnected || event.Summary != string(DisconnectCorrelationMismatch) {
			t.Fatalf("mismatch event = %+v", event)
		}
	})
	t.Run("non_interrupted_terminal", func(t *testing.T) {
		s, child, scanner, _, turn := startEventTurn(t)
		done := beginCancellation(t, s, scanner, cancellationIntent(turn))
		acknowledgeInterrupt(t, child, 4, `{"thread":{"id":"thread-1"},"turn":{"id":"turn-1","status":"inProgress"}}`)
		if err := <-done; err != nil {
			t.Fatal(err)
		}
		sendNotification(t, child, "turn/completed", `{"threadId":"thread-1","turn":{"id":"turn-1","status":"completed"}}`)
		if event := nextEvent(t, s); event.State != providersession.StateDisconnected || event.Summary != string(DisconnectCancellationRejected) {
			t.Fatalf("non-interrupted event = %+v", event)
		}
	})
}

func TestCancellationFailsClosedOnTimeoutMissingTerminalTransportChildExitProviderErrorAndReroute(t *testing.T) {
	t.Run("interrupt_timeout", func(t *testing.T) {
		s, child, scanner, _, turn := startEventTurn(t)
		s.deadlines.Request = 20 * time.Millisecond
		done := beginCancellation(t, s, scanner, cancellationIntent(turn))
		if err := <-done; !errors.Is(err, ErrCancellationUnavailable) {
			t.Fatalf("timeout error = %v", err)
		}
		if event := nextEvent(t, s); event.State != providersession.StateDisconnected || event.Summary != string(DisconnectRequestDeadline) {
			t.Fatalf("timeout event = %+v", event)
		}
		waitForCancellationKill(t, child)
	})
	t.Run("missing_terminal", func(t *testing.T) {
		s, child, scanner, _, turn := startEventTurn(t)
		s.deadlines.Request = 20 * time.Millisecond
		done := beginCancellation(t, s, scanner, cancellationIntent(turn))
		acknowledgeInterrupt(t, child, 4, `{"thread":{"id":"thread-1"},"turn":{"id":"turn-1","status":"inProgress"}}`)
		if err := <-done; err != nil {
			t.Fatal(err)
		}
		if event := nextEvent(t, s); event.State != providersession.StateDisconnected || event.Summary != string(DisconnectRequestDeadline) {
			t.Fatalf("missing-terminal event = %+v", event)
		}
		waitForCancellationKill(t, child)
	})
	for _, fixture := range []struct {
		name   string
		apply  func(*testing.T, *fakeChild)
		reason DisconnectReason
	}{
		{"transport", func(_ *testing.T, child *fakeChild) { _ = child.stdoutW.Close() }, DisconnectTransportClosed},
		{"child_exit", func(_ *testing.T, child *fakeChild) { child.exit(errors.New("lost")) }, DisconnectChildExit},
		{"provider_error", func(t *testing.T, child *fakeChild) { sendServerRequestError(t, child, 4) }, DisconnectProviderError},
		{"provider_error_notification", func(t *testing.T, child *fakeChild) {
			sendNotification(t, child, "error", `{"threadId":"thread-1","turnId":"turn-1","error":{"codexErrorInfo":"other","message":"private"}}`)
		}, DisconnectProviderError},
		{"reroute", func(t *testing.T, child *fakeChild) {
			sendNotification(t, child, "model/rerouted", `{"threadId":"thread-1","turnId":"turn-1"}`)
		}, DisconnectModelRerouted},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			s, child, scanner, _, turn := startEventTurn(t)
			_ = beginCancellation(t, s, scanner, cancellationIntent(turn))
			fixture.apply(t, child)
			if event := nextEvent(t, s); event.State != providersession.StateDisconnected || event.Summary != string(fixture.reason) {
				t.Fatalf("disconnect event = %+v", event)
			}
		})
	}
}

func TestCancellationItemTransitionRemainsNonTerminal(t *testing.T) {
	s, child, scanner, _, turn := startApprovalTurn(t)
	done := beginCancellation(t, s, scanner, cancellationIntent(turn))
	acknowledgeInterrupt(t, child, 4, `{"thread":{"id":"thread-1"},"turn":{"id":"turn-1","status":"inProgress"}}`)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	sendNotification(t, child, "item/completed", `{"threadId":"thread-1","turnId":"turn-1","item":{"id":"item-1","type":"commandExecution","status":"completed"}}`)
	if event := nextEvent(t, s); event.Kind != providersession.EventProgress || event.Summary != "item_completed" || s.State() != providersession.StateRunning {
		t.Fatalf("item transition claimed cancellation completion: %+v, state=%s", event, s.State())
	}
	sendNotification(t, child, "turn/completed", `{"threadId":"thread-1","turn":{"id":"turn-1","status":"interrupted"}}`)
	if event := nextEvent(t, s); event.State != providersession.StateCancelled {
		t.Fatalf("terminal cancellation event = %+v", event)
	}
}

func waitForCancellationKill(t *testing.T, child *fakeChild) {
	t.Helper()
	deadline := time.After(time.Second)
	for child.killed() == 0 {
		select {
		case <-deadline:
			t.Fatal("cancellation failure did not use bounded kill escalation")
		case <-time.After(time.Millisecond):
		}
	}
}

func TestCancellationRedactsPrivateDataAndProjectsOnlyBackgroundRisk(t *testing.T) {
	s, child, scanner, _, turn := startEventTurn(t)
	intent := cancellationIntent(turn)
	done := beginCancellation(t, s, scanner, intent)
	acknowledgeInterrupt(t, child, 4, `{"thread":{"id":"thread-1"},"turn":{"id":"turn-1","status":"inProgress"},"backgroundProcesses":[{"command":"private command","pid":42}]}`)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	event := nextEvent(t, s)
	data, _ := json.Marshal(event)
	if event.Kind != providersession.EventProgress || event.Summary != "background_process_risk_possible" || strings.Contains(string(data), "private") || strings.Contains(string(data), "command") {
		t.Fatalf("background-process projection leaked private data: %+v", event)
	}
	sendNotification(t, child, "turn/completed", `{"threadId":"thread-1","turn":{"id":"turn-1","status":"interrupted"}}`)
	if event := nextEvent(t, s); event.State != providersession.StateCancelled {
		t.Fatalf("cancelled event = %+v", event)
	}
}
