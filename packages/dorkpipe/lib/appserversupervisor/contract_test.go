package appserversupervisor

import (
	"context"
	"errors"
	"testing"

	"dorkpipe.orchestrator/providersession"
)

// assertCAS12FailClosed verifies the observable boundary after a rejected
// fixture frame or operation. It deliberately checks private state here so a
// later refactor cannot leave a usable route behind a disconnected projection.
func assertCAS12FailClosed(t *testing.T, s *Supervisor, policy LifecyclePolicy) {
	t.Helper()
	s.mu.RLock()
	state, stdin, stdout, client, lifecycle := s.state, s.stdin, s.stdout, s.client, s.lifecycle
	s.mu.RUnlock()
	if state != providersession.StateDisconnected {
		t.Fatalf("state = %s, want disconnected", state)
	}
	if stdin != nil || stdout != nil || client != nil {
		t.Fatal("rejected fixture retained usable private transport or client")
	}
	if lifecycle.active || lifecycle.pending != nil || lifecycle.cancellation != nil || lifecycle.threadID != "" || lifecycle.turnID != "" || lifecycle.itemID != "" || lifecycle.steerable || lifecycle.requestCounter != 0 {
		t.Fatalf("rejected fixture retained active lifecycle state: %+v", lifecycle)
	}
	if err := s.Start(context.Background()); !errors.Is(err, ErrAlreadyStarted) {
		t.Fatalf("disconnected supervisor restart error = %v", err)
	}
	if _, err := s.StartThread(context.Background(), policy); !errors.Is(err, ErrLifecycleRejected) {
		t.Fatalf("disconnected supervisor lifecycle error = %v", err)
	}
}

func TestCAS12FixtureContractsRejectAndClearEveryLiveRoute(t *testing.T) {
	t.Run("initialization_duplicate_field", func(t *testing.T) {
		child := newFakeChild()
		s := newTestSupervisor(t, fakeLauncher{start: func(context.Context) (Child, error) { return child, nil }}, testDeadlines())
		done, id := beginInitialize(t, s, child)
		_, _ = child.stdoutW.Write([]byte(response(id, `{"protocolVersion":"v2","protocolVersion":"v2","serverInfo":{"name":"codex","version":"0.144.1"},"capabilities":{"stableV2":true}}`)))
		if err := <-done; err == nil {
			t.Fatal("duplicate initialization field was accepted")
		}
		if event := nextEvent(t, s); event.State != providersession.StateDisconnected || event.Summary != string(DisconnectUnsupportedSchema) {
			t.Fatalf("initialization rejection = %+v", event)
		}
		assertCAS12FailClosed(t, s, testLifecyclePolicy(t))
	})

	t.Run("stale_lifecycle_reference", func(t *testing.T) {
		s, child, scanner, policy := initializedLifecycle(t)
		thread := startThreadForTest(t, s, child, scanner, policy)
		thread.Correlation.ConnectionID = "stale-connection"
		if _, err := s.ReadThread(context.Background(), thread, policy); !errors.Is(err, ErrLifecycleRejected) {
			t.Fatalf("stale lifecycle reference error = %v", err)
		}
		_ = nextEvent(t, s) // initialized
		if event := nextEvent(t, s); event.Summary != string(DisconnectLifecycleRejected) {
			t.Fatalf("stale lifecycle rejection = %+v", event)
		}
		assertCAS12FailClosed(t, s, policy)
	})

	t.Run("out_of_order_event", func(t *testing.T) {
		s, child, _, policy, _ := startEventTurn(t)
		sendNotification(t, child, "turn/started", `{"threadId":"thread-1","turn":{"id":"turn-1","status":"inProgress"}}`)
		if event := nextEvent(t, s); event.State != providersession.StateDisconnected || event.Summary != string(DisconnectEventOrdering) {
			t.Fatalf("out-of-order event rejection = %+v", event)
		}
		assertCAS12FailClosed(t, s, policy)
	})

	t.Run("cross_process_approval_decision", func(t *testing.T) {
		s, child, _, policy, _ := startApprovalTurn(t)
		event := approvalRequest(t, s, child, 88, "item/commandExecution/requestApproval", `{"threadId":"thread-1","turnId":"turn-1","itemId":"item-1"}`)
		decision := providersession.ApprovalDecision{Correlation: event.Approval.Correlation, Decision: providersession.DecisionDeny}
		decision.Correlation.ProcessIncarnationID = "other-process"
		if err := s.Decide(context.Background(), decision); !errors.Is(err, ErrDecisionRejected) {
			t.Fatalf("cross-process decision error = %v", err)
		}
		if disconnected := nextEvent(t, s); disconnected.State != providersession.StateDisconnected || disconnected.Summary != string(DisconnectCorrelationMismatch) {
			t.Fatalf("cross-process decision rejection = %+v", disconnected)
		}
		assertCAS12FailClosed(t, s, policy)
	})

	t.Run("cross_connection_cancellation", func(t *testing.T) {
		s, _, _, policy, turn := startEventTurn(t)
		intent := cancellationIntent(turn)
		intent.Correlation.ConnectionID = "other-connection"
		if err := s.Cancel(context.Background(), intent); !errors.Is(err, ErrCancellationRejected) {
			t.Fatalf("cross-connection cancellation error = %v", err)
		}
		if disconnected := nextEvent(t, s); disconnected.State != providersession.StateDisconnected || disconnected.Summary != string(DisconnectCorrelationMismatch) {
			t.Fatalf("cross-connection cancellation rejection = %+v", disconnected)
		}
		assertCAS12FailClosed(t, s, policy)
	})
}

func TestCAS12FixtureRecoveryAndAuditRemainIdleOnlyAndAtomic(t *testing.T) {
	policy := recoveryPolicy(t)
	session := providersession.SessionRef{Provider: "test", SessionID: "thread-1"}
	evidence := "recovery-safe"
	store := &memorySnapshotStore{}
	snapshot := validRecoverySnapshot(policy, session, evidence)
	snapshot.Lifecycle = "waiting_approval"
	saveRecoverySnapshot(t, store, snapshot)
	starts := 0
	s, err := NewWithSnapshotStore(session, fakeLauncher{start: func(context.Context) (Child, error) {
		starts++
		return nil, errors.New("fixture must not launch")
	}}, testDeadlines(), testInitialization(), store)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Recover(context.Background(), providersession.RecoveryRequest{Session: session, RecoveryEvidence: evidence}, policy); !errors.Is(err, ErrRecoveryRejected) {
		t.Fatalf("active recovery error = %v", err)
	}
	if starts != 0 {
		t.Fatal("active snapshot launched a child")
	}
	if event := nextEvent(t, s); event.Kind != providersession.EventRecoveryRequired || event.Summary != "snapshot_rejected" {
		t.Fatalf("active recovery projection = %+v", event)
	}
	assertCAS12FailClosed(t, s, policy)

	journal := newAuditJournal(session, "audit-safe", &memoryAuditStore{})
	if err := journal.append(context.Background(), auditTestRecord(session, 1)); err != nil {
		t.Fatal(err)
	}
	if err := journal.append(context.Background(), auditTestRecord(session, 3)); err == nil {
		t.Fatal("gapped audit cursor was accepted")
	}
}
