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
	Status    json.RawMessage `json:"status"`
	Thread    struct {
		ID string `json:"id"`
	} `json:"thread"`
	Turn struct {
		ID     string          `json:"id"`
		Status json.RawMessage `json:"status"`
		Error  json.RawMessage `json:"error"`
	} `json:"turn"`
	Item struct {
		ID     string `json:"id"`
		Type   string `json:"type"`
		Status string `json:"status"`
	} `json:"item"`
	Warning    json.RawMessage `json:"warning"`
	Error      json.RawMessage `json:"error"`
	TokenUsage tokenUsage      `json:"tokenUsage"`
}

type tokenUsage struct {
	TotalTokens        *uint64             `json:"totalTokens"`
	Last               tokenUsageBreakdown `json:"last"`
	Total              tokenUsageBreakdown `json:"total"`
	ModelContextWindow *uint64             `json:"modelContextWindow"`
}

type tokenUsageBreakdown struct {
	CachedInputTokens     *uint64 `json:"cachedInputTokens"`
	InputTokens           *uint64 `json:"inputTokens"`
	OutputTokens          *uint64 `json:"outputTokens"`
	ReasoningOutputTokens *uint64 `json:"reasoningOutputTokens"`
	TotalTokens           *uint64 `json:"totalTokens"`
}

// handleNotification returns a safe disconnect reason rather than an error so
// the protocol client can apply its one-shot fail-closed shutdown path.
func (s *Supervisor) handleNotification(method string, raw json.RawMessage) DisconnectReason {
	s.mu.Lock()
	s.lastNotification = notificationClass(method)
	s.mu.Unlock()
	if method == "item/started" || method == "item/completed" {
		var probe struct {
			Item struct {
				Type string `json:"type"`
			} `json:"item"`
		}
		if json.Unmarshal(raw, &probe) == nil {
			s.mu.Lock()
			s.lastNotification = "item_" + itemClass(probe.Item.Type)
			s.mu.Unlock()
		}
	}
	if len(raw) == 0 || !json.Valid(raw) {
		return DisconnectMalformedEnvelope
	}
	if !notificationShapeAllowed(method, raw) {
		return DisconnectUnsupportedEvent
	}
	var params eventParams
	if json.Unmarshal(raw, &params) != nil {
		return DisconnectMalformedEnvelope
	}
	if method == "item/started" || method == "item/completed" {
		s.mu.Lock()
		s.lastNotification = "item_" + itemClass(params.Item.Type)
		s.mu.Unlock()
	}
	if containsModelReroute(raw) {
		return DisconnectModelRerouted
	}
	if s.discardEarlyProgress(method) {
		return ""
	}
	if method == "item/started" || method == "item/completed" {
		if reason, discarded := s.discardEarlyUserMessage(method, raw); discarded {
			return reason
		}
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
		if method != "thread/status/changed" || !validID(params.ThreadID) || params.ThreadID != s.lifecycle.threadID {
			return DisconnectEventOrdering
		}
		status := threadStatus(params.Status)
		if !validThreadStatus(status) {
			return DisconnectUnsupportedLifecycle
		}
		if status != s.lifecycle.threadStatus && !validThreadTransition(s.lifecycle.threadStatus, status) {
			return DisconnectEventOrdering
		}
		s.lifecycle.threadStatus = status
		return ""
	}
	var summary string
	var correlation providersession.Correlation
	persistIdle := false
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
		status := threadStatus(params.Status)
		if !validThreadStatus(status) {
			s.lastNotification = "thread_status_" + threadStatusShape(params.Status)
			return DisconnectUnsupportedLifecycle
		}
		if !s.lifecycle.threadNotified {
			if (status != "active" && status != "notLoaded") || s.lifecycle.threadStatus != "" {
				return DisconnectEventOrdering
			}
			s.lifecycle.threadNotified = true
		} else if status == s.lifecycle.threadStatus {
			return ""
		}
		if s.lifecycle.threadStatus != "" && !validThreadTransition(s.lifecycle.threadStatus, status) {
			return DisconnectEventOrdering
		}
		s.lifecycle.threadStatus = status
		summary, correlation = threadStatusSummary(status), s.eventCorrelation("", "")
		if status == "idle" {
			persistIdle = true
		}
	case "turn/started":
		if !s.validTurnNotification(params) {
			return eventMismatch(params.ThreadID != s.lifecycle.threadID || params.Turn.ID != s.lifecycle.turnID)
		}
		if turnEventStatus(params.Turn.Status) != "inProgress" {
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
		if !validItemStarted(params.Item.Type, params.Item.Status) {
			return DisconnectUnsupportedLifecycle
		}
		if s.lifecycle.itemID != "" {
			return DisconnectEventOrdering
		}
		s.lifecycle.turnNotified = true
		s.lifecycle.itemID = params.Item.ID
		summary, correlation = "item_started", s.eventCorrelation(s.lifecycle.turnID, params.Item.ID)
	case "item/updated":
		if !s.validActiveItem(params) {
			return eventMismatch(params.ThreadID != s.lifecycle.threadID || params.TurnID != s.lifecycle.turnID || params.Item.ID != s.lifecycle.itemID)
		}
		if !validItemType(params.Item.Type) {
			return DisconnectUnsupportedEvent
		}
		if !validItemStarted(params.Item.Type, params.Item.Status) {
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
		if !validItemCompleted(params.Item.Type, params.Item.Status) {
			return DisconnectUnsupportedLifecycle
		}
		itemID := s.lifecycle.itemID
		s.lifecycle.itemID = ""
		summary, correlation = "item_completed", s.eventCorrelation(s.lifecycle.turnID, itemID)
	case "item/agentMessage/delta", "item/reasoning/textDelta", "item/reasoning/summaryTextDelta", "item/reasoning/summaryPartAdded", "item/commandExecution/outputDelta", "item/fileChange/outputDelta", "item/mcpToolCall/progress", "item/plan/delta":
		if !s.validDelta(params) {
			return eventMismatch(params.ThreadID != s.lifecycle.threadID || params.TurnID != s.lifecycle.turnID || params.ItemID != s.lifecycle.itemID)
		}
		summary, correlation = "item_progress", s.eventCorrelation(s.lifecycle.turnID, s.lifecycle.itemID)
	case "thread/tokenUsage/updated":
		if !s.validActiveTurn(params) {
			return eventMismatch(params.ThreadID != s.lifecycle.threadID || params.TurnID != s.lifecycle.turnID)
		}
		total, valid := tokenUsageTotal(params.TokenUsage)
		if !valid || total == 0 || total > maxSafeEventCounter {
			return DisconnectUnsupportedEvent
		}
		if total <= s.lifecycle.tokenTotal {
			return DisconnectEventOrdering
		}
		s.lifecycle.tokenTotal = total
		summary, correlation = safeTokenSummary(total), s.eventCorrelation(s.lifecycle.turnID, "")
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
		if s.lifecycle.errorCount >= 16 {
			return DisconnectEventOrdering
		}
		classification := classifyEventError(params.Error)
		if classification == "unknown" {
			return DisconnectUnsupportedEvent
		}
		s.lifecycle.errorCount++
		summary, correlation = "error_"+classification, s.eventCorrelation(s.lifecycle.turnID, "")
	case "turn/completed":
		event, reason := s.terminalEventLocked(params)
		if reason != "" {
			return reason
		}
		s.mu.Unlock()
		if !s.publish(event, "event", "completed", auditLifecycle(s.State())) {
			s.mu.Lock()
			return DisconnectAuditFailure
		}
		s.mu.Lock()
		return ""
	default:
		return DisconnectUnsupportedEvent
	}
	s.sequence++
	event := providersession.Event{ContractVersion: providersession.ContractVersion, Sequence: s.sequence, OccurredAt: nowUTC(), Session: s.session, Kind: providersession.EventProgress, Correlation: correlation, Summary: summary}
	s.mu.Unlock()
	if !s.auditEvent(event, "event", "completed", auditLifecycle(s.State())) {
		s.mu.Lock()
		return DisconnectAuditFailure
	}
	if persistIdle {
		if reason := s.persistIdle(event.Sequence); reason != "" {
			s.mu.Lock()
			return reason
		}
	}
	s.events <- event
	s.mu.Lock()
	return ""
}

func itemClass(kind string) string {
	switch kind {
	case "userMessage":
		return "user"
	case "agentMessage":
		return "agent"
	case "reasoning":
		return "reasoning"
	case "plan":
		return "plan"
	case "commandExecution":
		return "command"
	case "fileChange":
		return "file"
	case "mcpToolCall":
		return "mcp"
	default:
		return "other"
	}
}

func (s *Supervisor) discardEarlyProgress(method string) bool {
	switch method {
	case "turn/started", "item/started", "item/updated", "item/completed", "item/agentMessage/delta", "item/reasoning/textDelta", "item/reasoning/summaryTextDelta", "item/reasoning/summaryPartAdded", "thread/tokenUsage/updated", "turn/plan/updated", "turn/diff/updated":
	default:
		return false
	}
	s.mu.RLock()
	pending := s.lifecycle.startPending && s.lifecycle.turnID == ""
	s.mu.RUnlock()
	return pending
}

func (s *Supervisor) discardEarlyUserMessage(method string, raw json.RawMessage) (DisconnectReason, bool) {
	var params struct {
		Item struct {
			Type string `json:"type"`
		} `json:"item"`
	}
	if json.Unmarshal(raw, &params) != nil {
		return DisconnectMalformedEnvelope, true
	}
	s.mu.RLock()
	pending := s.lifecycle.startPending && s.lifecycle.turnID == ""
	s.mu.RUnlock()
	if !pending || params.Item.Type != "userMessage" {
		return "", false
	}
	return "", true
}

func notificationClass(method string) string {
	switch method {
	case "thread/status/changed":
		return "thread_status"
	case "thread/started":
		return "thread_start"
	case "turn/started":
		return "turn_start"
	case "item/started":
		return "item_start"
	case "item/completed":
		return "item_complete"
	case "item/updated":
		return "item_update"
	case "item/agentMessage/delta":
		return "agent_delta"
	case "item/reasoning/textDelta", "item/reasoning/summaryTextDelta", "item/reasoning/summaryPartAdded":
		return "reasoning_delta"
	case "item/plan/delta":
		return "plan_delta"
	case "thread/tokenUsage/updated":
		return "token_usage"
	case "turn/plan/updated":
		return "turn_plan"
	case "turn/diff/updated":
		return "turn_diff"
	case "model/safetyBuffering/updated", "model/verification":
		return "model_status"
	case "turn/completed":
		return "turn_complete"
	case "error":
		return "error"
	case "warning":
		return "warning"
	case "deprecationNotice", "guardianWarning", "windows/worldWritableWarning":
		return "environment_warning"
	case "item/commandExecution/terminalInteraction":
		return "terminal_interaction"
	case "item/fileChange/patchUpdated":
		return "file_patch"
	case "process/exited", "process/outputDelta":
		return "process"
	case "skills/changed", "app/list/updated":
		return "configuration"
	case "thread/compacted", "thread/archived", "thread/closed", "thread/deleted", "thread/name/updated", "thread/unarchived":
		return "thread_lifecycle"
	case "thread/goal/cleared", "thread/goal/updated":
		return "thread_goal"
	case "turn/moderationMetadata":
		return "moderation"
	case "serverRequest/resolved":
		return "request_resolved"
	case "configWarning":
		return "config_warning"
	case "account/updated":
		return "account_update"
	case "account/rateLimits/updated":
		return "account_rate_limits"
	case "account/login/completed":
		return "account_login"
	case "mcpServer/oauthLogin/completed", "mcpServer/startupStatus/updated":
		return "mcp_status"
	case "hook/started", "hook/completed", "externalAgentConfig/import/completed", "externalAgentConfig/import/progress":
		return "integration"
	case "fs/changed", "fuzzyFileSearch/sessionCompleted", "fuzzyFileSearch/sessionUpdated":
		return "filesystem"
	case "windowsSandbox/setupCompleted":
		return "sandbox_status"
	default:
		return "other"
	}
}

func validItemShape(raw json.RawMessage) bool {
	var item struct {
		Type string `json:"type"`
	}
	if json.Unmarshal(raw, &item) != nil {
		return false
	}
	switch item.Type {
	case "userMessage":
		return nestedObjectFields(raw, "clientId", "content", "id", "type", "status")
	case "agentMessage":
		return nestedObjectFields(raw, "id", "memoryCitation", "phase", "text", "type", "status")
	case "reasoning":
		return nestedObjectFields(raw, "content", "id", "summary", "type", "status", "text")
	case "commandExecution":
		return nestedObjectFields(raw, "aggregatedOutput", "command", "commandActions", "cwd", "durationMs", "exitCode", "id", "processId", "source", "status", "type", "text")
	case "fileChange":
		return nestedObjectFields(raw, "changes", "id", "status", "type", "text")
	case "mcpToolCall":
		return nestedObjectFields(raw, "appContext", "arguments", "durationMs", "error", "id", "mcpAppResourceUri", "pluginId", "result", "server", "status", "tool", "type", "text")
	case "plan":
		return nestedObjectFields(raw, "id", "text", "type", "status")
	default:
		return false
	}
}

// notificationShapeAllowed is an intentionally narrow protocol boundary. It
// permits only the observed fields needed to classify an event (plus transient
// private content fields that are never projected or retained).
func notificationShapeAllowed(method string, raw json.RawMessage) bool {
	fields, ok := objectFields(raw, notificationFields(method)...)
	if !ok {
		return false
	}
	switch method {
	case "thread/started":
		return nestedObjectFields(fields["thread"], "id")
	case "item/started", "item/updated", "item/completed":
		return validItemShape(fields["item"])
	case "turn/started", "turn/completed":
		return nestedObjectFields(fields["turn"], "id", "status", "items", "itemsView", "startedAt", "completedAt", "durationMs", "error") && (len(fields["turn"]) == 0 || validTurnErrorShape(fields["turn"]))
	case "thread/tokenUsage/updated":
		return validTokenUsageShape(fields["tokenUsage"])
	case "warning":
		return warningShapeAllowed(fields["warning"])
	case "error":
		return validErrorShape(fields["error"])
	case "serverRequest/resolved":
		return true
	default:
		return true
	}
}

func notificationFields(method string) []string {
	switch method {
	case "thread/started":
		return []string{"thread"}
	case "thread/status/changed":
		return []string{"threadId", "status"}
	case "turn/started", "turn/completed":
		return []string{"threadId", "turn"}
	case "item/started", "item/updated", "item/completed":
		return []string{"threadId", "turnId", "item", "startedAtMs", "completedAtMs"}
	case "item/agentMessage/delta", "item/reasoning/textDelta", "item/reasoning/summaryTextDelta", "item/commandExecution/outputDelta", "item/fileChange/outputDelta", "item/mcpToolCall/progress", "item/plan/delta":
		return []string{"threadId", "turnId", "itemId", "delta"}
	case "item/reasoning/summaryPartAdded":
		return []string{"threadId", "turnId", "itemId", "summaryIndex"}
	case "thread/tokenUsage/updated":
		return []string{"threadId", "turnId", "tokenUsage"}
	case "warning":
		return []string{"threadId", "turnId", "warning", "message"}
	case "error":
		return []string{"threadId", "turnId", "error", willRepeatField}
	case "serverRequest/resolved":
		return []string{"threadId", "requestId"}
	default:
		return nil
	}
}

func validTurnErrorShape(raw json.RawMessage) bool {
	var turn struct {
		Error json.RawMessage `json:"error"`
	}
	return json.Unmarshal(raw, &turn) == nil && (len(turn.Error) == 0 || string(turn.Error) == "null" || validErrorShape(turn.Error))
}

func validTokenUsageShape(raw json.RawMessage) bool {
	fields, ok := objectFields(raw, "totalTokens", "text", "last", "total", "modelContextWindow")
	if !ok {
		return false
	}
	if len(fields["totalTokens"]) != 0 {
		return len(fields["last"]) == 0 && len(fields["total"]) == 0 && len(fields["modelContextWindow"]) == 0
	}
	if len(fields["last"]) == 0 || len(fields["total"]) == 0 {
		return false
	}
	if len(fields["modelContextWindow"]) != 0 && !validSafeCounter(fields["modelContextWindow"]) {
		return false
	}
	return validTokenUsageBreakdown(fields["last"]) && validTokenUsageBreakdown(fields["total"])
}

func validTokenUsageBreakdown(raw json.RawMessage) bool {
	fields, ok := objectFields(raw, "cachedInputTokens", "inputTokens", "outputTokens", "reasoningOutputTokens", "totalTokens")
	if !ok {
		return false
	}
	for _, name := range []string{"cachedInputTokens", "inputTokens", "outputTokens", "reasoningOutputTokens", "totalTokens"} {
		if len(fields[name]) == 0 || !validSafeCounter(fields[name]) {
			return false
		}
	}
	return true
}

func validSafeCounter(raw json.RawMessage) bool {
	var value uint64
	return json.Unmarshal(raw, &value) == nil && value <= maxSafeEventCounter
}

func tokenUsageTotal(usage tokenUsage) (uint64, bool) {
	if usage.TotalTokens != nil {
		return *usage.TotalTokens, true
	}
	if usage.Total.TotalTokens == nil {
		return 0, false
	}
	return *usage.Total.TotalTokens, true
}

func validErrorShape(raw json.RawMessage) bool {
	fields, ok := objectFields(raw, "codexErrorInfo", "message", "stack", "additionalDetails")
	if !ok || len(fields["message"]) == 0 {
		return false
	}
	var message string
	if json.Unmarshal(fields["message"], &message) != nil || len(message) > 4096 {
		return false
	}
	if len(fields["codexErrorInfo"]) == 0 || string(fields["codexErrorInfo"]) == "null" {
		return true
	}
	return len(fields["codexErrorInfo"]) <= 4096 && json.Valid(fields["codexErrorInfo"])
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
	return s.lifecycle.active && (params.ThreadID == "" || validID(params.ThreadID) && params.ThreadID == s.lifecycle.threadID) && validID(params.Turn.ID) && params.Turn.ID == s.lifecycle.turnID
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

func validThreadStatus(status string) bool {
	switch status {
	case "active", "idle", "notLoaded", "systemError":
		return true
	default:
		return false
	}
}

func validThreadTransition(from, to string) bool {
	switch from {
	case "notLoaded", "idle":
		return to == "active"
	case "active":
		return to == "idle" || to == "notLoaded" || to == "systemError"
	case "systemError":
		return to == "idle" || to == "notLoaded"
	default:
		return false
	}
}

func threadStatusSummary(status string) string {
	switch status {
	case "notLoaded":
		return "thread_not_loaded"
	case "systemError":
		return "thread_system_error"
	default:
		return "thread_" + status
	}
}

func threadStatus(raw json.RawMessage) string {
	var value string
	if json.Unmarshal(raw, &value) == nil {
		return value
	}
	fields, ok := objectFields(raw, "type", "activeFlags")
	if !ok || json.Unmarshal(fields["type"], &value) != nil {
		return ""
	}
	return value
}

func threadStatusShape(raw json.RawMessage) string {
	var text string
	if json.Unmarshal(raw, &text) == nil {
		return "string_other"
	}
	fields, ok := objectFields(raw, "type", "activeFlags")
	if !ok {
		return "object_extended"
	}
	if json.Unmarshal(fields["type"], &text) != nil || text == "" {
		return "object_invalid_type"
	}
	return "object_other"
}

func turnEventStatus(raw json.RawMessage) string {
	var value string
	if json.Unmarshal(raw, &value) == nil {
		return value
	}
	fields, ok := objectFields(raw, "type", "activeFlags")
	if !ok || json.Unmarshal(fields["type"], &value) != nil {
		return ""
	}
	return value
}

func validTurnTerminal(status string) bool {
	return status == "completed" || status == "interrupted" || status == "failed"
}

func validItemStarted(kind, status string) bool {
	if status == "inProgress" {
		return true
	}
	if kind == "userMessage" && status == "completed" {
		return true
	}
	return status == "" && (kind == "userMessage" || kind == "agentMessage" || kind == "reasoning" || kind == "plan")
}

func validItemCompleted(kind, status string) bool {
	if status == "completed" || status == "failed" {
		return true
	}
	return status == "" && (kind == "userMessage" || kind == "agentMessage" || kind == "reasoning" || kind == "plan")
}

func validItemType(kind string) bool {
	switch kind {
	case "userMessage", "agentMessage", "reasoning", "commandExecution", "fileChange", "mcpToolCall", "plan":
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
	if len(wrapped.CodexErrorInfo) == 0 || string(wrapped.CodexErrorInfo) == "null" {
		return "other"
	}
	classification := classifyErrorInfo(wrapped.CodexErrorInfo)
	if classification == "unknown" {
		return "other"
	}
	return classification
}

func classifyErrorInfo(raw json.RawMessage) string {
	var kind string
	if json.Unmarshal(raw, &kind) == nil {
		switch kind {
		case "contextWindowExceeded":
			return "context_window_exceeded"
		case "sessionBudgetExceeded":
			return "session_budget_exceeded"
		case "usageLimitExceeded":
			return "usage_limit_exceeded"
		case "serverOverloaded":
			return "server_overloaded"
		case "cyberPolicy":
			return "cyber_policy"
		case "internalServerError":
			return "internal_server_error"
		case "unauthorized":
			return "unauthorized"
		case "badRequest":
			return "bad_request"
		case "threadRollbackFailed":
			return "thread_rollback_failed"
		case "sandboxError":
			return "sandbox_error"
		case "other":
			return "other"
		default:
			return "unknown"
		}
	}
	var detail map[string]json.RawMessage
	if json.Unmarshal(raw, &detail) != nil {
		return "unknown"
	}
	for wire, safe := range map[string]string{"httpConnectionFailed": "http_connection_failed", "responseStreamConnectionFailed": "response_stream_connection_failed", "responseStreamDisconnected": "response_stream_disconnected", "responseTooManyFailedAttempts": "response_too_many_failed_attempts", "activeTurnNotSteerable": "active_turn_not_steerable"} {
		if _, found := detail[wire]; found {
			return safe
		}
	}
	return "unknown"
}

func nowUTC() time.Time { return time.Now().UTC() }
