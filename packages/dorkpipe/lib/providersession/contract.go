// Package providersession defines the provider-neutral contract for top-level sessions.
// Implementations belong to future adapter slices; this package owns no transport or process behavior.
package providersession

import (
	"context"
	"errors"
	"strings"
	"time"
)

const ContractVersion = "dorkpipe.provider_session.v1"

type State string

const (
	StateReady               State = "ready"
	StateRunning             State = "running"
	StateWaitingForApproval  State = "waiting_for_approval"
	StateWaitingForUserInput State = "waiting_for_user_input"
	StateCompleted           State = "completed"
	StateCancelled           State = "cancelled"
	StateFailed              State = "failed"
	StateDisconnected        State = "disconnected"
)

func (s State) IsTerminal() bool {
	return s == StateCompleted || s == StateCancelled || s == StateFailed
}

func (s State) IsKnown() bool {
	switch s {
	case StateReady, StateRunning, StateWaitingForApproval, StateWaitingForUserInput, StateCompleted, StateCancelled, StateFailed, StateDisconnected:
		return true
	default:
		return false
	}
}

// CanTransition is intentionally fail-closed. A disconnected session may return to ready
// only when a future adapter has completed its verified recovery check.
func CanTransition(from, to State, recoveryVerified bool) bool {
	if !from.IsKnown() || !to.IsKnown() || from == to || from.IsTerminal() {
		return false
	}
	switch from {
	case StateReady:
		return to == StateRunning || to == StateFailed || to == StateDisconnected
	case StateRunning:
		return to == StateWaitingForApproval || to == StateWaitingForUserInput || to.IsTerminal() || to == StateDisconnected
	case StateWaitingForApproval, StateWaitingForUserInput:
		return to == StateRunning || to == StateCancelled || to == StateFailed || to == StateDisconnected
	case StateDisconnected:
		return to == StateReady && recoveryVerified
	default:
		return false
	}
}

// ValidateNextSequence rejects duplicate, stale, and gapped events before an
// adapter applies them. Persistence and reconciliation remain future work.
func ValidateNextSequence(previous, next uint64) error {
	if next == 0 || next != previous+1 {
		return errors.New("event sequence must advance by one")
	}
	return nil
}

type SessionRef struct {
	Provider  string `json:"provider"`
	SessionID string `json:"session_id"`
}

func (r SessionRef) Validate() error {
	if strings.TrimSpace(r.Provider) == "" || strings.TrimSpace(r.SessionID) == "" {
		return errors.New("provider and session identity are required")
	}
	return nil
}

// Correlation is opaque to callers. A provider adapter maps its own identifiers into
// these neutral scopes and must require every field for a one-time human decision.
type Correlation struct {
	ProcessIncarnationID string `json:"process_incarnation_id"`
	ConnectionID         string `json:"connection_id"`
	SessionID            string `json:"session_id"`
	InteractionID        string `json:"interaction_id"`
	ActivityID           string `json:"activity_id"`
	RequestID            string `json:"request_id"`
	DecisionID           string `json:"decision_id"`
}

func (c Correlation) ValidateForDecision() error {
	for _, value := range []string{c.ProcessIncarnationID, c.ConnectionID, c.SessionID, c.InteractionID, c.ActivityID, c.RequestID, c.DecisionID} {
		if strings.TrimSpace(value) == "" {
			return errors.New("complete correlation is required for a decision")
		}
	}
	return nil
}

type ApprovalRequest struct {
	Correlation Correlation `json:"correlation"`
	ActionClass string      `json:"action_class"`
	Summary     string      `json:"summary"`
	Scope       []string    `json:"scope,omitempty"`
}

func (r ApprovalRequest) Validate() error {
	if err := r.Correlation.ValidateForDecision(); err != nil {
		return err
	}
	if strings.TrimSpace(r.ActionClass) == "" || strings.TrimSpace(r.Summary) == "" {
		return errors.New("approval action class and summary are required")
	}
	return nil
}

type UserInputRequest struct {
	Correlation Correlation `json:"correlation"`
	PromptRef   string      `json:"prompt_ref"`
}

func (r UserInputRequest) Validate() error {
	if err := r.Correlation.ValidateForDecision(); err != nil {
		return err
	}
	if strings.TrimSpace(r.PromptRef) == "" {
		return errors.New("user-input prompt reference is required")
	}
	return nil
}

type CancellationIntent struct {
	Session     SessionRef  `json:"session"`
	Correlation Correlation `json:"correlation"`
	Reason      string      `json:"reason"`
}

const (
	CancellationReasonUserRequested = "user_requested"
	CancellationReasonSafetyStop    = "safety_stop"
	CancellationReasonDeadline      = "deadline_exceeded"
)

func (i CancellationIntent) Validate() error {
	if err := i.Session.Validate(); err != nil {
		return err
	}
	if strings.TrimSpace(i.Correlation.SessionID) == "" || strings.TrimSpace(i.Correlation.InteractionID) == "" {
		return errors.New("session and interaction are required for cancellation")
	}
	switch i.Reason {
	case CancellationReasonUserRequested, CancellationReasonSafetyStop, CancellationReasonDeadline:
		return nil
	default:
		return errors.New("known cancellation reason is required")
	}
}

type EventKind string

const (
	EventStateChanged          EventKind = "state_changed"
	EventProgress              EventKind = "progress"
	EventApprovalRequested     EventKind = "approval_requested"
	EventUserInputRequested    EventKind = "user_input_requested"
	EventCancellationRequested EventKind = "cancellation_requested"
	EventRecoveryRequired      EventKind = "recovery_required"
)

type Event struct {
	ContractVersion string              `json:"contract_version"`
	Sequence        uint64              `json:"sequence"`
	OccurredAt      time.Time           `json:"occurred_at"`
	Session         SessionRef          `json:"session"`
	Kind            EventKind           `json:"kind"`
	State           State               `json:"state,omitempty"`
	Correlation     Correlation         `json:"correlation,omitempty"`
	Summary         string              `json:"summary,omitempty"`
	Approval        *ApprovalRequest    `json:"approval,omitempty"`
	UserInput       *UserInputRequest   `json:"user_input,omitempty"`
	Cancellation    *CancellationIntent `json:"cancellation,omitempty"`
}

func (e Event) Validate() error {
	if e.ContractVersion != ContractVersion || e.Sequence == 0 || e.OccurredAt.IsZero() {
		return errors.New("event contract version, sequence, and timestamp are required")
	}
	if err := e.Session.Validate(); err != nil {
		return err
	}
	switch e.Kind {
	case EventStateChanged:
		if !e.State.IsKnown() {
			return errors.New("known state is required")
		}
	case EventApprovalRequested:
		if e.Approval == nil {
			return errors.New("approval request is required")
		}
		return e.Approval.Validate()
	case EventUserInputRequested:
		if e.UserInput == nil {
			return errors.New("user-input request is required")
		}
		return e.UserInput.Validate()
	case EventCancellationRequested:
		if e.Cancellation == nil {
			return errors.New("cancellation intent is required")
		}
		return e.Cancellation.Validate()
	case EventProgress, EventRecoveryRequired:
		return nil
	default:
		return errors.New("known event kind is required")
	}
	return nil
}

type StartRequest struct {
	WorkspaceRef string `json:"workspace_ref"`
	PolicyRef    string `json:"policy_ref"`
	InputRef     string `json:"input_ref"`
}

type InteractionRequest struct {
	Session     SessionRef  `json:"session"`
	InputRef    string      `json:"input_ref"`
	Correlation Correlation `json:"correlation"`
}

type ApprovalDecision struct {
	Correlation Correlation `json:"correlation"`
	Decision    string      `json:"decision"`
}

const (
	DecisionApprove = "approve"
	DecisionDeny    = "deny"
)

// Validate keeps decision values provider-neutral and deliberately bounded.
// Adapters map these one-turn human choices to their private protocol values;
// session grants, policy changes, and user-input answers need separate future
// contract surfaces.
func (d ApprovalDecision) Validate() error {
	if err := d.Correlation.ValidateForDecision(); err != nil {
		return err
	}
	if d.Decision != DecisionApprove && d.Decision != DecisionDeny {
		return errors.New("known approval decision is required")
	}
	return nil
}

type RecoveryRequest struct {
	Session          SessionRef `json:"session"`
	RecoveryEvidence string     `json:"recovery_evidence"`
}

// Adapter is a contract only. CAS-03 and later own process supervision, transport,
// lifecycle execution, approval delivery, and persistence.
type Adapter interface {
	Start(context.Context, StartRequest) (SessionRef, error)
	Send(context.Context, InteractionRequest) (Correlation, error)
	Decide(context.Context, ApprovalDecision) error
	Cancel(context.Context, CancellationIntent) error
	Recover(context.Context, RecoveryRequest) (SessionRef, error)
}
