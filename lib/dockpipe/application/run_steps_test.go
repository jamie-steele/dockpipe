package application

import (
	"path/filepath"
	"strings"
	"testing"

	"dockpipe/lib/dockpipe/domain"
)

func boolPtr(v bool) *bool { return &v }

// TestValidateParallelOutputPaths rejects duplicate outputs paths within one async group.
func TestValidateParallelOutputPaths(t *testing.T) {
	wf := &domain.Workflow{
		Steps: []domain.Step{
			{Outputs: ".dockpipe/a.env", Blocking: boolPtr(false)},
			{Outputs: ".dockpipe/b.env", Blocking: boolPtr(false)},
		},
	}
	if err := validateParallelOutputPaths(wf, 0, 2); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	wf.Steps[1].Outputs = ".dockpipe/a.env"
	if err := validateParallelOutputPaths(wf, 0, 2); err == nil || !strings.Contains(err.Error(), "duplicate outputs path") {
		t.Fatalf("expected duplicate outputs path error, got %v", err)
	}
}

// TestValidateParallelNoHostCommit forbids bundled commit-worktree in parallel async steps.
func TestValidateParallelNoHostCommit(t *testing.T) {
	repoRoot := testRepoRoot(t)
	o := &runStepsOpts{
		repoRoot: repoRoot,
		wfRoot:   filepath.Join(repoRoot, "templates/llm-worktree"),
		wf: &domain.Workflow{
			Steps: []domain.Step{
				{Action: "scripts/commit-worktree.sh", Blocking: boolPtr(false)},
			},
		},
	}
	err := validateParallelNoHostCommit(o, 0, 1)
	if err == nil || !strings.Contains(err.Error(), "cannot run inside a parallel") {
		t.Fatalf("expected host-commit-in-parallel error, got %v", err)
	}

	o.wf.Steps[0].Action = "scripts/print-summary.sh"
	if err := validateParallelNoHostCommit(o, 0, 1); err != nil {
		t.Fatalf("expected non-commit action to pass, got %v", err)
	}
}

// TestParseStepArgv splits shell command lines for container argv and treats blank as nil argv.
func TestParseStepArgv(t *testing.T) {
	argv, err := parseStepArgv("echo hello")
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if len(argv) != 2 || argv[0] != "echo" || argv[1] != "hello" {
		t.Fatalf("unexpected argv: %v", argv)
	}

	argv, err = parseStepArgv("   ")
	if err != nil {
		t.Fatalf("blank command should not error: %v", err)
	}
	if argv != nil {
		t.Fatalf("blank command should return nil argv, got %v", argv)
	}
}

// TestMergeStepVarsRespectsLocks applies step vars to unlocked keys only (CLI-locked keys unchanged).
func TestMergeStepVarsRespectsLocks(t *testing.T) {
	o := &runStepsOpts{
		envMap: map[string]string{"LOCKED": "old", "FREE": "old"},
		locked: map[string]bool{"LOCKED": true},
	}
	dockerEnv := map[string]string{"LOCKED": "old", "FREE": "old"}
	step := domain.Step{
		Vars: map[string]string{
			"LOCKED": "new",
			"FREE":   "new",
		},
	}
	mergeStepVars(o, step, dockerEnv)

	if got := o.envMap["LOCKED"]; got != "old" {
		t.Fatalf("locked key mutated: %q", got)
	}
	if got := o.envMap["FREE"]; got != "new" {
		t.Fatalf("free key not updated: %q", got)
	}
	if got := dockerEnv["LOCKED"]; got != "old" {
		t.Fatalf("docker locked key mutated: %q", got)
	}
	if got := dockerEnv["FREE"]; got != "new" {
		t.Fatalf("docker free key not updated: %q", got)
	}
}

