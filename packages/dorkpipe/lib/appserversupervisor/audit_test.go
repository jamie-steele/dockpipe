package appserversupervisor

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"dorkpipe.orchestrator/providersession"
)

type failingAuditStore struct {
	memoryAuditStore
	err error
}

func (s *failingAuditStore) Save(ctx context.Context, evidence string, data []byte) error {
	if s.err != nil {
		return s.err
	}
	return s.memoryAuditStore.Save(ctx, evidence, data)
}

func auditTestRecord(session providersession.SessionRef, event uint64) AuditRecord {
	return AuditRecord{Version: auditSchemaVersion, EventSequence: event, Operation: "event", Outcome: "completed", Lifecycle: "ready", Summary: "initialized", Session: session, Progress: "low", Latency: "none"}
}

func TestAuditJournalIsBoundedContiguousAndRedacted(t *testing.T) {
	session := providersession.SessionRef{Provider: "test", SessionID: "session"}
	store := &memoryAuditStore{}
	journal := newAuditJournal(session, "audit-safe", store)
	for sequence := uint64(1); sequence <= maxAuditRecords+2; sequence++ {
		if err := journal.append(context.Background(), auditTestRecord(session, sequence)); err != nil {
			t.Fatal(err)
		}
	}
	data, err := store.Load(context.Background(), "audit-safe")
	if err != nil || len(data) > maxAuditBytes {
		t.Fatalf("audit document = %d, %v", len(data), err)
	}
	var document auditDocument
	if json.Unmarshal(data, &document) != nil || document.LastEvent != maxAuditRecords+2 || len(document.Segments) != 2 {
		t.Fatalf("document = %+v", document)
	}
	for _, forbidden := range []string{"jsonrpc", "prompt", "command", "patch", "path", "credential", "token text", "provider error", "process detail"} {
		if strings.Contains(strings.ToLower(string(data)), forbidden) {
			t.Fatalf("audit leaked %q: %s", forbidden, data)
		}
	}
}

func TestAuditJournalRejectsDuplicateGapCrossSessionAndCorruptEvidence(t *testing.T) {
	session := providersession.SessionRef{Provider: "test", SessionID: "session"}
	store := &memoryAuditStore{}
	journal := newAuditJournal(session, "audit-safe", store)
	if err := journal.append(context.Background(), auditTestRecord(session, 1)); err != nil {
		t.Fatal(err)
	}
	if err := journal.append(context.Background(), auditTestRecord(session, 1)); err == nil {
		t.Fatal("duplicate event evidence was accepted")
	}
	if err := journal.append(context.Background(), auditTestRecord(session, 3)); err == nil {
		t.Fatal("gapped event evidence was accepted")
	}
	cross := auditTestRecord(providersession.SessionRef{Provider: "test", SessionID: "other"}, 2)
	if err := journal.append(context.Background(), cross); err == nil {
		t.Fatal("cross-session evidence was accepted")
	}
	store.data["audit-safe"] = []byte(`{"version":1`)
	other := newAuditJournal(session, "audit-safe", store)
	if err := other.append(context.Background(), auditTestRecord(session, 2)); err == nil {
		t.Fatal("partial evidence was accepted")
	}
}

func TestAuditStoreRejectsOversizedAndWriteFailure(t *testing.T) {
	session := providersession.SessionRef{Provider: "test", SessionID: "session"}
	store := &failingAuditStore{err: errors.New("write failed")}
	journal := newAuditJournal(session, "audit-safe", store)
	if err := journal.append(context.Background(), auditTestRecord(session, 1)); err == nil {
		t.Fatal("write failure was accepted")
	}
	file := FileAuditStore{Root: t.TempDir()}
	if err := file.Save(context.Background(), "audit-safe", []byte(strings.Repeat("x", maxAuditBytes+1))); err == nil {
		t.Fatal("oversized document was accepted")
	}
	if _, err := file.Load(context.Background(), "audit-safe"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("interrupted write left evidence: %v", err)
	}
}

func TestAuditProjectionRejectsUnsafeClassesAndFailsClosed(t *testing.T) {
	session := providersession.SessionRef{Provider: "test", SessionID: "session"}
	journal := newAuditJournal(session, "audit-safe", &memoryAuditStore{})
	unsafe := auditTestRecord(session, 1)
	unsafe.Summary = "private_prompt"
	if err := journal.append(context.Background(), unsafe); err == nil {
		t.Fatal("unsafe summary was accepted")
	}
	cross := auditTestRecord(session, 1)
	cross.Correlation.SessionID = "other"
	if err := journal.append(context.Background(), cross); err == nil {
		t.Fatal("cross-session correlation was accepted")
	}
	if auditLatency(time.Time{}) != "none" || auditProgress(1) != "low" || auditProgress(1000) != "very_high" {
		t.Fatal("safe timing or progress buckets changed")
	}

	child := newFakeChild()
	failing := &failingAuditStore{err: errors.New("write failed")}
	s, err := NewWithStores(session, fakeLauncher{start: func(context.Context) (Child, error) { return child, nil }}, testDeadlines(), testInitialization(), nil, failing)
	if err != nil {
		t.Fatal(err)
	}
	if err := startInitialized(t, s, child, ""); err == nil || s.State() != providersession.StateDisconnected {
		t.Fatalf("audit write failure did not fail closed: %v, %s", err, s.State())
	}
	if event := nextEvent(t, s); event.Summary != string(DisconnectAuditFailure) {
		t.Fatalf("event = %+v", event)
	}
}
