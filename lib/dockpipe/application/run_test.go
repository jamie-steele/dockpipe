package application

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"dockpipe/lib/dockpipe/domain"
	"dockpipe/lib/dockpipe/infrastructure"
)

func withRunSeams(t *testing.T) {
	t.Helper()
	t.Setenv("DOCKPIPE_SKIP_DOCKER_PREFLIGHT", "1")
	oldRepoRoot := repoRootAppFn
	oldLoadWf := loadWorkflowAppFn
	oldLoadResolver := loadResolverFileAppFn
	oldTemplateBuild := templateBuildAppFn
	oldMaybeTag := maybeVersionTagAppFn
	oldResolveAction := resolveActionPathFn
	oldSourceHost := sourceHostScriptAppFn
	oldDockerBuild := dockerBuildAppFn
	oldRunContainer := runContainerAppFn
	oldResolvePre := resolvePreScriptAppFn
	oldResolveWfScript := resolveWorkflowAppFn
	oldBundledCommit := isBundledCommitAppFn
	oldRunSteps := runStepsAppFn
	oldExit := osExitAppFn
	t.Cleanup(func() {
		repoRootAppFn = oldRepoRoot
		loadWorkflowAppFn = oldLoadWf
		loadResolverFileAppFn = oldLoadResolver
		templateBuildAppFn = oldTemplateBuild
		maybeVersionTagAppFn = oldMaybeTag
		resolveActionPathFn = oldResolveAction
		sourceHostScriptAppFn = oldSourceHost
		dockerBuildAppFn = oldDockerBuild
		runContainerAppFn = oldRunContainer
		resolvePreScriptAppFn = oldResolvePre
		resolveWorkflowAppFn = oldResolveWfScript
		isBundledCommitAppFn = oldBundledCommit
		runStepsAppFn = oldRunSteps
		osExitAppFn = oldExit
	})
}

// TestRunNonStepsHappyPath runs resolver-driven single-command mode with mocked docker build/run.
func TestRunNonStepsHappyPath(t *testing.T) {
	withRunSeams(t)
	repoRoot := t.TempDir()
	repoRootAppFn = func() (string, error) { return repoRoot, nil }
	loadResolverFileAppFn = func(path string) (map[string]string, error) {
		return map[string]string{"DOCKPIPE_RESOLVER_TEMPLATE": "codex"}, nil
	}
	templateBuildAppFn = func(repoRoot, name string) (string, string, bool) {
		if name == "codex" {
			return "dockpipe-codex", "/build/codex", true
		}
		return "", "", false
	}
	maybeVersionTagAppFn = func(repoRoot, image string) string { return image }
	dockerBuilt := false
	dockerBuildAppFn = func(image, dockerfileDir, contextDir string) error {
		dockerBuilt = true
		if image != "dockpipe-codex" || dockerfileDir != "/build/codex" || contextDir != repoRoot {
			t.Fatalf("unexpected docker build args: %q %q %q", image, dockerfileDir, contextDir)
		}
		return nil
	}
	containerRan := false
	runContainerAppFn = func(o infrastructure.RunOpts, argv []string) (int, error) {
		containerRan = true
		if o.Image != "dockpipe-codex" {
			t.Fatalf("unexpected image: %q", o.Image)
		}
		if len(argv) != 2 || argv[0] != "echo" || argv[1] != "hi" {
			t.Fatalf("unexpected argv: %#v", argv)
		}
		return 0, nil
	}

	err := Run([]string{"--resolver", "codex", "--", "echo", "hi"}, []string{"BASE=1"})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if !dockerBuilt || !containerRan {
		t.Fatalf("expected docker build and run, built=%v ran=%v", dockerBuilt, containerRan)
	}
}

// TestRunMissingWorkflowErrors when --workflow name has no templates/<name>/config.yml.
func TestRunMissingWorkflowErrors(t *testing.T) {
	withRunSeams(t)
	repoRoot := t.TempDir()
	repoRootAppFn = func() (string, error) { return repoRoot, nil }

	err := Run([]string{"--workflow", "missing", "--", "echo", "x"}, nil)
	if err == nil || !strings.Contains(err.Error(), `workflow "missing" not found`) {
		t.Fatalf("expected workflow missing error, got %v", err)
	}
}

// TestRunPreScriptNotFoundErrors when resolved pre-script path does not exist on disk.
func TestRunPreScriptNotFoundErrors(t *testing.T) {
	withRunSeams(t)
	repoRoot := t.TempDir()
	repoRootAppFn = func() (string, error) { return repoRoot, nil }
	loadResolverFileAppFn = func(path string) (map[string]string, error) {
		return map[string]string{"DOCKPIPE_RESOLVER_TEMPLATE": "codex"}, nil
	}
	templateBuildAppFn = func(repoRoot, name string) (string, string, bool) {
		return "dockpipe-codex", "/build", true
	}
	maybeVersionTagAppFn = func(repoRoot, image string) string { return image }
	resolvePreScriptAppFn = func(p, root string) string { return filepath.Join(root, "does-not-exist.sh") }

	err := Run([]string{"--resolver", "codex", "--run", "scripts/pre.sh", "--", "echo", "x"}, nil)
	if err == nil || !strings.Contains(err.Error(), "pre-script not found") {
		t.Fatalf("expected pre-script not found error, got %v", err)
	}
}

// TestRunMissingDashErrors when user omits the standalone -- before the command.
func TestRunMissingDashErrors(t *testing.T) {
	withRunSeams(t)
	repoRoot := t.TempDir()
	repoRootAppFn = func() (string, error) { return repoRoot, nil }
	loadResolverFileAppFn = func(path string) (map[string]string, error) {
		return map[string]string{"DOCKPIPE_RESOLVER_TEMPLATE": "codex"}, nil
	}
	templateBuildAppFn = func(repoRoot, name string) (string, string, bool) {
		return "dockpipe-codex", "/build", true
	}
	maybeVersionTagAppFn = func(repoRoot, image string) string { return image }

	err := Run([]string{"--resolver", "codex"}, nil)
	if err == nil || !strings.Contains(err.Error(), "expected -- before command") {
		t.Fatalf("expected missing -- error, got %v", err)
	}
}

// TestRunNonZeroContainerExitCallsExitFn propagates container exit code via os.Exit shim.
func TestRunNonZeroContainerExitCallsExitFn(t *testing.T) {
	withRunSeams(t)
	repoRoot := t.TempDir()
	repoRootAppFn = func() (string, error) { return repoRoot, nil }
	loadResolverFileAppFn = func(path string) (map[string]string, error) {
		return map[string]string{"DOCKPIPE_RESOLVER_TEMPLATE": "codex"}, nil
	}
	templateBuildAppFn = func(repoRoot, name string) (string, string, bool) {
		return "dockpipe-codex", "/build", true
	}
	maybeVersionTagAppFn = func(repoRoot, image string) string { return image }
	dockerBuildAppFn = func(image, dockerfileDir, contextDir string) error { return nil }
	runContainerAppFn = func(o infrastructure.RunOpts, argv []string) (int, error) { return 9, nil }
	exitCode := -1
	osExitAppFn = func(code int) { exitCode = code }

	err := Run([]string{"--resolver", "codex", "--", "echo", "hi"}, nil)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if exitCode != 9 {
		t.Fatalf("expected exit code 9, got %d", exitCode)
	}
}

// TestRunWorkflowStepsModeDelegatesToRunSteps when workflow has steps:, Run calls runStepsAppFn.
func TestRunWorkflowStepsModeDelegatesToRunSteps(t *testing.T) {
	withRunSeams(t)
	repoRoot := t.TempDir()
	repoRootAppFn = func() (string, error) { return repoRoot, nil }
	workflowDir := filepath.Join(repoRoot, "templates", "demo")
	if err := os.MkdirAll(workflowDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(workflowDir, "config.yml"), []byte("name: demo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	loadWorkflowAppFn = func(path string) (*domain.Workflow, error) {
		bFalse := false
		return &domain.Workflow{
			Resolver: "codex",
			Steps:    []domain.Step{{Cmd: "echo hi", Blocking: &bFalse}},
		}, nil
	}
	loadResolverFileAppFn = func(path string) (map[string]string, error) {
		return map[string]string{"DOCKPIPE_RESOLVER_TEMPLATE": "codex"}, nil
	}
	called := false
	runStepsAppFn = func(o runStepsOpts) error {
		called = true
		if o.resolver != "codex" {
			t.Fatalf("expected resolver from workflow, got %q", o.resolver)
		}
		return nil
	}

	err := Run([]string{"--workflow", "demo", "--", "echo", "x"}, nil)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if !called {
		t.Fatal("expected runSteps delegation in steps mode")
	}
}

// TestRunAutoBranchForRepoWithoutBranch sets work branch when --repo is set without --branch (clone-worktree flow).
func TestRunAutoBranchForRepoWithoutBranch(t *testing.T) {
	withRunSeams(t)
	repoRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repoRoot, "scripts"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repoRoot, "scripts", "clone-worktree.sh"), []byte("#!/usr/bin/env bash\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	repoRootAppFn = func() (string, error) { return repoRoot, nil }
	loadResolverFileAppFn = func(path string) (map[string]string, error) {
		return map[string]string{"DOCKPIPE_RESOLVER_TEMPLATE": "codex"}, nil
	}
	templateBuildAppFn = func(repoRoot, name string) (string, string, bool) {
		return "dockpipe-codex", "/build", true
	}
	maybeVersionTagAppFn = func(repoRoot, image string) string { return image }
	dockerBuildAppFn = func(image, dockerfileDir, contextDir string) error { return nil }
	var gotRunOpts infrastructure.RunOpts
	runContainerAppFn = func(o infrastructure.RunOpts, argv []string) (int, error) {
		gotRunOpts = o
		return 0, nil
	}

	err := Run([]string{"--resolver", "codex", "--repo", "https://example.com/r.git", "--", "echo", "x"}, nil)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if gotRunOpts.Image == "" {
		t.Fatalf("expected run container options to be populated")
	}
}

// TestRunInfersOriginWhenWorkflowUsesCloneWorktree fills --repo from git remote when workflow uses clone-worktree pre-script.
func TestRunInfersOriginWhenWorkflowUsesCloneWorktree(t *testing.T) {
	withRunSeams(t)
	gitDir := t.TempDir()
	if out, err := exec.Command("git", "-C", gitDir, "init").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}
	wantURL := "https://example.test/unite.git"
	if out, err := exec.Command("git", "-C", gitDir, "remote", "add", "origin", wantURL).CombinedOutput(); err != nil {
		t.Fatalf("git remote add: %v\n%s", err, out)
	}

	repoRoot := t.TempDir()
	wfDir := filepath.Join(repoRoot, "templates", "wfinfer")
	if err := os.MkdirAll(filepath.Join(wfDir, "resolvers"), 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := `run: scripts/clone-worktree.sh
act: scripts/noop.sh
isolate: codex
`
	if err := os.WriteFile(filepath.Join(wfDir, "config.yml"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wfDir, "resolvers", "codex"), []byte("DOCKPIPE_RESOLVER_TEMPLATE=codex\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(repoRoot, "scripts"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"clone-worktree.sh", "noop.sh"} {
		if err := os.WriteFile(filepath.Join(repoRoot, "scripts", name), []byte("#!/usr/bin/env bash\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	repoRootAppFn = func() (string, error) { return repoRoot, nil }
	templateBuildAppFn = func(repoRoot, name string) (string, string, bool) {
		return "dockpipe-codex", "/build", true
	}
	maybeVersionTagAppFn = func(repoRoot, image string) string { return image }
	dockerBuildAppFn = func(image, dockerfileDir, contextDir string) error { return nil }

	var capturedPreEnv []string
	sourceHostScriptAppFn = func(scriptPath string, env []string) (map[string]string, error) {
		capturedPreEnv = append([]string(nil), env...)
		return map[string]string{"DOCKPIPE_WORKDIR": filepath.Join(gitDir, "fake-worktree")}, nil
	}

	runContainerAppFn = func(o infrastructure.RunOpts, argv []string) (int, error) {
		return 0, nil
	}

	oldWd, _ := os.Getwd()
	if err := os.Chdir(gitDir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldWd) })

	err := Run([]string{"--workflow", "wfinfer", "--", "echo", "hi"}, nil)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	want := "DOCKPIPE_REPO_URL=" + wantURL
	found := false
	for _, e := range capturedPreEnv {
		if e == want {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected %q in pre-script env, got:\n%s", want, strings.Join(capturedPreEnv, "\n"))
	}
}
