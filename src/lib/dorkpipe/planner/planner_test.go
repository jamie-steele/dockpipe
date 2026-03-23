package planner

import (
	"testing"

	"dockpipe/src/lib/dorkpipe/spec"
)

func TestValidateDAG_OK(t *testing.T) {
	d := &spec.Doc{
		Name: "t",
		Nodes: []spec.Node{
			{ID: "a", Kind: "shell", Script: "echo 1"},
			{ID: "b", Kind: "shell", Script: "echo 2", Needs: []string{"a"}},
		},
	}
	if err := Validate(d); err != nil {
		t.Fatal(err)
	}
}

func TestValidateCycle(t *testing.T) {
	d := &spec.Doc{
		Name: "t",
		Nodes: []spec.Node{
			{ID: "a", Kind: "shell", Script: "echo", Needs: []string{"b"}},
			{ID: "b", Kind: "shell", Script: "echo", Needs: []string{"a"}},
		},
	}
	if err := Validate(d); err == nil {
		t.Fatal("expected cycle error")
	}
}

func TestBranchPolicyNeedsJudge(t *testing.T) {
	d := &spec.Doc{
		Name: "t",
		Policy: spec.Policy{
			BranchJudge: "judge",
		},
		Nodes: []spec.Node{
			{ID: "judge", Kind: "shell", Script: `echo '{"winner":"x"}'`},
			{ID: "branch", Kind: "shell", Script: "echo 1", BranchRequired: "x", Needs: []string{"judge"}},
		},
	}
	if err := Validate(d); err != nil {
		t.Fatal(err)
	}
}

func TestBranchPolicyMissingNeeds(t *testing.T) {
	d := &spec.Doc{
		Name: "t",
		Policy: spec.Policy{
			BranchJudge: "judge",
		},
		Nodes: []spec.Node{
			{ID: "judge", Kind: "shell", Script: `echo '{"winner":"x"}'`},
			{ID: "branch", Kind: "shell", Script: "echo 1", BranchRequired: "x"},
		},
	}
	if err := Validate(d); err == nil {
		t.Fatal("expected error for missing needs")
	}
}
