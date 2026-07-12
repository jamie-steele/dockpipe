package appserversupervisor

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"

	"dorkpipe.orchestrator/providersession"
)

const (
	snapshotVersion  = 1
	maxSnapshotBytes = 4096
)

// SnapshotStore is package-local recovery storage. Its values are always the
// bounded JSON emitted by Supervisor; callers must not use it for RPC data.
type SnapshotStore interface {
	Load(context.Context, string) ([]byte, error)
	Save(context.Context, string, []byte) error
}

// FileSnapshotStore is a small atomic package-owned store. The root is chosen
// by the package runtime rather than derived from provider state.
type FileSnapshotStore struct{ Root string }

func (f FileSnapshotStore) Load(ctx context.Context, evidence string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	path, err := f.path(evidence)
	if err != nil {
		return nil, err
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxSnapshotBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) == 0 || len(data) > maxSnapshotBytes {
		return nil, errors.New("snapshot is not bounded")
	}
	return data, nil
}

var snapshotTemporaryFiles atomic.Uint64

func (f FileSnapshotStore) Save(ctx context.Context, evidence string, data []byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if len(data) == 0 || len(data) > maxSnapshotBytes {
		return errors.New("snapshot is not bounded")
	}
	if _, err := parseSnapshot(data, evidence); err != nil {
		return errors.New("snapshot is unsafe")
	}
	path, err := f.path(evidence)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	temporary := path + fmt.Sprintf(".tmp-%d", snapshotTemporaryFiles.Add(1))
	file, err := os.OpenFile(temporary, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	if _, err := file.Write(data); err == nil {
		err = file.Sync()
	}
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		_ = os.Remove(temporary)
		return err
	}
	if err := os.Rename(temporary, path); err != nil {
		_ = os.Remove(temporary)
		return err
	}
	return nil
}

func (f FileSnapshotStore) path(evidence string) (string, error) {
	if strings.TrimSpace(f.Root) == "" || len(evidence) == 0 || len(evidence) > 128 {
		return "", errors.New("safe recovery store root and evidence are required")
	}
	for _, character := range evidence {
		if !(character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z' || character >= '0' && character <= '9' || character == '-' || character == '_' || character == '.') {
			return "", errors.New("recovery evidence is unsafe")
		}
	}
	sum := sha256.Sum256([]byte(evidence))
	return filepath.Join(filepath.Clean(f.Root), hex.EncodeToString(sum[:])+".json"), nil
}

type recoverySnapshot struct {
	Version     int                        `json:"version"`
	Evidence    string                     `json:"evidence"`
	Session     providersession.SessionRef `json:"session"`
	Policy      string                     `json:"policy_fingerprint"`
	Lifecycle   string                     `json:"lifecycle_class"`
	Process     string                     `json:"prior_process_incarnation"`
	Connection  string                     `json:"prior_connection"`
	EventCursor uint64                     `json:"event_cursor"`
	NextCursor  uint64                     `json:"next_event_cursor"`
	SafeSummary string                     `json:"safe_summary"`
}

func (s *Supervisor) persistIdle(eventCursor uint64) DisconnectReason {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.store == nil {
		return ""
	}
	if s.state == providersession.StateDisconnected || s.lifecycle.threadStatus != "idle" || s.lifecycle.active || s.lifecycle.turnID != "" || s.lifecycle.itemID != "" || s.lifecycle.pending != nil || s.lifecycle.cancellation != nil || s.lifecycle.policyKey == [sha256.Size]byte{} || eventCursor == 0 || eventCursor != s.sequence {
		return DisconnectPersistenceFailure
	}
	snapshot := recoverySnapshot{Version: snapshotVersion, Evidence: s.recoveryEvidence, Session: s.session, Policy: hex.EncodeToString(s.lifecycle.policyKey[:]), Lifecycle: "idle", Process: s.processRef, Connection: s.connectionRef, EventCursor: eventCursor, NextCursor: eventCursor + 1, SafeSummary: "thread_idle"}
	data, err := json.Marshal(snapshot)
	if err != nil || len(data) == 0 || len(data) > maxSnapshotBytes {
		return DisconnectPersistenceFailure
	}
	if s.audit == nil {
		return DisconnectAuditFailure
	}
	if err := s.audit.append(context.Background(), AuditRecord{Version: auditSchemaVersion, Operation: "persistence", Outcome: "completed", Lifecycle: "idle", Summary: "thread_idle", Session: s.session, Progress: auditProgress(s.sequence), Latency: "none"}); err != nil {
		return DisconnectAuditFailure
	}
	if err := s.store.Save(context.Background(), snapshot.Evidence, data); err != nil {
		return DisconnectPersistenceFailure
	}
	return ""
}

func parseSnapshot(data []byte, evidence string) (recoverySnapshot, error) {
	var snapshot recoverySnapshot
	if len(data) == 0 || len(data) > maxSnapshotBytes || json.Unmarshal(data, &snapshot) != nil {
		return recoverySnapshot{}, errors.New("invalid snapshot")
	}
	if snapshot.Version != snapshotVersion || snapshot.Evidence != evidence || snapshot.Lifecycle != "idle" || snapshot.SafeSummary != "thread_idle" || snapshot.EventCursor == 0 || snapshot.NextCursor != snapshot.EventCursor+1 {
		return recoverySnapshot{}, errors.New("unsupported snapshot")
	}
	if err := validateSupervisorSession(snapshot.Session); err != nil || !validID(snapshot.Process) || !validID(snapshot.Connection) || len(snapshot.Policy) != sha256.Size*2 {
		return recoverySnapshot{}, errors.New("unsafe snapshot")
	}
	if _, err := hex.DecodeString(snapshot.Policy); err != nil {
		return recoverySnapshot{}, errors.New("unsafe snapshot")
	}
	return snapshot, nil
}

var ErrRecoveryRejected = errors.New("app server recovery was rejected")

// Recover never reuses a prior child or continues prior work. It loads only a
// safe idle snapshot, launches a fresh initialized child, then proves the
// persisted thread is idle through one exact private reconciliation read.
func (s *Supervisor) Recover(ctx context.Context, request providersession.RecoveryRequest, policy LifecyclePolicy) (LifecycleReference, error) {
	if err := request.Validate(); err != nil || policy.validate() != nil {
		s.recoveryRequired("recovery_invalid")
		return LifecycleReference{}, ErrRecoveryRejected
	}
	s.mu.RLock()
	fresh, configured, expected := !s.started && !s.initialized, s.store != nil, s.session
	s.mu.RUnlock()
	if !fresh || !configured || request.Session != expected {
		s.recoveryRequired("recovery_invalid")
		return LifecycleReference{}, ErrRecoveryRejected
	}
	data, err := s.store.Load(ctx, request.RecoveryEvidence)
	if err != nil {
		s.recoveryRequired("snapshot_missing")
		return LifecycleReference{}, ErrRecoveryRejected
	}
	snapshot, err := parseSnapshot(data, request.RecoveryEvidence)
	policyKey := policy.key()
	if err != nil || snapshot.Session != request.Session || snapshot.Policy != hex.EncodeToString(policyKey[:]) {
		s.recoveryRequired("snapshot_rejected")
		return LifecycleReference{}, ErrRecoveryRejected
	}
	if s.audit == nil || s.audit.recoverCursor(ctx, request.RecoveryEvidence, snapshot.Session, snapshot.EventCursor) != nil {
		s.recoveryRequired("audit_rejected")
		return LifecycleReference{}, ErrRecoveryRejected
	}
	s.mu.Lock()
	if snapshot.Process == s.processRef || snapshot.Connection == s.connectionRef {
		s.mu.Unlock()
		s.recoveryRequired("snapshot_rejected")
		return LifecycleReference{}, ErrRecoveryRejected
	}
	s.sequence, s.recoveryEvidence = snapshot.EventCursor, request.RecoveryEvidence
	s.lifecycle.threadID, s.lifecycle.policyKey, s.lifecycle.threadStatus = snapshot.Session.SessionID, policyKey, ""
	s.lifecycle.declaredRoots = map[string]bool{}
	for _, root := range policy.WritableRoots {
		s.lifecycle.declaredRoots[filepath.Clean(root)] = true
	}
	s.mu.Unlock()
	if err := s.start(ctx, false); err != nil {
		return LifecycleReference{}, ErrRecoveryRejected
	}
	client, ready := s.lifecycleReady()
	if !ready {
		s.fail(DisconnectLifecycleRejected)
		return LifecycleReference{}, ErrRecoveryRejected
	}
	params := policy.params()
	params["threadId"], params["includeTurns"] = snapshot.Session.SessionID, false
	result, err := s.lifecycleRequest(ctx, client, "thread/read", params)
	if err != nil {
		s.fail(client.failureReason())
		return LifecycleReference{}, ErrRecoveryRejected
	}
	if reason := projectIdleReconciliation(result, snapshot.Session.SessionID); reason != "" {
		s.fail(reason)
		return LifecycleReference{}, ErrRecoveryRejected
	}
	s.mu.Lock()
	if s.state == providersession.StateDisconnected || s.session != snapshot.Session || s.lifecycle.active || s.lifecycle.pending != nil || s.lifecycle.cancellation != nil {
		s.mu.Unlock()
		s.fail(DisconnectEventOrdering)
		return LifecycleReference{}, ErrRecoveryRejected
	}
	s.lifecycle.threadStatus = "idle"
	s.mu.Unlock()
	if !s.emit(providersession.StateReady, "recovered_idle", "recovery", "completed") {
		return LifecycleReference{}, ErrRecoveryRejected
	}
	return s.lifecycleReference(""), nil
}

func projectIdleReconciliation(result json.RawMessage, expectedThread string) DisconnectReason {
	if containsModelReroute(result) {
		return DisconnectModelRerouted
	}
	var response struct {
		Thread struct {
			ID     string `json:"id"`
			Status string `json:"status"`
		} `json:"thread"`
	}
	if json.Unmarshal(result, &response) != nil || !validID(response.Thread.ID) || response.Thread.ID != expectedThread {
		return DisconnectCorrelationMismatch
	}
	if response.Thread.Status != "idle" {
		return DisconnectUnsupportedLifecycle
	}
	return ""
}

func (s *Supervisor) recoveryRequired(summary string) {
	s.mu.Lock()
	if s.state == providersession.StateDisconnected {
		s.mu.Unlock()
		return
	}
	stdin, stdout := s.stdin, s.stdout
	s.stdin, s.stdout = nil, nil
	s.clearPrivateStateLocked()
	s.state, s.sequence = providersession.StateDisconnected, s.sequence+1
	event := providersession.Event{ContractVersion: providersession.ContractVersion, Sequence: s.sequence, OccurredAt: nowUTC(), Session: s.session, Kind: providersession.EventRecoveryRequired, Summary: summary}
	s.mu.Unlock()
	if stdin != nil {
		_ = stdin.Close()
	}
	if stdout != nil {
		_ = stdout.Close()
	}
	s.publish(event, "recovery", "rejected", "disconnected")
	s.startShutdown()
}
