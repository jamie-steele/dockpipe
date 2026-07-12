package appserversupervisor

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"path/filepath"
	"regexp"
	"strings"

	"dorkpipe.orchestrator/providersession"
)

// LifecyclePolicy is the closed native-turn policy used for every CAS-05
// lifecycle operation. It deliberately has no full-access, shell, automatic
// review, fallback, or network-enabled mode.
type LifecyclePolicy struct {
	Workspace        string
	WritableRoots    []string
	Sandbox          string
	NetworkEnabled   bool
	ApprovalPolicy   string
	Reviewer         string
	Model            string
	ReasoningEffort  string
	ModelProvider    string
	FullAccess       bool
	AllowShell       bool
	AutoReview       bool
	FallbackModel    string
	FallbackProvider string
}

// LifecycleReference is the narrow package-facing result of a lifecycle
// operation. Provider thread and turn identifiers are represented only by
// the neutral SessionRef and Correlation fields.
type LifecycleReference struct {
	Session     providersession.SessionRef
	Correlation providersession.Correlation
}

// InputReference is an opaque, bounded reference. CAS-05 never accepts or
// retains prompt text; a later adapter is responsible for resolving a ref.
type InputReference string

var (
	ErrLifecycleUnavailable = errors.New("app server lifecycle is unavailable")
	ErrLifecycleRejected    = errors.New("app server lifecycle request was rejected")
	identifierPattern       = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$`)
)

type lifecycleState struct {
	threadID  string
	turnID    string
	active    bool
	steerable bool
	policyKey [sha256.Size]byte
}

func (p LifecyclePolicy) validate() error {
	if !filepath.IsAbs(p.Workspace) || strings.TrimSpace(p.Workspace) == "" || p.Sandbox != "workspace-write" || p.NetworkEnabled || p.ApprovalPolicy != "untrusted" || p.Reviewer != "user" || p.Model != PinnedModel || p.ReasoningEffort != PinnedReasoningEffort || p.ModelProvider != "openai" || p.FullAccess || p.AllowShell || p.AutoReview || strings.TrimSpace(p.FallbackModel) != "" || strings.TrimSpace(p.FallbackProvider) != "" {
		return errors.New("CAS-05 native-turn policy is not permitted")
	}
	workspace := filepath.Clean(p.Workspace)
	seen, containsWorkspace := map[string]bool{}, false
	for _, root := range p.WritableRoots {
		if !filepath.IsAbs(root) || strings.TrimSpace(root) == "" {
			return errors.New("writable roots must be declared absolute paths")
		}
		root = filepath.Clean(root)
		if seen[root] {
			return errors.New("writable roots must be unique")
		}
		seen[root] = true
		containsWorkspace = containsWorkspace || root == workspace
	}
	if len(seen) == 0 || !containsWorkspace {
		return errors.New("workspace must be an explicitly declared writable root")
	}
	return nil
}

func (p LifecyclePolicy) params() map[string]any {
	roots := append([]string(nil), p.WritableRoots...)
	return map[string]any{
		"cwd": p.Workspace, "sandbox": p.Sandbox,
		"sandboxPolicy":  map[string]any{"type": "workspaceWrite", "writableRoots": roots, "networkAccess": false},
		"approvalPolicy": p.ApprovalPolicy, "approvalsReviewer": p.Reviewer,
		"model": p.Model, "modelProvider": p.ModelProvider, "effort": p.ReasoningEffort,
	}
}

func (p LifecyclePolicy) key() [sha256.Size]byte {
	values := append([]string{p.Workspace, p.Sandbox, p.ApprovalPolicy, p.Reviewer, p.Model, p.ReasoningEffort, p.ModelProvider}, p.WritableRoots...)
	return sha256.Sum256([]byte(strings.Join(values, "\x00")))
}

func (r LifecycleReference) Validate() error {
	if err := r.Session.Validate(); err != nil || !identifierPattern.MatchString(r.Correlation.ProcessIncarnationID) || !identifierPattern.MatchString(r.Correlation.ConnectionID) || r.Correlation.SessionID != r.Session.SessionID || !identifierPattern.MatchString(r.Correlation.SessionID) {
		return errors.New("complete lifecycle session correlation is required")
	}
	if r.Correlation.InteractionID != "" && !identifierPattern.MatchString(r.Correlation.InteractionID) {
		return errors.New("lifecycle interaction correlation is invalid")
	}
	return nil
}

func (r LifecycleReference) validThread(session providersession.SessionRef, processRef, connectionRef string) bool {
	return r.Session == session && r.Correlation.ProcessIncarnationID == processRef && r.Correlation.ConnectionID == connectionRef && r.Correlation.SessionID == session.SessionID && r.Correlation.InteractionID == "" && r.Correlation.ActivityID == "" && r.Correlation.RequestID == "" && r.Correlation.DecisionID == ""
}

func (r LifecycleReference) validTurn(session providersession.SessionRef, processRef, connectionRef, turnID string) bool {
	return r.Session == session && r.Correlation.ProcessIncarnationID == processRef && r.Correlation.ConnectionID == connectionRef && r.Correlation.SessionID == session.SessionID && r.Correlation.InteractionID == turnID && r.Correlation.ActivityID == "" && r.Correlation.RequestID == "" && r.Correlation.DecisionID == ""
}

func (s *Supervisor) lifecycleReference(turnID string) LifecycleReference {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return LifecycleReference{Session: s.session, Correlation: providersession.Correlation{ProcessIncarnationID: s.processRef, ConnectionID: s.connectionRef, SessionID: s.session.SessionID, InteractionID: turnID}}
}

func (s *Supervisor) lifecycleReady() (*protocolClient, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.client, s.started && s.initialized && s.state != providersession.StateDisconnected && s.client != nil
}

func (s *Supervisor) rejectLifecycle(reason DisconnectReason) error {
	s.fail(reason)
	return ErrLifecycleRejected
}

func validateInputReference(input InputReference) error {
	if !identifierPattern.MatchString(string(input)) {
		return errors.New("input reference must be a bounded opaque identifier")
	}
	return nil
}

func inputParam(input InputReference) []any {
	// The reference is intentionally the only value transmitted here; no prompt
	// or command content is retained by the supervisor.
	return []any{map[string]any{"type": "text", "text": string(input)}}
}

// StartThread performs the first provider thread lifecycle operation after the
// CAS-04 initialization gate and returns only neutral references.
func (s *Supervisor) StartThread(ctx context.Context, policy LifecyclePolicy) (LifecycleReference, error) {
	s.lifecycleMu.Lock()
	defer s.lifecycleMu.Unlock()
	if err := policy.validate(); err != nil {
		return LifecycleReference{}, s.rejectLifecycle(DisconnectPolicyMismatch)
	}
	client, ready := s.lifecycleReady()
	if !ready || s.lifecycle.threadID != "" {
		return LifecycleReference{}, s.rejectLifecycle(DisconnectLifecycleRejected)
	}
	result, err := s.lifecycleRequest(ctx, client, "thread/start", policy.params())
	if err != nil {
		s.fail(client.failureReason())
		return LifecycleReference{}, ErrLifecycleUnavailable
	}
	threadID, reason := projectThread(result)
	if reason != "" {
		return LifecycleReference{}, s.rejectLifecycle(reason)
	}
	s.mu.Lock()
	s.session = providersession.SessionRef{Provider: s.session.Provider, SessionID: threadID}
	s.lifecycle.threadID = threadID
	s.lifecycle.policyKey = policy.key()
	s.mu.Unlock()
	return s.lifecycleReference(""), nil
}

// ReadThread verifies the active private thread still matches its neutral
// reference. It is not recovery and cannot proceed after a disconnect.
func (s *Supervisor) ReadThread(ctx context.Context, reference LifecycleReference, policy LifecyclePolicy) (LifecycleReference, error) {
	return s.threadOperation(ctx, "thread/read", reference, policy)
}

// ResumeThread is the caller-requested provider lifecycle operation on the
// healthy private transport only; it is not a recovery mechanism.
func (s *Supervisor) ResumeThread(ctx context.Context, reference LifecycleReference, policy LifecyclePolicy) (LifecycleReference, error) {
	return s.threadOperation(ctx, "thread/resume", reference, policy)
}

func (s *Supervisor) threadOperation(ctx context.Context, method string, reference LifecycleReference, policy LifecyclePolicy) (LifecycleReference, error) {
	s.lifecycleMu.Lock()
	defer s.lifecycleMu.Unlock()
	if err := policy.validate(); err != nil {
		return LifecycleReference{}, s.rejectLifecycle(DisconnectPolicyMismatch)
	}
	client, ready := s.lifecycleReady()
	s.mu.RLock()
	current := s.session
	threadID := s.lifecycle.threadID
	s.mu.RUnlock()
	if !ready || threadID == "" || !reference.validThread(current, s.processRef, s.connectionRef) {
		return LifecycleReference{}, s.rejectLifecycle(DisconnectLifecycleRejected)
	}
	if s.lifecycle.policyKey != policy.key() {
		return LifecycleReference{}, s.rejectLifecycle(DisconnectPolicyMismatch)
	}
	params := policy.params()
	params["threadId"] = threadID
	params["includeTurns"] = false
	result, err := s.lifecycleRequest(ctx, client, method, params)
	if err != nil {
		s.fail(client.failureReason())
		return LifecycleReference{}, ErrLifecycleUnavailable
	}
	returnedThread, reason := projectThread(result)
	if reason != "" || returnedThread != threadID {
		if reason == "" {
			reason = DisconnectCorrelationMismatch
		}
		return LifecycleReference{}, s.rejectLifecycle(reason)
	}
	return s.lifecycleReference(""), nil
}

// StartTurn allows one active turn for the supervisor-owned thread. The input
// is deliberately an opaque reference rather than a retained prompt.
func (s *Supervisor) StartTurn(ctx context.Context, reference LifecycleReference, policy LifecyclePolicy, input InputReference) (LifecycleReference, error) {
	s.lifecycleMu.Lock()
	defer s.lifecycleMu.Unlock()
	if err := policy.validate(); err != nil {
		return LifecycleReference{}, s.rejectLifecycle(DisconnectPolicyMismatch)
	}
	if err := validateInputReference(input); err != nil {
		return LifecycleReference{}, s.rejectLifecycle(DisconnectLifecycleRejected)
	}
	client, ready := s.lifecycleReady()
	s.mu.RLock()
	current, threadID, active := s.session, s.lifecycle.threadID, s.lifecycle.active
	s.mu.RUnlock()
	if !ready || threadID == "" || active || !reference.validThread(current, s.processRef, s.connectionRef) {
		return LifecycleReference{}, s.rejectLifecycle(DisconnectLifecycleRejected)
	}
	if s.lifecycle.policyKey != policy.key() {
		return LifecycleReference{}, s.rejectLifecycle(DisconnectPolicyMismatch)
	}
	params := policy.params()
	params["threadId"] = threadID
	params["input"] = inputParam(input)
	result, err := s.lifecycleRequest(ctx, client, "turn/start", params)
	if err != nil {
		s.fail(client.failureReason())
		return LifecycleReference{}, ErrLifecycleUnavailable
	}
	returnedThread, turnID, steerable, reason := projectTurn(result)
	if reason != "" || returnedThread != threadID {
		if reason == "" {
			reason = DisconnectCorrelationMismatch
		}
		return LifecycleReference{}, s.rejectLifecycle(reason)
	}
	s.mu.Lock()
	s.lifecycle.turnID, s.lifecycle.active, s.lifecycle.steerable = turnID, true, steerable
	s.mu.Unlock()
	s.emit(providersession.StateRunning, "turn_started")
	return s.lifecycleReference(turnID), nil
}

// SteerTurn is allowed only for the current active steerable turn.
func (s *Supervisor) SteerTurn(ctx context.Context, reference LifecycleReference, policy LifecyclePolicy, input InputReference) error {
	s.lifecycleMu.Lock()
	defer s.lifecycleMu.Unlock()
	if err := policy.validate(); err != nil {
		return s.rejectLifecycle(DisconnectPolicyMismatch)
	}
	if err := validateInputReference(input); err != nil {
		return s.rejectLifecycle(DisconnectLifecycleRejected)
	}
	client, ready := s.lifecycleReady()
	s.mu.RLock()
	current, turnID, active, steerable := s.session, s.lifecycle.turnID, s.lifecycle.active, s.lifecycle.steerable
	s.mu.RUnlock()
	if !ready || !active || !steerable || !reference.validTurn(current, s.processRef, s.connectionRef, turnID) {
		return s.rejectLifecycle(DisconnectLifecycleRejected)
	}
	if s.lifecycle.policyKey != policy.key() {
		return s.rejectLifecycle(DisconnectPolicyMismatch)
	}
	params := policy.params()
	params["threadId"], params["turnId"], params["input"] = current.SessionID, turnID, inputParam(input)
	result, err := s.lifecycleRequest(ctx, client, "turn/steer", params)
	if err != nil {
		s.fail(client.failureReason())
		return ErrLifecycleUnavailable
	}
	returnedThread, returnedTurn, responseSteerable, reason := projectTurn(result)
	if reason != "" || returnedThread != current.SessionID || returnedTurn != turnID || !responseSteerable {
		if reason == "" {
			reason = DisconnectCorrelationMismatch
		}
		return s.rejectLifecycle(reason)
	}
	return nil
}

func (s *Supervisor) lifecycleRequest(parent context.Context, client *protocolClient, method string, params map[string]any) (json.RawMessage, error) {
	ctx, cancel := context.WithTimeout(parent, s.deadlines.Request)
	defer cancel()
	return client.request(ctx, method, params)
}

type threadResponse struct {
	Thread struct {
		ID string `json:"id"`
	} `json:"thread"`
}
type turnResponse struct {
	Thread struct {
		ID string `json:"id"`
	} `json:"thread"`
	Turn struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	} `json:"turn"`
}

func projectThread(result json.RawMessage) (string, DisconnectReason) {
	if containsModelReroute(result) {
		return "", DisconnectModelRerouted
	}
	var response threadResponse
	if json.Unmarshal(result, &response) != nil || !identifierPattern.MatchString(response.Thread.ID) {
		return "", DisconnectUnsupportedLifecycle
	}
	return response.Thread.ID, ""
}

func projectTurn(result json.RawMessage) (string, string, bool, DisconnectReason) {
	if containsModelReroute(result) {
		return "", "", false, DisconnectModelRerouted
	}
	var response turnResponse
	if json.Unmarshal(result, &response) != nil || !identifierPattern.MatchString(response.Thread.ID) || !identifierPattern.MatchString(response.Turn.ID) {
		return "", "", false, DisconnectUnsupportedLifecycle
	}
	if response.Turn.Status != "inProgress" {
		return "", "", false, DisconnectUnsupportedLifecycle
	}
	return response.Thread.ID, response.Turn.ID, true, ""
}

func containsModelReroute(raw json.RawMessage) bool {
	var value any
	if json.Unmarshal(raw, &value) != nil {
		return false
	}
	var visit func(any) bool
	visit = func(current any) bool {
		switch typed := current.(type) {
		case map[string]any:
			for key, child := range typed {
				if strings.Contains(strings.ToLower(key), "rerout") || visit(child) {
					return true
				}
			}
		case []any:
			for _, child := range typed {
				if visit(child) {
					return true
				}
			}
		case string:
			return strings.Contains(strings.ToLower(typed), "model_rerout") || strings.Contains(strings.ToLower(typed), "modelrerout")
		}
		return false
	}
	return visit(value)
}
