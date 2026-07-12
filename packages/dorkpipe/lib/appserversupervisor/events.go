package appserversupervisor

import (
	"encoding/json"
	"strconv"
	"time"

	"dorkpipe.orchestrator/providersession"
)

// CAS-06 accepts only the notification subset below. Provider frames remain
// transient: this parser extracts opaque identifiers and allow-listed status
// classes, never text, messages, command data, file data, or error bodies.
const maxSafeEventCounter uint64 = 100_000_000

type eventParams struct {
	ThreadID  string          `json:"threadId"`
	TurnID    string          `json:"turnId"`
	ItemID    string          `json:"itemId"`
	RequestID json.RawMessage `json:"requestId"`
	Status    string          `json:"status"`
	Thread    struct {
		ID string `json:"id"`
	} `json:"thread"`
	Turn struct {
		ID     string          `json:"id"`
		Status string          `json:"status"`
		Error  json.RawMessage `json:"error"`
	} `json:"turn"`
	Item struct {
		ID     string `json:"id"`
		Type   string `json:"type"`
		Status string `json:"status"`
	} `json:"item"`
	Warning    json.RawMessage `json:"warning"`
	Error      json.RawMessage `json:"error"`
	TokenUsage struct {
		TotalTokens *uint64 `json:"totalTokens"`
	} `json:"tokenUsage"`
}

// handleNotification returns a safe disconnect reason rather than an error so
// the protocol client can apply its one-shot fail-closed shutdown path.
func (s *Supervisor) handleNotification(method string, raw json.RawMessage) DisconnectReason {
	if len(raw) == 0 || !json.Valid(raw) {
		return DisconnectMalformedEnvelope
	}
	var params eventParams
	if json.Unmarshal(raw, &params) != nil {
		return DisconnectMalformedEnvelope
	}
	if containsModelReroute(raw) {
		return DisconnectModelRerouted
	}
	if method == "serverRequest/resolved" {
		return s.handleServerRequestResolved(params)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.started || !s.initialized || s.state == providersession.StateDisconnected || s.lifecycle.threadID == "" {
		return DisconnectLifecycleRejected
	}
	if s.lifecycle.pending != nil {
		return DisconnectEventOrdering
	}
	var summary string
	var correlation providersession.Correlation
	switch method {
	case "thread/started":
		if !validID(params.Thread.ID) {
			return DisconnectMalformedEnvelope
		}
		if params.Thread.ID != s.lifecycle.threadID {
			return DisconnectCorrelationMismatch
		}
		if s.lifecycle.threadNotified || s.lifecycle.turnID != "" {
			return DisconnectEventOrdering
		}
		s.lifecycle.threadNotified = true
		summary, correlation = "thread_started", s.eventCorrelation("", "")
	case "thread/status/changed":
		if !validID(params.ThreadID) {
			return DisconnectMalformedEnvelope
		}
		if params.ThreadID != s.lifecycle.threadID {
			return DisconnectCorrelationMismatch
		}
		if !validThreadStatus(params.Status) {
			return DisconnectUnsupportedLifecycle
		}
		if !s.lifecycle.threadNotified || params.Status == s.lifecycle.threadStatus {
			return DisconnectEventOrdering
		}
		if s.lifecycle.threadStatus != "" && !(s.lifecycle.threadStatus == "active" && params.Status == "idle") && !(s.lifecycle.threadStatus == "idle" && params.Status == "active") {
			return DisconnectEventOrdering
		}
		s.lifecycle.threadStatus = params.Status
		summary, correlation = "thread_"+params.Status, s.eventCorrelation("", "")
	case "turn/started":
		if !s.validTurnNotification(params) {
			return eventMismatch(params.ThreadID != s.lifecycle.threadID || params.Turn.ID != s.lifecycle.turnID)
		}
		if params.Turn.Status != "inProgress" {
			return DisconnectUnsupportedLifecycle
		}
		if s.lifecycle.turnNotified || s.lifecycle.itemID != "" {
			return DisconnectEventOrdering
		}
		s.lifecycle.turnNotified = true
		summary, correlation = "turn_started", s.eventCorrelation(s.lifecycle.turnID, "")
	case "item/started":
		if !s.validActiveTurn(params) {
			return eventMismatch(params.ThreadID != s.lifecycle.threadID || params.TurnID != s.lifecycle.turnID)
		}
		if !validID(params.Item.ID) {
			return DisconnectMalformedEnvelope
		}
		if !validItemType(params.Item.Type) {
			return DisconnectUnsupportedEvent
		}
		if params.Item.Status != "inProgress" {
			return DisconnectUnsupportedLifecycle
		}
		if !s.lifecycle.turnNotified || s.lifecycle.itemID != "" {
			return DisconnectEventOrdering
		}
		s.lifecycle.itemID = params.Item.ID
		summary, correlation = "item_started", s.eventCorrelation(s.lifecycle.turnID, params.Item.ID)
	case "item/updated":
		if !s.validActiveItem(params) {
			return eventMismatch(params.ThreadID != s.lifecycle.threadID || params.TurnID != s.lifecycle.turnID || params.Item.ID != s.lifecycle.itemID)
		}
		if !validItemType(params.Item.Type) {
			return DisconnectUnsupportedEvent
		}
		if params.Item.Status != "inProgress" {
			return DisconnectUnsupportedLifecycle
		}
		summary, correlation = "item_progress", s.eventCorrelation(s.lifecycle.turnID, s.lifecycle.itemID)
	case "item/completed":
		if !s.validActiveItem(params) {
			return eventMismatch(params.ThreadID != s.lifecycle.threadID || params.TurnID != s.lifecycle.turnID || params.Item.ID != s.lifecycle.itemID)
		}
		if !validItemType(params.Item.Type) {
			return DisconnectUnsupportedEvent
		}
		if !validItemTerminal(params.Item.Status) {
			return DisconnectUnsupportedLifecycle
		}
		itemID := s.lifecycle.itemID
		s.lifecycle.itemID = ""
		summary, correlation = "item_completed", s.eventCorrelation(s.lifecycle.turnID, itemID)
	case "item/agentMessage/delta", "item/reasoning/textDelta", "item/reasoning/summaryTextDelta", "item/commandExecution/outputDelta", "item/fileChange/outputDelta", "item/mcpToolCall/progress", "item/plan/delta":
		if !s.validDelta(params) {
			return eventMismatch(params.ThreadID != s.lifecycle.threadID || params.TurnID != s.lifecycle.turnID || params.ItemID != s.lifecycle.itemID)
		}
		summary, correlation = "item_progress", s.eventCorrelation(s.lifecycle.turnID, s.lifecycle.itemID)
	case "thread/tokenUsage/updated":
		if !s.validActiveTurn(params) {
			return eventMismatch(params.ThreadID != s.lifecycle.threadID || params.TurnID != s.lifecycle.turnID)
		}
		if params.TokenUsage.TotalTokens == nil || *params.TokenUsage.TotalTokens == 0 || *params.TokenUsage.TotalTokens > maxSafeEventCounter {
			return DisconnectUnsupportedEvent
		}
		if *params.TokenUsage.TotalTokens <= s.lifecycle.tokenTotal {
			return DisconnectEventOrdering
		}
		s.lifecycle.tokenTotal = *params.TokenUsage.TotalTokens
		summary, correlation = safeTokenSummary(*params.TokenUsage.TotalTokens), s.eventCorrelation(s.lifecycle.turnID, "")
	case "warning":
		if !s.validActiveTurn(params) {
			return eventMismatch(params.ThreadID != s.lifecycle.threadID || params.TurnID != s.lifecycle.turnID)
		}
		if classifyWarning(params.Warning) == "unclassified" {
			return DisconnectUnsupportedEvent
		}
		if s.lifecycle.warningNotified {
			return DisconnectEventOrdering
		}
		s.lifecycle.warningNotified = true
		summary, correlation = "warning_"+classifyWarning(params.Warning), s.eventCorrelation(s.lifecycle.turnID, "")
	case "error":
		if !s.validActiveTurn(params) {
			return eventMismatch(params.ThreadID != s.lifecycle.threadID || params.TurnID != s.lifecycle.turnID)
		}
		if s.lifecycle.cancellation != nil {
			return DisconnectProviderError
		}
		if s.lifecycle.errorNotified {
			return DisconnectEventOrdering
		}
		classification := classifyEventError(params.Error)
		if classification == "unknown" {
			return DisconnectUnsupportedEvent
		}
		s.lifecycle.errorNotified = true
		summary, correlation = "error_"+classification, s.eventCorrelation(s.lifecycle.turnID, "")
	case "turn/completed":
		event, reason := s.terminalEventLocked(params)
		if reason != "" {
			return reason
		}
		s.mu.Unlock()
		s.events <- event
		s.mu.Lock()
		return ""
	default:
		return DisconnectUnsupportedEvent
	}
	s.sequence++
	event := providersession.Event{ContractVersion: providersession.ContractVersion, Sequence: s.sequence, OccurredAt: nowUTC(), Session: s.session, Kind: providersession.EventProgress, Correlation: correlation, Summary: summary}
	s.mu.Unlock()
	s.events <- event
	s.mu.Lock()
	return ""
}

func (s *Supervisor) eventCorrelation(turnID, itemID string) providersession.Correlation {
	return providersession.Correlation{ProcessIncarnationID: s.processRef, ConnectionID: s.connectionRef, SessionID: s.session.SessionID, InteractionID: turnID, ActivityID: itemID}
}

func (s *Supervisor) lifecycleCorrelation(turnID, itemID string) providersession.Correlation {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.eventCorrelation(turnID, itemID)
}

func (s *Supervisor) validActiveTurn(params eventParams) bool {
	return s.lifecycle.active && validID(params.ThreadID) && validID(params.TurnID) && params.ThreadID == s.lifecycle.threadID && params.TurnID == s.lifecycle.turnID
}

func (s *Supervisor) validTurnNotification(params eventParams) bool {
	return s.lifecycle.active && validID(params.ThreadID) && validID(params.Turn.ID) && params.ThreadID == s.lifecycle.threadID && params.Turn.ID == s.lifecycle.turnID
}

func (s *Supervisor) validActiveItem(params eventParams) bool {
	return s.validActiveTurn(params) && s.lifecycle.itemID != "" && params.Item.ID == s.lifecycle.itemID
}

func (s *Supervisor) validDelta(params eventParams) bool {
	return s.validActiveTurn(params) && s.lifecycle.itemID != "" && validID(params.ItemID) && params.ItemID == s.lifecycle.itemID
}

func eventMismatch(correlation bool) DisconnectReason {
	if correlation {
		return DisconnectCorrelationMismatch
	}
	return DisconnectEventOrdering
}

func validID(value string) bool { return identifierPattern.MatchString(value) }

func validThreadStatus(status string) bool { return status == "active" || status == "idle" }

func validTurnTerminal(status string) bool {
	return status == "completed" || status == "interrupted" || status == "failed"
}

func validItemTerminal(status string) bool { return status == "completed" || status == "failed" }

func validItemType(kind string) bool {
	switch kind {
	case "agentMessage", "reasoning", "commandExecution", "fileChange", "mcpToolCall", "plan", "webSearch", "todoList":
		return true
	default:
		return false
	}
}

func safeTokenSummary(total uint64) string {
	return "token_usage_total_" + strconv.FormatUint(total, 10)
}

func classifyEventError(raw json.RawMessage) string {
	var wrapped struct {
		CodexErrorInfo json.RawMessage `json:"codexErrorInfo"`
	}
	if len(raw) == 0 || json.Unmarshal(raw, &wrapped) != nil {
		return "unknown"
	}
	return classifyErrorInfo(wrapped.CodexErrorInfo)
}

func classifyErrorInfo(raw json.RawMessage) string {
	var kind string
	if json.Unmarshal(raw, &kind) == nil {
		switch kind {
		case "contextWindowExceeded", "sessionBudgetExceeded", "usageLimitExceeded", "serverOverloaded", "cyberPolicy", "internalServerError", "unauthorized", "badRequest", "threadRollbackFailed", "sandboxError", "other":
			return kind
		default:
			return "unknown"
		}
	}
	var detail map[string]json.RawMessage
	if json.Unmarshal(raw, &detail) != nil {
		return "unknown"
	}
	for _, kind := range []string{"httpConnectionFailed", "responseStreamConnectionFailed", "responseStreamDisconnected", "responseTooManyFailedAttempts", "activeTurnNotSteerable"} {
		if _, found := detail[kind]; found {
			return kind
		}
	}
	return "unknown"
}

func nowUTC() time.Time { return time.Now().UTC() }
