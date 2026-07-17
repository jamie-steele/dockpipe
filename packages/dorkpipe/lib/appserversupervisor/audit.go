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
	"sync"
	"sync/atomic"
	"time"

	"dorkpipe.orchestrator/providersession"
)

// CAS-10 audit evidence is deliberately a package-local, write-only
// projection. It is not an operation queue, recovery input, or replay log.
const (
	auditSchemaVersion  = 1
	maxAuditBytes       = 32 * 1024
	maxAuditRecords     = 64
	maxAuditSegments    = 3
	maxAuditRecordBytes = 1024
)

type AuditRecord struct {
	Version       int                         `json:"version"`
	Sequence      uint64                      `json:"sequence"`
	EventSequence uint64                      `json:"event_sequence,omitempty"`
	Operation     string                      `json:"operation"`
	Outcome       string                      `json:"outcome"`
	Lifecycle     string                      `json:"lifecycle_class"`
	Summary       string                      `json:"summary_class,omitempty"`
	Session       providersession.SessionRef  `json:"session"`
	Correlation   providersession.Correlation `json:"correlation,omitempty"`
	Progress      string                      `json:"progress_bucket,omitempty"`
	Latency       string                      `json:"latency_bucket,omitempty"`
}

type auditSegment struct {
	FirstSequence uint64        `json:"first_sequence"`
	LastSequence  uint64        `json:"last_sequence"`
	Records       []AuditRecord `json:"records"`
}

type auditDocument struct {
	Version    int                        `json:"version"`
	Evidence   string                     `json:"evidence"`
	Session    providersession.SessionRef `json:"session"`
	PriorEvent uint64                     `json:"prior_event_sequence"`
	LastEvent  uint64                     `json:"last_event_sequence"`
	Segments   []auditSegment             `json:"segments"`
}

// AuditStore is intentionally local to appserversupervisor. Implementations
// receive only the bounded audit document, never frames or payloads.
type AuditStore interface {
	Load(context.Context, string) ([]byte, error)
	Save(context.Context, string, []byte) error
}

// FileAuditStore atomically replaces a bounded package-owned audit document.
// The document itself retains a bounded sequence of immutable segments; a
// replacement never mutates an existing published document in place.
type FileAuditStore struct{ Root string }

func (f FileAuditStore) Load(ctx context.Context, evidence string) ([]byte, error) {
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
	data, err := io.ReadAll(io.LimitReader(file, maxAuditBytes+1))
	if err != nil || len(data) == 0 || len(data) > maxAuditBytes {
		return nil, errors.New("audit document is not bounded")
	}
	return data, nil
}

var auditTemporaryFiles atomic.Uint64

func (f FileAuditStore) Save(ctx context.Context, evidence string, data []byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if len(data) == 0 || len(data) > maxAuditBytes {
		return errors.New("audit document is not bounded")
	}
	var document auditDocument
	if json.Unmarshal(data, &document) != nil || document.validate(evidence, document.Session) != nil {
		return errors.New("audit document is unsafe")
	}
	path, err := f.path(evidence)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	temporary := path + fmt.Sprintf(".tmp-%d", auditTemporaryFiles.Add(1))
	file, err := os.OpenFile(temporary, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	if _, err = file.Write(data); err == nil {
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

func (f FileAuditStore) path(evidence string) (string, error) {
	if strings.TrimSpace(f.Root) == "" || !safeEvidence(evidence) {
		return "", errors.New("safe audit store root and evidence are required")
	}
	sum := sha256.Sum256([]byte(evidence))
	return filepath.Join(filepath.Clean(f.Root), hex.EncodeToString(sum[:])+".audit.json"), nil
}

type memoryAuditStore struct {
	mu   sync.Mutex
	data map[string][]byte
}

var defaultAuditStore = &memoryAuditStore{}

func (m *memoryAuditStore) Load(_ context.Context, evidence string) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	data, found := m.data[evidence]
	if !found {
		return nil, os.ErrNotExist
	}
	return append([]byte(nil), data...), nil
}

func (m *memoryAuditStore) Save(_ context.Context, evidence string, data []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.data == nil {
		m.data = map[string][]byte{}
	}
	m.data[evidence] = append([]byte(nil), data...)
	return nil
}

type auditJournal struct {
	mu       sync.Mutex
	store    AuditStore
	evidence string
	session  providersession.SessionRef
	document auditDocument
}

func newAuditJournal(session providersession.SessionRef, evidence string, store AuditStore) *auditJournal {
	if store == nil {
		store = defaultAuditStore
	}
	return &auditJournal{store: store, evidence: evidence, session: session}
}

func (j *auditJournal) append(ctx context.Context, record AuditRecord) error {
	j.mu.Lock()
	defer j.mu.Unlock()
	if err := j.loadLocked(ctx); err != nil {
		return err
	}
	if err := record.validContent(j.session.Provider, true); err != nil {
		return err
	}
	if record.Session != j.session {
		return errors.New("audit record session is not current")
	}
	record.Sequence = j.lastSequenceLocked() + 1
	if record.EventSequence != 0 {
		if record.EventSequence != j.document.LastEvent+1 {
			return errors.New("audit event evidence is not contiguous")
		}
		j.document.LastEvent = record.EventSequence
	}
	data, err := json.Marshal(record)
	if err != nil || len(data) == 0 || len(data) > maxAuditRecordBytes {
		return errors.New("audit record is unsafe")
	}
	if len(j.document.Segments) == 0 || len(j.document.Segments[len(j.document.Segments)-1].Records) == maxAuditRecords {
		j.document.Segments = append(j.document.Segments, auditSegment{FirstSequence: record.Sequence})
		if len(j.document.Segments) > maxAuditSegments {
			for _, retired := range j.document.Segments[0].Records {
				if retired.EventSequence != 0 {
					j.document.PriorEvent = retired.EventSequence
				}
			}
			j.document.Segments = append([]auditSegment(nil), j.document.Segments[len(j.document.Segments)-maxAuditSegments:]...)
		}
	}
	segment := &j.document.Segments[len(j.document.Segments)-1]
	segment.Records = append(segment.Records, record)
	segment.LastSequence = record.Sequence
	encoded, err := json.Marshal(j.document)
	if err != nil || len(encoded) == 0 || len(encoded) > maxAuditBytes {
		return errors.New("audit journal rotation is not bounded")
	}
	return j.store.Save(ctx, j.evidence, encoded)
}

func (j *auditJournal) loadLocked(ctx context.Context) error {
	if j.document.Version != 0 {
		return nil
	}
	data, err := j.store.Load(ctx, j.evidence)
	if errors.Is(err, os.ErrNotExist) {
		j.document = auditDocument{Version: auditSchemaVersion, Evidence: j.evidence, Session: j.session}
		return nil
	}
	if err != nil || len(data) == 0 || len(data) > maxAuditBytes || json.Unmarshal(data, &j.document) != nil {
		return errors.New("audit journal cannot be loaded")
	}
	if err := j.document.validate(j.evidence, j.session); err != nil {
		return err
	}
	return nil
}

func (j *auditJournal) lastSequenceLocked() uint64 {
	if len(j.document.Segments) == 0 {
		return 0
	}
	return j.document.Segments[len(j.document.Segments)-1].LastSequence
}

func (j *auditJournal) updateSession(session providersession.SessionRef) {
	j.mu.Lock()
	j.session = session
	j.mu.Unlock()
}

// recoverCursor binds a fresh supervisor to retained audit evidence before it
// may emit a recovered event. Missing, corrupt, or cursor-mismatched evidence
// is deliberately rejected rather than making prior work appear to survive.
func (j *auditJournal) recoverCursor(ctx context.Context, evidence string, session providersession.SessionRef, cursor uint64) error {
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.document.Version != 0 || !safeEvidence(evidence) {
		return errors.New("audit recovery evidence is ambiguous")
	}
	j.evidence, j.session = evidence, session
	data, err := j.store.Load(ctx, evidence)
	if err != nil || len(data) == 0 || len(data) > maxAuditBytes || json.Unmarshal(data, &j.document) != nil {
		return errors.New("audit recovery evidence is missing")
	}
	if err := j.document.validate(evidence, session); err != nil || j.document.LastEvent != cursor {
		return errors.New("audit recovery cursor is invalid")
	}
	return nil
}

func (r AuditRecord) validContent(provider string, requireUnsetSequence bool) error {
	if r.Version != auditSchemaVersion {
		return errors.New("audit record version is invalid")
	}
	if !validAuditOperation(r.Operation) {
		return errors.New("audit operation is invalid")
	}
	if !validAuditOutcome(r.Outcome) {
		return errors.New("audit outcome is invalid")
	}
	if !validAuditLifecycle(r.Lifecycle) {
		return errors.New("audit lifecycle is invalid")
	}
	if !validAuditSummary(r.Summary) {
		return fmt.Errorf("audit summary is invalid: %q", r.Summary)
	}
	if !validAuditBucket(r.Progress) || !validAuditBucket(r.Latency) {
		return errors.New("audit bucket is invalid")
	}
	if validateSupervisorSession(r.Session) != nil || r.Session.Provider != provider {
		return errors.New("audit session is invalid")
	}
	if !validAuditCorrelation(r.Correlation, r.Session.SessionID) {
		return errors.New("audit correlation is invalid")
	}
	if requireUnsetSequence && r.Sequence != 0 {
		return errors.New("audit sequence is not new")
	}
	return nil
}

func (d auditDocument) validate(evidence string, session providersession.SessionRef) error {
	if d.Version != auditSchemaVersion || !safeEvidence(d.Evidence) || d.Evidence != evidence || validateSupervisorSession(d.Session) != nil || d.Session.Provider != session.Provider || d.PriorEvent > d.LastEvent || len(d.Segments) > maxAuditSegments {
		return errors.New("audit document is invalid")
	}
	var lastSequence uint64
	lastEvent := d.PriorEvent
	for index, segment := range d.Segments {
		if len(segment.Records) == 0 || len(segment.Records) > maxAuditRecords || segment.FirstSequence == 0 || segment.LastSequence < segment.FirstSequence {
			return errors.New("audit segment is invalid")
		}
		if index == 0 {
			lastSequence = segment.FirstSequence - 1
		} else if segment.FirstSequence != lastSequence+1 {
			return errors.New("audit sequence has a gap")
		}
		for _, record := range segment.Records {
			if record.Sequence != lastSequence+1 || record.validContent(session.Provider, false) != nil {
				return errors.New("audit record ordering is invalid")
			}
			lastSequence = record.Sequence
			if record.EventSequence != 0 {
				if record.EventSequence != lastEvent+1 {
					return errors.New("audit event sequence has a gap")
				}
				lastEvent = record.EventSequence
			}
		}
		if segment.LastSequence != lastSequence {
			return errors.New("audit segment cursor is invalid")
		}
	}
	if lastEvent != d.LastEvent {
		return errors.New("audit event cursor is invalid")
	}
	return nil
}

func validAuditOperation(value string) bool {
	switch value {
	case "initialization", "lifecycle", "approval", "user_input", "cancellation", "recovery", "persistence", "disconnect", "shutdown", "event":
		return true
	default:
		return false
	}
}
func validAuditOutcome(value string) bool {
	switch value {
	case "accepted", "completed", "rejected", "expired", "failed", "delivered", "resolved":
		return true
	}
	return false
}
func validAuditLifecycle(value string) bool {
	switch value {
	case "ready", "running", "waiting", "terminal", "disconnected", "idle":
		return true
	}
	return false
}
func validAuditSummary(value string) bool {
	if value == "" {
		return true
	}
	if strings.HasPrefix(value, "token_usage_total_") {
		return len(value) <= 30 && strings.Trim(value[len("token_usage_total_"):], "0123456789") == ""
	}
	for _, prefix := range []string{"thread_", "turn_", "item_", "warning_", "error_", "recovery_", "snapshot_"} {
		if strings.HasPrefix(value, prefix) {
			return safeAuditClass(value)
		}
	}
	switch value {
	case "initialized", "approval_requested", "user_input_requested", "approval_resolved", "cancellation_requested", "background_process_risk_possible", "cancelled", "recovered_idle", "thread_checked", "approval_delivered", "request_expired", "interrupt_delivered", "audit_rejected":
		return true
	default:
		return validAuditDisconnectReason(value)
	}
}

func safeAuditClass(value string) bool {
	if len(value) == 0 || len(value) > 64 {
		return false
	}
	for _, character := range value {
		if !(character >= 'a' && character <= 'z' || character >= '0' && character <= '9' || character == '_') {
			return false
		}
	}
	return true
}

func validAuditDisconnectReason(value string) bool {
	switch DisconnectReason(value) {
	case DisconnectStartupFailure, DisconnectStartupDeadline, DisconnectChildExit, DisconnectTransportClosed, DisconnectMalformedInput, DisconnectLivenessDeadline, DisconnectShutdown, DisconnectRequestDeadline, DisconnectMalformedEnvelope, DisconnectCorrelationMismatch, DisconnectProviderError, DisconnectInitializationRejected, DisconnectUnsupportedSchema, DisconnectUnsupportedCapability, DisconnectModelRerouted, DisconnectPolicyMismatch, DisconnectLifecycleRejected, DisconnectUnsupportedLifecycle, DisconnectUnsupportedEvent, DisconnectEventOrdering, DisconnectDecisionRejected, DisconnectCancellationRejected, DisconnectPersistenceFailure, DisconnectAuditFailure, DisconnectUnsafeConfiguration, DisconnectTransportOwnership:
		return true
	default:
		return false
	}
}

func validAuditCorrelation(correlation providersession.Correlation, session string) bool {
	values := []string{correlation.ProcessIncarnationID, correlation.ConnectionID, correlation.SessionID, correlation.InteractionID, correlation.ActivityID, correlation.RequestID, correlation.DecisionID}
	for _, value := range values {
		if value != "" && !validID(value) {
			return false
		}
	}
	return correlation.SessionID == "" || correlation.SessionID == session
}
func validAuditBucket(value string) bool {
	switch value {
	case "", "none", "low", "medium", "high", "very_high":
		return true
	}
	return false
}
func safeEvidence(value string) bool {
	request := providersession.RecoveryRequest{Session: providersession.SessionRef{Provider: "audit", SessionID: "audit"}, RecoveryEvidence: value}
	return len(value) > 0 && len(value) <= 128 && request.Validate() == nil
}

func auditLatency(start time.Time) string {
	if start.IsZero() {
		return "none"
	}
	switch elapsed := time.Since(start); {
	case elapsed < 10*time.Millisecond:
		return "low"
	case elapsed < 100*time.Millisecond:
		return "medium"
	case elapsed < time.Second:
		return "high"
	default:
		return "very_high"
	}
}

func auditProgress(sequence uint64) string {
	switch {
	case sequence < 10:
		return "low"
	case sequence < 100:
		return "medium"
	case sequence < 1000:
		return "high"
	default:
		return "very_high"
	}
}
