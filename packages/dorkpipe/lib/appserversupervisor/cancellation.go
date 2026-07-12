package appserversupervisor

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"dorkpipe.orchestrator/providersession"
)

// CAS-08 keeps interruption protocol shapes private. A caller can only submit
// the existing provider-neutral intent; this package maps it to the exact
// active private turn and waits for the terminal confirmation.
var (
	ErrCancellationUnavailable = errors.New("app server cancellation is unavailable")
	ErrCancellationRejected    = errors.New("app server cancellation was rejected")
)

type pendingCancellation struct {
	intent                providersession.CancellationIntent
	interruptAcknowledged bool
	timer                 *time.Timer
}

// Cancel requests interruption of exactly the active turn. Its successful
// return acknowledges delivery only: the session remains running until the
// correlated interrupted terminal notification is received.
func (s *Supervisor) Cancel(parent context.Context, intent providersession.CancellationIntent) error {
	started := time.Now()
	s.lifecycleMu.Lock()
	defer s.lifecycleMu.Unlock()
	if err := intent.Validate(); err != nil {
		return s.rejectCancellation(DisconnectCancellationRejected)
	}
	client, ready := s.lifecycleReady()
	s.mu.Lock()
	if !ready || s.state != providersession.StateRunning || !s.lifecycle.active || s.lifecycle.pending != nil || s.lifecycle.cancellation != nil {
		s.mu.Unlock()
		return s.rejectCancellation(DisconnectEventOrdering)
	}
	expected := s.eventCorrelation(s.lifecycle.turnID, "")
	if intent.Session != s.session || intent.Correlation != expected {
		s.mu.Unlock()
		return s.rejectCancellation(DisconnectCorrelationMismatch)
	}
	s.lifecycle.cancellation = &pendingCancellation{intent: intent}
	s.sequence++
	event := providersession.Event{ContractVersion: providersession.ContractVersion, Sequence: s.sequence, OccurredAt: nowUTC(), Session: s.session, Kind: providersession.EventCancellationRequested, Correlation: intent.Correlation, Summary: "cancellation_requested", Cancellation: &intent}
	s.mu.Unlock()
	if !s.publish(event, "cancellation", "accepted", "running") {
		return ErrCancellationUnavailable
	}

	result, err := s.lifecycleRequest(parent, client, "turn/interrupt", map[string]any{"threadId": intent.Session.SessionID, "turnId": intent.Correlation.InteractionID})
	if err != nil {
		s.fail(client.failureReason())
		return ErrCancellationUnavailable
	}
	threadID, turnID, risk, reason := projectInterrupt(result)
	if reason != "" || threadID != intent.Session.SessionID || turnID != intent.Correlation.InteractionID {
		if reason == "" {
			reason = DisconnectCorrelationMismatch
		}
		return s.rejectCancellation(reason)
	}

	var riskEvent *providersession.Event
	s.mu.Lock()
	pending := s.lifecycle.cancellation
	if s.state == providersession.StateDisconnected || pending == nil || pending.intent != intent || !s.lifecycle.active || s.lifecycle.threadID != threadID || s.lifecycle.turnID != turnID {
		s.mu.Unlock()
		return s.rejectCancellation(DisconnectEventOrdering)
	}
	pending.interruptAcknowledged = true
	pending.timer = time.AfterFunc(s.deadlines.Request, func() { s.expireCancellation(intent.Correlation) })
	if risk {
		s.sequence++
		event := providersession.Event{ContractVersion: providersession.ContractVersion, Sequence: s.sequence, OccurredAt: nowUTC(), Session: s.session, Kind: providersession.EventProgress, Correlation: intent.Correlation, Summary: "background_process_risk_possible"}
		riskEvent = &event
	}
	s.mu.Unlock()
	if riskEvent != nil && !s.publish(*riskEvent, "cancellation", "delivered", "running") {
		return ErrCancellationUnavailable
	}
	if !s.auditOperation("cancellation", "delivered", "running", "interrupt_delivered", intent.Correlation, started) {
		return ErrCancellationUnavailable
	}
	return nil
}

func (s *Supervisor) rejectCancellation(reason DisconnectReason) error {
	s.fail(reason)
	return ErrCancellationRejected
}

func (s *Supervisor) expireCancellation(correlation providersession.Correlation) {
	s.mu.Lock()
	pending := s.lifecycle.cancellation
	if s.state == providersession.StateDisconnected || pending == nil || pending.intent.Correlation != correlation || !pending.interruptAcknowledged {
		s.mu.Unlock()
		return
	}
	s.mu.Unlock()
	s.fail(DisconnectRequestDeadline)
}

type interruptResponse struct {
	Thread struct {
		ID string `json:"id"`
	} `json:"thread"`
	Turn struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	} `json:"turn"`
}

func projectInterrupt(result json.RawMessage) (string, string, bool, DisconnectReason) {
	if containsModelReroute(result) {
		return "", "", false, DisconnectModelRerouted
	}
	var response interruptResponse
	if json.Unmarshal(result, &response) != nil || !identifierPattern.MatchString(response.Thread.ID) || !identifierPattern.MatchString(response.Turn.ID) || response.Turn.Status != "inProgress" {
		return "", "", false, DisconnectUnsupportedLifecycle
	}
	return response.Thread.ID, response.Turn.ID, hasBackgroundProcessRisk(result), ""
}

func hasBackgroundProcessRisk(raw json.RawMessage) bool {
	var value any
	if json.Unmarshal(raw, &value) != nil {
		return false
	}
	var visit func(any) bool
	visit = func(current any) bool {
		switch typed := current.(type) {
		case map[string]any:
			for key, child := range typed {
				lower := strings.ToLower(key)
				if strings.Contains(lower, "background") && strings.Contains(lower, "process") && child != nil {
					return true
				}
				if visit(child) {
					return true
				}
			}
		case []any:
			for _, child := range typed {
				if visit(child) {
					return true
				}
			}
		}
		return false
	}
	return visit(value)
}

// terminalEventLocked accepts the exact final confirmation for a pending
// interruption. It is called with s.mu held and never retains provider data.
func (s *Supervisor) terminalEventLocked(params eventParams) (providersession.Event, DisconnectReason) {
	if !s.validTurnNotification(params) {
		return providersession.Event{}, eventMismatch(params.ThreadID != s.lifecycle.threadID || params.Turn.ID != s.lifecycle.turnID)
	}
	if !validTurnTerminal(params.Turn.Status) {
		return providersession.Event{}, DisconnectUnsupportedLifecycle
	}
	if !s.lifecycle.turnNotified || s.lifecycle.itemID != "" {
		return providersession.Event{}, DisconnectEventOrdering
	}
	turnID := s.lifecycle.turnID
	status := params.Turn.Status
	if status == "failed" && len(params.Turn.Error) != 0 && classifyEventError(params.Turn.Error) == "unknown" {
		return providersession.Event{}, DisconnectUnsupportedEvent
	}
	if pending := s.lifecycle.cancellation; pending != nil {
		if !pending.interruptAcknowledged {
			return providersession.Event{}, DisconnectEventOrdering
		}
		if status != "interrupted" {
			return providersession.Event{}, DisconnectCancellationRejected
		}
		if pending.timer != nil {
			pending.timer.Stop()
		}
		s.lifecycle.turnID, s.lifecycle.active, s.lifecycle.steerable, s.lifecycle.turnNotified, s.lifecycle.cancellation = "", false, false, false, nil
		s.state = providersession.StateCancelled
		s.sequence++
		return providersession.Event{ContractVersion: providersession.ContractVersion, Sequence: s.sequence, OccurredAt: nowUTC(), Session: s.session, Kind: providersession.EventStateChanged, State: providersession.StateCancelled, Correlation: pending.intent.Correlation, Summary: "cancelled"}, ""
	}
	s.lifecycle.turnID, s.lifecycle.active, s.lifecycle.steerable, s.lifecycle.turnNotified = "", false, false, false
	s.sequence++
	return providersession.Event{ContractVersion: providersession.ContractVersion, Sequence: s.sequence, OccurredAt: nowUTC(), Session: s.session, Kind: providersession.EventProgress, Correlation: s.eventCorrelation(turnID, ""), Summary: "turn_" + status}, ""
}
