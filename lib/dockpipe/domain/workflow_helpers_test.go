package domain

import "testing"

// TestStepHelpers covers Step DisplayName, IsBlocking, ActPath, CmdLine, OutputsPath, RunPaths defaults and fallbacks.
func TestStepHelpers(t *testing.T) {
	bFalse := false
	bTrue := true
	s := Step{
		ID:        "  my-step  ",
		Run:       RunSpec{"a.sh"},
		PreScript: "b.sh",
		Act:       "act.sh",
		Cmd:       "echo hi",
		Outputs:   "out.env",
		Blocking:  &bFalse,
	}
	if s.DisplayName(0) != "my-step" {
		t.Fatalf("DisplayName should trim id, got %q", s.DisplayName(0))
	}
	if s.DisplayName(9) != "my-step" {
		t.Fatalf("DisplayName should still use id, got %q", s.DisplayName(9))
	}
	if s.IsBlocking() {
		t.Fatalf("expected non-blocking when pointer is false")
	}
	s.Blocking = &bTrue
	if !s.IsBlocking() {
		t.Fatalf("expected blocking when pointer is true")
	}
	s.Blocking = nil
	if !s.IsBlocking() {
		t.Fatalf("expected default blocking=true when pointer is nil")
	}
	if got := s.ActPath(); got != "act.sh" {
		t.Fatalf("ActPath mismatch: %q", got)
	}
	s.Act = ""
	s.Action = "action.sh"
	if got := s.ActPath(); got != "action.sh" {
		t.Fatalf("ActPath fallback mismatch: %q", got)
	}
	if got := s.CmdLine(); got != "echo hi" {
		t.Fatalf("CmdLine mismatch: %q", got)
	}
	s.Cmd = ""
	s.Command = "echo bye"
	if got := s.CmdLine(); got != "echo bye" {
		t.Fatalf("CmdLine fallback mismatch: %q", got)
	}
	if got := s.OutputsPath(); got != "out.env" {
		t.Fatalf("OutputsPath mismatch: %q", got)
	}
	s.Outputs = ""
	if got := s.OutputsPath(); got != ".dockpipe/outputs.env" {
		t.Fatalf("OutputsPath default mismatch: %q", got)
	}
	r := s.RunPaths()
	if len(r) != 2 || r[0] != "a.sh" || r[1] != "b.sh" {
		t.Fatalf("RunPaths mismatch: %#v", r)
	}
}

// TestRunSpecInvalidNodeType rejects YAML where run: is not a string or string list.
func TestRunSpecInvalidNodeType(t *testing.T) {
	_, err := ParseWorkflowYAML([]byte("run:\n  nested: x\n"))
	if err == nil {
		t.Fatal("expected parse error for invalid run node type")
	}
}

// TestFlattenStepsInternalEmptyEntryError ensures an empty step entry fails flattening.
func TestFlattenStepsInternalEmptyEntryError(t *testing.T) {
	_, err := flattenSteps([]stepOrGroupYAML{{}})
	if err == nil {
		t.Fatal("expected flattenSteps internal parse error")
	}
}
