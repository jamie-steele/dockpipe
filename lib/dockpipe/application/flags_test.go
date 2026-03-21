package application

import (
	"path/filepath"
	"strings"
	"testing"
)

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
		"--resolver", "claude",
		"--mount", "/h:/c",
		"--env", "A=B",
		"--env-file", ".env.local",
		"--var", "X=1",
		"--build", "images/dev",
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
	if o.Workflow != "demo" || o.Isolate != "codex" || o.Action != "scripts/act.sh" {
		t.Fatalf("basic options mismatch: %+v", o)
	}
	if o.RepoURL == "" || o.RepoBranch != "feat/x" || o.WorkPath != "subdir" {
		t.Fatalf("repo/work options mismatch: %+v", o)
	}
	if got, want := o.BuildPath, filepath.Join(repoRoot, "images/dev"); got != want {
		t.Fatalf("BuildPath = %q, want %q", got, want)
	}
	if !o.Reinit || !o.Force || !o.Detach {
		t.Fatalf("bool flags mismatch: %+v", o)
	}
}

func TestParseFlagsWorkflowFile(t *testing.T) {
	_, o, err := ParseFlags("/tmp/repo", []string{"--workflow-file", "/projects/foo/dockpipe.yml"})
	if err != nil {
		t.Fatalf("ParseFlags() err = %v", err)
	}
	if o.WorkflowFile != "/projects/foo/dockpipe.yml" {
		t.Fatalf("WorkflowFile = %q", o.WorkflowFile)
	}
}

func TestParseFlagsWorkflowAndWorkflowFileConflict(t *testing.T) {
	_, _, err := ParseFlags("/tmp/repo", []string{"--workflow", "demo", "--workflow-file", "/x/dockpipe.yml"})
	if err == nil || !strings.Contains(err.Error(), "both") {
		t.Fatalf("expected mutual exclusion error, got %v", err)
	}
}

func TestParseFlagsBuildAbsolutePath(t *testing.T) {
	_, o, err := ParseFlags("/tmp/repo", []string{"--build", "/abs/build"})
	if err != nil {
		t.Fatalf("ParseFlags() err = %v", err)
	}
	if o.BuildPath != "/abs/build" {
		t.Fatalf("BuildPath = %q, want /abs/build", o.BuildPath)
	}
}

func TestParseFlagsVarValidation(t *testing.T) {
	_, _, err := ParseFlags("/tmp/repo", []string{"--var", "BROKEN"})
	if err == nil || !strings.Contains(err.Error(), "--var requires KEY=VAL") {
		t.Fatalf("expected --var validation error, got %v", err)
	}
}

func TestParseFlagsUnknownOption(t *testing.T) {
	_, _, err := ParseFlags("/tmp/repo", []string{"--def-not-real"})
	if err == nil || !strings.Contains(err.Error(), "unknown option") {
		t.Fatalf("expected unknown option error, got %v", err)
	}
}

func TestParseFlagsUnexpectedPositionalBeforeDash(t *testing.T) {
	_, _, err := ParseFlags("/tmp/repo", []string{"echo"})
	if err == nil || !strings.Contains(err.Error(), "expected options before --") {
		t.Fatalf("expected positional-before-dash error, got %v", err)
	}
}

