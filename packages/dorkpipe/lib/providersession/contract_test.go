package providersession

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func decisionCorrelation() Correlation {
	return Correlation{ProcessIncarnationID: "process", ConnectionID: "connection", SessionID: "session", InteractionID: "interaction", ActivityID: "activity", RequestID: "request", DecisionID: "decision"}
}

func TestFailClosedTransitions(t *testing.T) {
	if CanTransition(StateDisconnected, StateReady, false) {
		t.Fatal("unverified recovery must remain disconnected")
	}
	if !CanTransition(StateDisconnected, StateReady, true) {
		t.Fatal("verified recovery must permit ready")
	}
	if !CanTransition(StateRunning, StateWaitingForApproval, false) || !CanTransition(StateWaitingForApproval, StateRunning, false) {
		t.Fatal("approval wait transitions must be explicit")
	}
	if CanTransition(StateCompleted, StateRunning, true) {
		t.Fatal("terminal sessions must not restart")
	}
}

func TestSequenceRejectsDuplicateStaleAndGappedEvents(t *testing.T) {
	if err := ValidateNextSequence(7, 8); err != nil {
		t.Fatalf("next sequence: %v", err)
	}
	for _, next := range []uint64{0, 7, 6, 9} {
		if err := ValidateNextSequence(7, next); err == nil {
			t.Fatalf("sequence %d must be rejected", next)
		}
	}
}

func TestApprovalRequiresCompleteOneTimeCorrelation(t *testing.T) {
	event := Event{ContractVersion: ContractVersion, Sequence: 1, OccurredAt: time.Now(), Session: SessionRef{Provider: "example", SessionID: "session"}, Kind: EventApprovalRequested, Approval: &ApprovalRequest{Correlation: decisionCorrelation(), ActionClass: "workspace_change", Summary: "Apply reviewed change"}}
	if err := event.Validate(); err != nil {
		t.Fatalf("valid approval event: %v", err)
	}
	event.Approval.Correlation.DecisionID = ""
	if err := event.Validate(); err == nil {
		t.Fatal("missing decision identity must be rejected")
	}
}

func TestApprovalDecisionIsNeutralAndBounded(t *testing.T) {
	for _, decision := range []string{DecisionApprove, DecisionDeny} {
		if err := (ApprovalDecision{Correlation: decisionCorrelation(), Decision: decision}).Validate(); err != nil {
			t.Fatalf("valid decision %q: %v", decision, err)
		}
	}
	if err := (ApprovalDecision{Correlation: decisionCorrelation(), Decision: "acceptForSession"}).Validate(); err == nil {
		t.Fatal("provider-specific session grant must be rejected")
	}
}

func TestEventKindsRequireTheirSafeReferences(t *testing.T) {
	session := SessionRef{Provider: "example", SessionID: "session"}
	input := Event{ContractVersion: ContractVersion, Sequence: 1, OccurredAt: time.Now(), Session: session, Kind: EventUserInputRequested, UserInput: &UserInputRequest{Correlation: decisionCorrelation(), PromptRef: "artifact://prompt/1"}}
	if err := input.Validate(); err != nil {
		t.Fatalf("valid user-input event: %v", err)
	}
	cancellation := Event{ContractVersion: ContractVersion, Sequence: 2, OccurredAt: time.Now(), Session: session, Kind: EventCancellationRequested, Cancellation: &CancellationIntent{Session: session, Correlation: decisionCorrelation(), Reason: "user_requested"}}
	if err := cancellation.Validate(); err != nil {
		t.Fatalf("valid cancellation event: %v", err)
	}
}

func TestCancellationReasonsAreNeutralAndBounded(t *testing.T) {
	intent := CancellationIntent{Session: SessionRef{Provider: "example", SessionID: "session"}, Correlation: decisionCorrelation(), Reason: CancellationReasonUserRequested}
	for _, reason := range []string{CancellationReasonUserRequested, CancellationReasonSafetyStop, CancellationReasonDeadline} {
		intent.Reason = reason
		if err := intent.Validate(); err != nil {
			t.Fatalf("valid cancellation reason %q: %v", reason, err)
		}
	}
	intent.Reason = "provider_specific_reason"
	if err := intent.Validate(); err == nil {
		t.Fatal("unbounded cancellation reason must be rejected")
	}
}

func TestRecoveryRequestRequiresExactBoundedEvidence(t *testing.T) {
	request := RecoveryRequest{Session: SessionRef{Provider: "example", SessionID: "session"}, RecoveryEvidence: "recovery-safe_1"}
	if err := request.Validate(); err != nil {
		t.Fatalf("valid recovery request: %v", err)
	}
	for _, evidence := range []string{"", "unsafe/evidence", strings.Repeat("x", 129)} {
		request.RecoveryEvidence = evidence
		if err := request.Validate(); err == nil {
			t.Fatalf("unsafe recovery evidence %q was accepted", evidence)
		}
	}
}

func TestContractSourceDoesNotLeakProviderProtocolTypes(t *testing.T) {
	_, source, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("contract source location unavailable")
	}
	data, err := os.ReadFile(filepath.Join(filepath.Dir(source), "contract.go"))
	if err != nil {
		t.Fatal(err)
	}
	lower := strings.ToLower(string(data))
	for _, forbidden := range []string{"codex", "jsonrpc", "rawmessage", "threadid", "turnid", "itemid", "credential", "token"} {
		if strings.Contains(lower, forbidden) {
			t.Fatalf("generic contract leaks forbidden provider detail %q", forbidden)
		}
	}
}
