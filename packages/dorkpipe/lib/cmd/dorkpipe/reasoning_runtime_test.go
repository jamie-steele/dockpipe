package main

import "testing"

func TestResolveRuntimePolicy_ActivatesForArchitectureChat(t *testing.T) {
	t.Parallel()
	policy := resolveRuntimePolicy("chat", "Explain the architecture and tradeoffs of ask mode.", "", "", 5, 0)
	if !policy.HighAmbiguity {
		t.Fatalf("expected high ambiguity policy, got %#v", policy)
	}
	if !policy.BranchingActive {
		t.Fatalf("expected branching to activate, got %#v", policy)
	}
	if policy.BestOfN < 1 || policy.MaxBranches < 1 {
		t.Fatalf("expected policy knobs to be populated, got %#v", policy)
	}
}

func TestBuildEditBranchAttempts_PrunesLowerRankedBranches(t *testing.T) {
	t.Parallel()
	policy := runtimePolicy{BestOfN: 3, MaxBranches: 3, BranchingActive: true}
	attempts, decision := buildEditBranchAttempts([]string{"a.go", "b.go", "c.go"}, &editPlan{TargetFiles: []string{"a.go", "b.go"}}, policy)
	if len(attempts) != 3 {
		t.Fatalf("expected 3 attempts, got %#v", attempts)
	}
	if !attempts[0].Selected || attempts[0].Status != "selected" {
		t.Fatalf("expected first branch selected, got %#v", attempts[0])
	}
	if !attempts[1].Pruned || attempts[1].PrunedReason == "" {
		t.Fatalf("expected lower-ranked branch to be pruned, got %#v", attempts[1])
	}
	if decision.SelectedAttemptID == "" || decision.BranchesPruned != 2 {
		t.Fatalf("unexpected decision %#v", decision)
	}
}
