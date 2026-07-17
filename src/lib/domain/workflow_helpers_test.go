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
	if got := s.OutputsPath(); got != DefaultOutputsEnvRel {
		t.Fatalf("OutputsPath default mismatch: %q", got)
	}
	r := s.RunPaths()
	if len(r) != 2 || r[0] != "a.sh" || r[1] != "b.sh" {
		t.Fatalf("RunPaths mismatch: %#v", r)
	}
}

func TestStepCWDMode(t *testing.T) {
	cases := map[string]string{
		"":          "source",
		"source":    "source",
		"repo":      "source",
		"workdir":   "source",
		"artifacts": "artifacts",
		"artifact":  "artifacts",
	}
	for input, want := range cases {
		s := Step{CWD: input}
		if got := s.CWDMode(); got != want {
			t.Fatalf("CWDMode(%q) = %q want %q", input, got, want)
		}
		if err := ValidateStepCWD(0, s); err != nil {
			t.Fatalf("ValidateStepCWD(%q): %v", input, err)
		}
	}
	if err := ValidateStepCWD(0, Step{CWD: "elsewhere"}); err == nil {
		t.Fatal("expected invalid cwd mode to fail")
	}
}

func TestStepScopesModes(t *testing.T) {
	s := Step{Scopes: StepScopes{Source: "repo", Artifacts: "artifacts"}}
	if got := s.SourceScopeMode(); got != "source" {
		t.Fatalf("SourceScopeMode = %q want source", got)
	}
	if got := s.ArtifactsScopeMode(); got != "artifacts" {
		t.Fatalf("ArtifactsScopeMode = %q want artifacts", got)
	}
	if err := ValidateStepScopes(0, s); err != nil {
		t.Fatalf("ValidateStepScopes: %v", err)
	}
	defaults := Step{}
	if got := defaults.SourceScopeMode(); got != "source" {
		t.Fatalf("default SourceScopeMode = %q want source", got)
	}
	if got := defaults.ArtifactsScopeMode(); got != "artifacts" {
		t.Fatalf("default ArtifactsScopeMode = %q want artifacts", got)
	}
	repoArtifacts := Step{Scopes: StepScopes{Artifacts: "repo"}}
	if got := repoArtifacts.ArtifactsScopeMode(); got != "source" {
		t.Fatalf("repo ArtifactsScopeMode = %q want source", got)
	}
	if err := ValidateStepScopes(0, Step{Scopes: StepScopes{Artifacts: "elsewhere"}}); err == nil {
		t.Fatal("expected invalid artifacts scope to fail")
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
