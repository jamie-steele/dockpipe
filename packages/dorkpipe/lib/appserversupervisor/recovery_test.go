package appserversupervisor

import (
	"bufio"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"dorkpipe.orchestrator/providersession"
)

type memorySnapshotStore struct {
	mu   sync.Mutex
	data map[string][]byte
	err  error
}

func (m *memorySnapshotStore) Load(_ context.Context, evidence string) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.err != nil {
		return nil, m.err
	}
	data, ok := m.data[evidence]
	if !ok {
		return nil, os.ErrNotExist
	}
	return append([]byte(nil), data...), nil
}

func (m *memorySnapshotStore) Save(_ context.Context, evidence string, data []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.err != nil {
		return m.err
	}
	if m.data == nil {
		m.data = map[string][]byte{}
	}
	m.data[evidence] = append([]byte(nil), data...)
	return nil
}

func recoveryPolicy(t *testing.T) LifecyclePolicy { return testLifecyclePolicy(t) }

func validRecoverySnapshot(policy LifecyclePolicy, session providersession.SessionRef, evidence string) recoverySnapshot {
	key := policy.key()
	return recoverySnapshot{Version: snapshotVersion, Evidence: evidence, Session: session, Policy: hex.EncodeToString(key[:]), Lifecycle: "idle", Process: "process-old", Connection: "connection-old", EventCursor: 7, NextCursor: 8, SafeSummary: "thread_idle"}
}

func saveRecoverySnapshot(t *testing.T, store *memorySnapshotStore, snapshot recoverySnapshot) {
	t.Helper()
	data, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if store.data == nil {
		store.data = map[string][]byte{}
	}
	store.data[snapshot.Evidence] = data
}

func saveAuditCursor(t *testing.T, evidence string, session providersession.SessionRef, cursor uint64) {
	t.Helper()
	records := make([]AuditRecord, 0, cursor)
	for sequence := uint64(1); sequence <= cursor; sequence++ {
		records = append(records, AuditRecord{Version: auditSchemaVersion, Sequence: sequence, EventSequence: sequence, Operation: "event", Outcome: "completed", Lifecycle: "idle", Summary: "thread_idle", Session: session, Progress: "low", Latency: "none"})
	}
	document := auditDocument{Version: auditSchemaVersion, Evidence: evidence, Session: session, LastEvent: cursor, Segments: []auditSegment{{FirstSequence: 1, LastSequence: cursor, Records: records}}}
	data, err := json.Marshal(document)
	if err != nil {
		t.Fatal(err)
	}
	if err := defaultAuditStore.Save(context.Background(), evidence, data); err != nil {
		t.Fatal(err)
	}
}

func newStoredIdleSupervisor(t *testing.T, store SnapshotStore) (*Supervisor, providersession.SessionRef, uint64, LifecyclePolicy) {
	t.Helper()
	child := newFakeChild()
	s, err := NewWithSnapshotStore(providersession.SessionRef{Provider: "test", SessionID: "session"}, fakeLauncher{start: func(context.Context) (Child, error) { return child, nil }}, testDeadlines(), testInitialization(), store)
	if err != nil {
		t.Fatal(err)
	}
	if err := startInitialized(t, s, child, ""); err != nil {
		t.Fatal(err)
	}
	scanner := bufio.NewScanner(child.stdinR)
	policy := recoveryPolicy(t)
	thread := startThreadForTest(t, s, child, scanner, policy)
	if event := nextEvent(t, s); event.State != providersession.StateReady {
		t.Fatal(event)
	}
	sendNotification(t, child, "thread/started", `{"thread":{"id":"thread-1"}}`)
	_ = nextEvent(t, s)
	sendNotification(t, child, "thread/status/changed", `{"threadId":"thread-1","status":"active"}`)
	_ = nextEvent(t, s)
	sendNotification(t, child, "thread/status/changed", `{"threadId":"thread-1","status":"idle"}`)
	idle := nextEvent(t, s)
	if idle.Summary != "thread_idle" {
		t.Fatal(idle)
	}
	return s, thread.Session, idle.Sequence, policy
}

func TestRecoveryReconcilesOnlyPersistedIdleSessionOnFreshChild(t *testing.T) {
	store := &memorySnapshotStore{}
	prior, session, cursor, policy := newStoredIdleSupervisor(t, store)
	evidence := prior.RecoveryEvidence()
	data := store.data[evidence]
	if len(data) == 0 {
		t.Fatal("idle snapshot was not written")
	}
	for _, forbidden := range []string{"private command", "private path", "private prompt", "private patch", "jsonrpc", "credential", "question"} {
		if strings.Contains(strings.ToLower(string(data)), strings.ToLower(forbidden)) {
			t.Fatalf("snapshot leaked %q: %s", forbidden, data)
		}
	}

	child := newFakeChild()
	recovered, err := NewWithSnapshotStore(session, fakeLauncher{start: func(context.Context) (Child, error) { return child, nil }}, testDeadlines(), testInitialization(), store)
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan struct {
		ref LifecycleReference
		err error
	}, 1)
	go func() {
		ref, err := recovered.Recover(context.Background(), providersession.RecoveryRequest{Session: session, RecoveryEvidence: evidence}, policy)
		done <- struct {
			ref LifecycleReference
			err error
		}{ref, err}
	}()
	scanner := bufio.NewScanner(child.stdinR)
	if !scanner.Scan() {
		t.Fatal("missing fresh initialize")
	}
	var initialize struct {
		ID     uint64 `json:"id"`
		Method string `json:"method"`
	}
	if err := json.Unmarshal(scanner.Bytes(), &initialize); err != nil || initialize.Method != "initialize" {
		t.Fatalf("initialize = %s, %v", scanner.Text(), err)
	}
	_, _ = child.stdoutW.Write([]byte(response(initialize.ID, `{"userAgent":"codex/0.144.1","codexHome":"C:/codex","platformFamily":"windows","platformOs":"windows"}`)))
	if !scanner.Scan() || !strings.Contains(scanner.Text(), `"initialized"`) {
		t.Fatal("missing fresh initialized notification")
	}
	request := lifecycleRequest(t, scanner, "thread/read", 2)
	params := requestParams(t, request)
	if params["threadId"] != "thread-1" || params["includeTurns"] != false {
		t.Fatalf("reconciliation params = %#v", request)
	}
	_, _ = child.stdoutW.Write([]byte(response(2, `{"thread":{"id":"thread-1","status":"idle"}}`)))
	result := <-done
	if result.err != nil {
		t.Fatal(result.err)
	}
	if result.ref.Correlation.ProcessIncarnationID == prior.processRef || result.ref.Correlation.ConnectionID == prior.connectionRef {
		t.Fatalf("recovery reused prior incarnation: %+v", result.ref)
	}
	event := nextEvent(t, recovered)
	if event.State != providersession.StateReady || event.Summary != "recovered_idle" || event.Sequence != cursor+1 {
		t.Fatalf("recovered event = %+v, cursor=%d", event, cursor)
	}
	if err := providersession.ValidateNextSequence(cursor, event.Sequence); err != nil {
		t.Fatal(err)
	}
}

func TestRecoveryRejectsUnsafeSnapshotsAndEvidenceWithoutLaunching(t *testing.T) {
	policy, session, evidence := recoveryPolicy(t), providersession.SessionRef{Provider: "test", SessionID: "thread-1"}, "recovery-safe"
	for name, mutate := range map[string]func(*recoverySnapshot){
		"active_turn": func(s *recoverySnapshot) { s.Lifecycle = "active" }, "waiting_approval": func(s *recoverySnapshot) { s.Lifecycle = "waiting_approval" },
		"waiting_input": func(s *recoverySnapshot) { s.Lifecycle = "waiting_user_input" }, "pending_cancellation": func(s *recoverySnapshot) { s.Lifecycle = "cancelling" },
		"failed": func(s *recoverySnapshot) { s.SafeSummary = "turn_failed" }, "unknown_terminal": func(s *recoverySnapshot) { s.SafeSummary = "terminal_unknown" },
		"non_idle": func(s *recoverySnapshot) { s.Lifecycle = "running" }, "unsupported_version": func(s *recoverySnapshot) { s.Version++ },
		"cursor_duplicate": func(s *recoverySnapshot) { s.NextCursor = s.EventCursor }, "cursor_stale": func(s *recoverySnapshot) { s.NextCursor = s.EventCursor - 1 }, "cursor_gap": func(s *recoverySnapshot) { s.NextCursor = s.EventCursor + 2 },
	} {
		t.Run(name, func(t *testing.T) {
			store := &memorySnapshotStore{}
			snapshot := validRecoverySnapshot(policy, session, evidence)
			mutate(&snapshot)
			saveRecoverySnapshot(t, store, snapshot)
			starts := 0
			s, err := NewWithSnapshotStore(session, fakeLauncher{start: func(context.Context) (Child, error) { starts++; return nil, errors.New("must not launch") }}, testDeadlines(), testInitialization(), store)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := s.Recover(context.Background(), providersession.RecoveryRequest{Session: session, RecoveryEvidence: evidence}, policy); !errors.Is(err, ErrRecoveryRejected) {
				t.Fatalf("recover error = %v", err)
			}
			if starts != 0 {
				t.Fatal("unsafe snapshot launched a child")
			}
			if event := nextEvent(t, s); event.Kind != providersession.EventRecoveryRequired || s.State() != providersession.StateDisconnected {
				t.Fatalf("event = %+v state=%s", event, s.State())
			}
		})
	}
	for name, data := range map[string][]byte{"corrupt": []byte("{"), "oversized": []byte(strings.Repeat("x", maxSnapshotBytes+1)), "partial": []byte(`{"version":1`)} {
		t.Run(name, func(t *testing.T) {
			store := &memorySnapshotStore{data: map[string][]byte{evidence: data}}
			s, _ := NewWithSnapshotStore(session, fakeLauncher{start: func(context.Context) (Child, error) { t.Fatal("must not launch"); return nil, nil }}, testDeadlines(), testInitialization(), store)
			_, _ = s.Recover(context.Background(), providersession.RecoveryRequest{Session: session, RecoveryEvidence: evidence}, policy)
			if event := nextEvent(t, s); event.Kind != providersession.EventRecoveryRequired {
				t.Fatal(event)
			}
		})
	}
	for name, request := range map[string]providersession.RecoveryRequest{
		"cross_session": {Session: providersession.SessionRef{Provider: "test", SessionID: "thread-2"}, RecoveryEvidence: evidence}, "stale_evidence": {Session: session, RecoveryEvidence: "recovery-other"},
	} {
		t.Run(name, func(t *testing.T) {
			store := &memorySnapshotStore{}
			saveRecoverySnapshot(t, store, validRecoverySnapshot(policy, session, evidence))
			s, _ := NewWithSnapshotStore(request.Session, fakeLauncher{start: func(context.Context) (Child, error) { t.Fatal("must not launch"); return nil, nil }}, testDeadlines(), testInitialization(), store)
			_, _ = s.Recover(context.Background(), request, policy)
			if event := nextEvent(t, s); event.Kind != providersession.EventRecoveryRequired {
				t.Fatal(event)
			}
		})
	}
}

func TestRecoveryReconciliationFailuresDisconnect(t *testing.T) {
	policy, session, evidence := recoveryPolicy(t), providersession.SessionRef{Provider: "test", SessionID: "thread-1"}, "recovery-safe"
	for name, action := range map[string]func(*testing.T, *fakeChild){
		"response_mismatch": func(t *testing.T, c *fakeChild) {
			_, _ = c.stdoutW.Write([]byte(response(2, `{"thread":{"id":"thread-2","status":"idle"}}`)))
		},
		"non_idle": func(t *testing.T, c *fakeChild) {
			_, _ = c.stdoutW.Write([]byte(response(2, `{"thread":{"id":"thread-1","status":"active"}}`)))
		},
		"malformed_response": func(t *testing.T, c *fakeChild) { _, _ = c.stdoutW.Write([]byte(response(2, `[]`))) },
		"reroute": func(t *testing.T, c *fakeChild) {
			_, _ = c.stdoutW.Write([]byte(response(2, `{"thread":{"id":"thread-1","status":"idle"},"modelRerouted":true}`)))
		},
		"provider_error": func(t *testing.T, c *fakeChild) {
			_, _ = c.stdoutW.Write([]byte(`{"jsonrpc":"2.0","id":2,"error":{"message":"private"}}` + "\n"))
		},
		"transport_loss": func(t *testing.T, c *fakeChild) { _ = c.stdoutW.Close() }, "child_exit": func(t *testing.T, c *fakeChild) { c.exit(errors.New("lost")) },
	} {
		t.Run(name, func(t *testing.T) {
			store := &memorySnapshotStore{}
			saveRecoverySnapshot(t, store, validRecoverySnapshot(policy, session, evidence))
			saveAuditCursor(t, evidence, session, 7)
			child := newFakeChild()
			s, _ := NewWithSnapshotStore(session, fakeLauncher{start: func(context.Context) (Child, error) { return child, nil }}, testDeadlines(), testInitialization(), store)
			done := make(chan error, 1)
			go func() {
				_, err := s.Recover(context.Background(), providersession.RecoveryRequest{Session: session, RecoveryEvidence: evidence}, policy)
				done <- err
			}()
			scanner := bufio.NewScanner(child.stdinR)
			scanner.Scan()
			_, _ = child.stdoutW.Write([]byte(response(1, `{"userAgent":"codex/0.144.1","codexHome":"C:/codex","platformFamily":"windows","platformOs":"windows"}`)))
			scanner.Scan()
			_ = lifecycleRequest(t, scanner, "thread/read", 2)
			action(t, child)
			if err := <-done; !errors.Is(err, ErrRecoveryRejected) {
				t.Fatalf("recover error = %v", err)
			}
			if event := nextEvent(t, s); event.State != providersession.StateDisconnected {
				t.Fatalf("event = %+v", event)
			}
		})
	}
	t.Run("timeout", func(t *testing.T) {
		store := &memorySnapshotStore{}
		saveRecoverySnapshot(t, store, validRecoverySnapshot(policy, session, evidence))
		saveAuditCursor(t, evidence, session, 7)
		child := newFakeChild()
		deadlines := testDeadlines()
		deadlines.Request = 20 * time.Millisecond
		s, _ := NewWithSnapshotStore(session, fakeLauncher{start: func(context.Context) (Child, error) { return child, nil }}, deadlines, testInitialization(), store)
		done := make(chan error, 1)
		go func() {
			_, err := s.Recover(context.Background(), providersession.RecoveryRequest{Session: session, RecoveryEvidence: evidence}, policy)
			done <- err
		}()
		scanner := bufio.NewScanner(child.stdinR)
		scanner.Scan()
		_, _ = child.stdoutW.Write([]byte(response(1, `{"userAgent":"codex/0.144.1","codexHome":"C:/codex","platformFamily":"windows","platformOs":"windows"}`)))
		scanner.Scan()
		_ = lifecycleRequest(t, scanner, "thread/read", 2)
		if err := <-done; !errors.Is(err, ErrRecoveryRejected) {
			t.Fatal(err)
		}
		if event := nextEvent(t, s); event.Summary != string(DisconnectRequestDeadline) {
			t.Fatal(event)
		}
	})
}

func TestFileSnapshotStoreUsesFinalBoundedAtomicFile(t *testing.T) {
	store := FileSnapshotStore{Root: t.TempDir()}
	policy := recoveryPolicy(t)
	snapshot := validRecoverySnapshot(policy, providersession.SessionRef{Provider: "test", SessionID: "thread-1"}, "recovery-safe")
	data, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Save(context.Background(), "recovery-safe", data); err != nil {
		t.Fatal(err)
	}
	initial := append([]byte(nil), data...)
	path, err := store.path("recovery-safe")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path+".tmp-interrupted", []byte("partial"), 0o600); err != nil {
		t.Fatal(err)
	}
	data, err = store.Load(context.Background(), "recovery-safe")
	if err != nil || string(data) != string(initial) {
		t.Fatalf("load = %q, %v", data, err)
	}
	snapshot.EventCursor, snapshot.NextCursor = 8, 9
	replacement, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Save(context.Background(), "recovery-safe", replacement); err != nil {
		t.Fatal(err)
	}
	data, err = store.Load(context.Background(), "recovery-safe")
	if err != nil || string(data) != string(replacement) {
		t.Fatalf("replaced load = %q, %v", data, err)
	}
	if err := store.Save(context.Background(), "recovery-safe", []byte(strings.Repeat("x", maxSnapshotBytes+1))); err == nil {
		t.Fatal("oversized snapshot accepted")
	}
}

func TestRecoverySourceKeepsPersistenceAndProtocolBoundaries(t *testing.T) {
	data, err := os.ReadFile("recovery.go")
	if err != nil {
		t.Fatal(err)
	}
	lower := strings.ToLower(string(data))
	for _, forbidden := range []string{"turn/resume", "turn/start", "turn/interrupt", "requestapproval", "commandexecution", "filechange", "credential", "patch", "prompt", "question", "retry", "replay"} {
		if strings.Contains(lower, forbidden) {
			t.Fatalf("recovery source contains forbidden %q", forbidden)
		}
	}
}
