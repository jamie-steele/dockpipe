package application

import (
	"path/filepath"
	"strings"
	"testing"

	"dockpipe/lib/dockpipe/domain"
)

// TestBuildStepContainer_UsesCliArgsForLastStep uses argv after -- when the last step has no cmd in YAML.
func TestBuildStepContainer_UsesCliArgsForLastStep(t *testing.T) {
	repoRoot := testRepoRoot(t)
	o := &runStepsOpts{
		repoRoot:    repoRoot,
		wfRoot:      filepath.Join(repoRoot, "templates", "test"),
		wf:          &domain.Workflow{Isolate: "base-dev"},
		cliArgs:     []string{"echo", "from-cli"},
		opts:        &CliOpts{},
		resolver:    "",
		userIsolate: "",
	}
	envMap := map[string]string{}
	dockerEnv := map[string]string{}
	step := domain.Step{} // no cmd
	argv, runOpts, buildDir, buildCtx, err := buildStepContainer(o, 0, 1, step, envMap, dockerEnv, nil)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if len(argv) != 2 || argv[0] != "echo" || argv[1] != "from-cli" {
		t.Fatalf("expected cli args fallback, got %v", argv)
	}
	if runOpts.Image == "" || buildDir == "" || buildCtx == "" {
		t.Fatalf("expected resolved image/build paths, got image=%q buildDir=%q buildCtx=%q", runOpts.Image, buildDir, buildCtx)
	}
}

// TestBuildStepContainer_ErrorsWhenActionMissing when act script path does not exist.
func TestBuildStepContainer_ErrorsWhenActionMissing(t *testing.T) {
	repoRoot := testRepoRoot(t)
	o := &runStepsOpts{
		repoRoot: repoRoot,
		wfRoot:   filepath.Join(repoRoot, "templates", "test"),
		wf:       &domain.Workflow{Isolate: "base-dev"},
		opts:     &CliOpts{},
	}
	step := domain.Step{Cmd: "echo hi", Action: "scripts/does-not-exist.sh"}
	_, _, _, _, err := buildStepContainer(o, 0, 1, step, map[string]string{}, map[string]string{}, nil)
	if err == nil || !strings.Contains(err.Error(), "action script not found") {
		t.Fatalf("expected missing action error, got %v", err)
	}
}

// TestBuildStepContainer_CommitWorktreeTurnsIntoHostCommit maps bundled commit-worktree to CommitOnHost instead of in-container act.
func TestBuildStepContainer_CommitWorktreeTurnsIntoHostCommit(t *testing.T) {
	repoRoot := testRepoRoot(t)
	o := &runStepsOpts{
		repoRoot: repoRoot,
		wfRoot:   filepath.Join(repoRoot, "templates", "test"),
		wf:       &domain.Workflow{Isolate: "base-dev"},
		opts:     &CliOpts{},
	}
	envMap := map[string]string{}
	step := domain.Step{Cmd: "echo hi", Action: "scripts/commit-worktree.sh"}
	_, runOpts, _, _, err := buildStepContainer(o, 0, 1, step, envMap, map[string]string{}, nil)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if !runOpts.CommitOnHost {
		t.Fatalf("expected CommitOnHost=true")
	}
	if runOpts.ActionPath != "" {
		t.Fatalf("expected ActionPath cleared for host commit, got %q", runOpts.ActionPath)
	}
	if envMap["DOCKPIPE_BRANCH_PREFIX"] == "" {
		t.Fatalf("expected branch prefix to be set for host commit path")
	}
}

// TestBuildStepContainer_StepResolverTemplate uses DOCKPIPE_RESOLVER_TEMPLATE from a per-step resolver assignment.
func TestBuildStepContainer_StepResolverTemplate(t *testing.T) {
	repoRoot := testRepoRoot(t)
	o := &runStepsOpts{
		repoRoot: repoRoot,
		wfRoot:   filepath.Join(repoRoot, "templates", "test"),
		wf:       &domain.Workflow{},
		opts:     &CliOpts{},
	}
	step := domain.Step{Cmd: "echo hi"}
	ra := &domain.ResolverAssignments{Template: "vscode"}
	_, runOpts, _, _, err := buildStepContainer(o, 0, 1, step, map[string]string{}, map[string]string{}, ra)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if !strings.Contains(runOpts.Image, "vscode") {
		t.Fatalf("expected vscode image from resolver template, got %q", runOpts.Image)
	}
}
