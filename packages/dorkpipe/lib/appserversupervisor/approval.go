package appserversupervisor

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"

	"dorkpipe.orchestrator/providersession"
)

// CAS-07 keeps App Server request shapes private. It projects only a small
// neutral subset and never retains command text, patches, prompts, messages,
// provider request IDs, or permission payloads.
var (
	ErrDecisionUnavailable = errors.New("app server approval decision is unavailable")
	ErrDecisionRejected    = errors.New("app server approval decision was rejected")
)

type pendingKind uint8

const (
	pendingCommand pendingKind = iota + 1
	pendingFileChange
	pendingPermission
	pendingUserInput
)

type pendingRequest struct {
	providerID       uint64
	kind             pendingKind
	correlation      providersession.Correlation
	decisionInFlight bool
	timer            *time.Timer
}

type serverRequestParams struct {
	ThreadID                     string            `json:"threadId"`
	TurnID                       string            `json:"turnId"`
	ItemID                       string            `json:"itemId"`
	GrantRoot                    string            `json:"grantRoot"`
	AdditionalPermissions        json.RawMessage   `json:"additionalPermissions"`
	NetworkApprovalContext       json.RawMessage   `json:"networkApprovalContext"`
	ProposedExecpolicyAmendment  json.RawMessage   `json:"proposedExecpolicyAmendment"`
	ProposedNetworkPolicyChanges json.RawMessage   `json:"proposedNetworkPolicyAmendments"`
	Permissions                  json.RawMessage   `json:"permissions"`
	Questions                    []json.RawMessage `json:"questions"`
}

func (s *Supervisor) handleServerRequest(providerID uint64, method string, raw json.RawMessage) DisconnectReason {
	if providerID == 0 || len(raw) == 0 || !json.Valid(raw) || containsModelReroute(raw) {
		if containsModelReroute(raw) {
			return DisconnectModelRerouted
		}
		return DisconnectMalformedEnvelope
	}
	var params serverRequestParams
	if json.Unmarshal(raw, &params) != nil {
		return DisconnectMalformedEnvelope
	}
	kind, actionClass, scope, reason := classifyServerRequest(method, params)
	if reason != "" {
		return reason
	}

	s.mu.Lock()
	if !s.started || !s.initialized || !s.lifecycle.active {
		s.mu.Unlock()
		return DisconnectEventOrdering
	}
	if !validID(params.ThreadID) || !validID(params.TurnID) || !validID(params.ItemID) {
		s.mu.Unlock()
		return DisconnectMalformedEnvelope
	}
	if params.ThreadID != s.lifecycle.threadID || params.TurnID != s.lifecycle.turnID || params.ItemID != s.lifecycle.itemID {
		s.mu.Unlock()
		return DisconnectCorrelationMismatch
	}
	if s.state != providersession.StateRunning || s.lifecycle.pending != nil {
		s.mu.Unlock()
		return DisconnectEventOrdering
	}
	if kind == pendingPermission && !s.permissionScopeDeclared(params.Permissions) {
		s.mu.Unlock()
		return DisconnectUnsupportedEvent
	}
	s.lifecycle.requestCounter++
	requestRef := "request-" + strconv.FormatUint(s.lifecycle.requestCounter, 10)
	decisionRef := "decision-" + strconv.FormatUint(s.lifecycle.requestCounter, 10)
	correlation := s.eventCorrelation(s.lifecycle.turnID, s.lifecycle.itemID)
	correlation.RequestID, correlation.DecisionID = requestRef, decisionRef
	pending := &pendingRequest{providerID: providerID, kind: kind, correlation: correlation}
	s.lifecycle.pending = pending
	pending.timer = time.AfterFunc(s.deadlines.Request, func() { s.expirePending(correlation) })
	s.sequence++
	event := providersession.Event{ContractVersion: providersession.ContractVersion, Sequence: s.sequence, OccurredAt: nowUTC(), Session: s.session, Correlation: correlation}
	if kind == pendingUserInput {
		s.state = providersession.StateWaitingForUserInput
		event.Kind, event.State, event.Summary = providersession.EventUserInputRequested, s.state, "user_input_requested"
		event.UserInput = &providersession.UserInputRequest{Correlation: correlation, PromptRef: requestRef}
	} else {
		s.state = providersession.StateWaitingForApproval
		event.Kind, event.State, event.Summary = providersession.EventApprovalRequested, s.state, "approval_requested"
		event.Approval = &providersession.ApprovalRequest{Correlation: correlation, ActionClass: actionClass, Summary: actionClass + "_approval", Scope: scope}
	}
	s.mu.Unlock()
	if !s.publish(event, "approval", "accepted", "waiting") {
		return DisconnectAuditFailure
	}
	return ""
}

func classifyServerRequest(method string, params serverRequestParams) (pendingKind, string, []string, DisconnectReason) {
	switch method {
	case "item/commandExecution/requestApproval":
		if unsafeApprovalExtension(params) {
			return 0, "", nil, DisconnectUnsupportedEvent
		}
		return pendingCommand, "command_execution", []string{"turn"}, ""
	case "item/fileChange/requestApproval":
		if strings.TrimSpace(params.GrantRoot) != "" {
			return 0, "", nil, DisconnectUnsupportedEvent
		}
		return pendingFileChange, "workspace_change", []string{"turn"}, ""
	case "item/permissions/requestApproval":
		if len(params.Permissions) == 0 || unsafeApprovalExtension(params) {
			return 0, "", nil, DisconnectUnsupportedEvent
		}
		return pendingPermission, "declared_permission", []string{"declared_writable_roots"}, ""
	case "item/tool/requestUserInput":
		if !validQuestions(params.Questions) {
			return 0, "", nil, DisconnectUnsupportedEvent
		}
		return pendingUserInput, "", nil, ""
	default:
		return 0, "", nil, DisconnectUnsupportedEvent
	}
}

func unsafeApprovalExtension(params serverRequestParams) bool {
	return presentJSON(params.AdditionalPermissions) || presentJSON(params.NetworkApprovalContext) || presentJSON(params.ProposedExecpolicyAmendment) || presentJSON(params.ProposedNetworkPolicyChanges)
}

func presentJSON(raw json.RawMessage) bool {
	return len(raw) != 0 && string(raw) != "null"
}

func validQuestions(questions []json.RawMessage) bool {
	if len(questions) < 1 || len(questions) > 3 {
		return false
	}
	seen := map[string]bool{}
	for _, raw := range questions {
		var question struct {
			ID string `json:"id"`
		}
		if json.Unmarshal(raw, &question) != nil || !validID(question.ID) || seen[question.ID] {
			return false
		}
		seen[question.ID] = true
	}
	return true
}

func (s *Supervisor) permissionScopeDeclared(raw json.RawMessage) bool {
	var permissions struct {
		FileSystem map[string]json.RawMessage `json:"fileSystem"`
	}
	if json.Unmarshal(raw, &permissions) != nil || len(permissions.FileSystem) != 1 {
		return false
	}
	write, found := permissions.FileSystem["write"]
	if !found {
		return false
	}
	var roots []string
	if json.Unmarshal(write, &roots) != nil || len(roots) == 0 {
		return false
	}
	for _, root := range roots {
		if !s.lifecycle.declaredRoots[root] {
			return false
		}
	}
	return true
}

// Decide maps the existing neutral one-turn approve/deny decision to the
// private App Server response. It cannot grant session access, amend policy,
// answer a user-input request, or approve a permission profile.
func (s *Supervisor) Decide(parent context.Context, decision providersession.ApprovalDecision) error {
	started := time.Now()
	if err := decision.Validate(); err != nil {
		return s.rejectDecision(DisconnectDecisionRejected)
	}
	s.mu.Lock()
	if !s.started || !s.initialized || s.state == providersession.StateDisconnected || s.lifecycle.pending == nil {
		s.mu.Unlock()
		return s.rejectDecision(DisconnectDecisionRejected)
	}
	pending := s.lifecycle.pending
	if decision.Correlation != pending.correlation {
		s.mu.Unlock()
		return s.rejectDecision(DisconnectCorrelationMismatch)
	}
	if pending.decisionInFlight || pending.kind == pendingUserInput || pending.kind == pendingPermission && decision.Decision != providersession.DecisionDeny {
		s.mu.Unlock()
		return s.rejectDecision(DisconnectDecisionRejected)
	}
	client := s.client
	pending.decisionInFlight = true
	providerID, result := pending.providerID, decisionResult(pending.kind, decision.Decision)
	s.mu.Unlock()
	ctx, cancel := context.WithTimeout(parent, s.deadlines.Request)
	defer cancel()
	if client == nil || client.respond(ctx, providerID, result) != nil {
		s.fail(DisconnectTransportClosed)
		return ErrDecisionUnavailable
	}
	if !s.auditOperation("approval", "delivered", "waiting", "approval_delivered", decision.Correlation, started) {
		return ErrDecisionUnavailable
	}
	return nil
}

func decisionResult(kind pendingKind, decision string) map[string]any {
	if kind == pendingPermission {
		return map[string]any{"permissions": map[string]any{}}
	}
	if decision == providersession.DecisionApprove {
		return map[string]any{"decision": "accept"}
	}
	return map[string]any{"decision": "decline"}
}

func (s *Supervisor) rejectDecision(reason DisconnectReason) error {
	s.fail(reason)
	return ErrDecisionRejected
}

func (s *Supervisor) expirePending(correlation providersession.Correlation) {
	s.mu.Lock()
	if s.state == providersession.StateDisconnected || s.lifecycle.pending == nil || s.lifecycle.pending.correlation != correlation {
		s.mu.Unlock()
		return
	}
	pending, client := s.lifecycle.pending, s.client
	s.lifecycle.pending = nil
	s.mu.Unlock()
	operation := "approval"
	if pending.kind == pendingUserInput {
		operation = "user_input"
	}
	if !s.auditOperation(operation, "expired", "waiting", "request_expired", correlation, time.Time{}) {
		return
	}
	if pending.kind != pendingUserInput && client != nil {
		ctx, cancel := context.WithTimeout(context.Background(), s.deadlines.Request)
		_ = client.respond(ctx, pending.providerID, decisionResult(pending.kind, providersession.DecisionDeny))
		cancel()
	}
	s.fail(DisconnectRequestDeadline)
}

func (s *Supervisor) handleServerRequestResolved(params eventParams) DisconnectReason {
	providerID, ok := providerRequestID(params.RequestID)
	if !ok || !validID(params.ThreadID) {
		return DisconnectMalformedEnvelope
	}
	s.mu.Lock()
	if !s.started || !s.initialized || !s.lifecycle.active || s.lifecycle.pending == nil || params.ThreadID != s.lifecycle.threadID {
		s.mu.Unlock()
		return DisconnectEventOrdering
	}
	pending := s.lifecycle.pending
	if pending.providerID != providerID {
		s.mu.Unlock()
		return DisconnectCorrelationMismatch
	}
	if !pending.decisionInFlight || s.state != providersession.StateWaitingForApproval {
		s.mu.Unlock()
		return DisconnectEventOrdering
	}
	if pending.timer != nil {
		pending.timer.Stop()
	}
	s.lifecycle.pending = nil
	s.state = providersession.StateRunning
	s.sequence++
	event := providersession.Event{ContractVersion: providersession.ContractVersion, Sequence: s.sequence, OccurredAt: nowUTC(), Session: s.session, Kind: providersession.EventStateChanged, State: s.state, Correlation: pending.correlation, Summary: "approval_resolved"}
	s.mu.Unlock()
	if !s.publish(event, "approval", "resolved", "running") {
		return DisconnectAuditFailure
	}
	return ""
}

func providerRequestID(raw json.RawMessage) (uint64, bool) {
	var id uint64
	if len(raw) == 0 || json.Unmarshal(raw, &id) != nil || id == 0 {
		return 0, false
	}
	return id, true
}
