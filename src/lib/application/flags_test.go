package application

import (
	"path/filepath"
	"strings"
	"testing"
)

// TestParseFlagsHappyPath verifies ParseFlags accepts a full option set, resolves relative --build, and splits command after --.
func TestParseFlagsHappyPath(t *testing.T) {
	repoRoot := "/tmp/repo"
	rest, o, err := ParseFlags(repoRoot, []string{
		"--workflow", "demo",
		"--run", "scripts/pre.sh",
		"--isolate", "codex",
		"--act", "scripts/act.sh",
		"--repo", "https://example/repo.git",
		"--branch", "feat/x",
		"--work-path", "subdir",
		"--work-branch", "dockpipe/wb",
		"--bundle-out", "out.bundle",
		"--runtime", "claude",
		"--mount", "/h:/c",
		"--env", "A=B",
		"--env-file", ".env.local",
		"--var", "X=1",
		"--build", "templates/core/assets/images/dev",
		"--data-dir", "/data",
		"--data-vol", "dockpipe-data",
		"--reinit",
		"-f",
		"-d",
		"--",
		"echo", "ok",
	})
	if err != nil {
		t.Fatalf("ParseFlags() err = %v", err)
	}
	if !o.SeenDash || len(rest) != 2 || rest[0] != "echo" || rest[1] != "ok" {
		t.Fatalf("rest/SeenDash mismatch: rest=%v seenDash=%v", rest, o.SeenDash)
	}
	if o.Workflow != "demo" || o.Isolate != "codex" || o.Action != "scripts/act.sh" || o.Runtime != "claude" {
		t.Fatalf("basic options mismatch: %+v", o)
	}
	if o.RepoURL == "" || o.RepoBranch != "feat/x" || o.WorkPath != "subdir" {
		t.Fatalf("repo/work options mismatch: %+v", o)
	}
	if got, want := o.BuildPath, filepath.Join(repoRoot, "templates/core/assets/images/dev"); got != want {
		t.Fatalf("BuildPath = %q, want %q", got, want)
	}
	if !o.Reinit || !o.Force || !o.Detach {
		t.Fatalf("bool flags mismatch: %+v", o)
	}
}

// TestParseFlagsWorkflowFile checks --workflow-file stores the path as given.
func TestParseFlagsWorkflowFile(t *testing.T) {
	_, o, err := ParseFlags("/tmp/repo", []string{"--workflow-file", "/projects/foo/workflows/demo/config.yml"})
	if err != nil {
		t.Fatalf("ParseFlags() err = %v", err)
	}
	if o.WorkflowFile != "/projects/foo/workflows/demo/config.yml" {
		t.Fatalf("WorkflowFile = %q", o.WorkflowFile)
	}
}

// TestParseFlagsWorkflowAndWorkflowFileConflict ensures --workflow and --workflow-file cannot be used together.
func TestParseFlagsWorkflowAndWorkflowFileConflict(t *testing.T) {
	_, _, err := ParseFlags("/tmp/repo", []string{"--workflow", "demo", "--workflow-file", "/x/workflows/demo/config.yml"})
	if err == nil || !strings.Contains(err.Error(), "both") {
		t.Fatalf("expected mutual exclusion error, got %v", err)
	}
}

// TestParseFlagsBuildAbsolutePath keeps POSIX-absolute --build paths without joining repo root.
func TestParseFlagsBuildAbsolutePath(t *testing.T) {
	_, o, err := ParseFlags("/tmp/repo", []string{"--build", "/abs/build"})
	if err != nil {
		t.Fatalf("ParseFlags() err = %v", err)
	}
	if o.BuildPath != "/abs/build" {
		t.Fatalf("BuildPath = %q, want /abs/build", o.BuildPath)
	}
}

// TestParseFlagsVarValidation rejects --var values that are not KEY=VAL.
func TestParseFlagsVarValidation(t *testing.T) {
	_, _, err := ParseFlags("/tmp/repo", []string{"--var", "BROKEN"})
	if err == nil || !strings.Contains(err.Error(), "--var requires KEY=VAL") {
		t.Fatalf("expected --var validation error, got %v", err)
	}
}

// TestParseFlagsNoOpInject sets NoOpInject on --no-op-inject.
func TestParseFlagsNoOpInject(t *testing.T) {
	_, o, err := ParseFlags("/tmp/repo", []string{"--workflow", "demo", "--no-op-inject"})
	if err != nil {
		t.Fatal(err)
	}
	if !o.NoOpInject {
		t.Fatalf("NoOpInject: %+v", o)
	}
}

// TestParseFlagsUnknownOption errors on unrecognized flags.
func TestParseFlagsUnknownOption(t *testing.T) {
	_, _, err := ParseFlags("/tmp/repo", []string{"--def-not-real"})
	if err == nil || !strings.Contains(err.Error(), "unknown option") {
		t.Fatalf("expected unknown option error, got %v", err)
	}
}

// TestParseFlagsRuntimeAndResolverBothAllowed allows both --runtime and --resolver.
func TestParseFlagsRuntimeAndResolverBothAllowed(t *testing.T) {
	_, o, err := ParseFlags("/tmp/repo", []string{"--runtime", "docker-node", "--resolver", "claude"})
	if err != nil {
		t.Fatal(err)
	}
	if o.Runtime != "docker-node" || o.Resolver != "claude" {
		t.Fatalf("got runtime=%q resolver=%q", o.Runtime, o.Resolver)
	}
}

// TestParseFlagsUnexpectedPositionalBeforeDash errors when a bare positional appears before --.
func TestParseFlagsUnexpectedPositionalBeforeDash(t *testing.T) {
	_, _, err := ParseFlags("/tmp/repo", []string{"echo"})
	if err == nil || !strings.Contains(err.Error(), "expected options before --") {
		t.Fatalf("expected positional-before-dash error, got %v", err)
	}
}
