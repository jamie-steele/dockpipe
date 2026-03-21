package infrastructure

import (
	"os"
	"path/filepath"
	"testing"
)

// TestResolveWorkflowScript resolves scripts/ from repo root and other paths from workflow root.
func TestResolveWorkflowScript(t *testing.T) {
	got := ResolveWorkflowScript("scripts/pre.sh", "/wf", "/repo")
	if got != "/repo/scripts/pre.sh" {
		t.Fatalf("scripts/* should resolve from repo root, got %q", got)
	}
	got = ResolveWorkflowScript("local/pre.sh", "/wf", "/repo")
	if got != "/wf/local/pre.sh" {
		t.Fatalf("non-scripts path should resolve from workflow root, got %q", got)
	}
}

// TestResolvePreScriptPath finds host pre-scripts under repo root or passes absolute paths through.
func TestResolvePreScriptPath(t *testing.T) {
	tmp := t.TempDir()
	repoRoot := filepath.Join(tmp, "repo")
	if err := os.MkdirAll(filepath.Join(repoRoot, "scripts"), 0o755); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(repoRoot, "scripts", "pre.sh")
	if err := os.WriteFile(target, []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := ResolvePreScriptPath("scripts/pre.sh", repoRoot); got != target {
		t.Fatalf("expected repo-root resolved path, got %q", got)
	}
	if got := ResolvePreScriptPath("/abs/pre.sh", repoRoot); got != "/abs/pre.sh" {
		t.Fatalf("absolute path should pass through, got %q", got)
	}
}

// TestResolveActionPath resolves act scripts from repo scripts/ when present.
func TestResolveActionPath(t *testing.T) {
	tmp := t.TempDir()
	repoRoot := filepath.Join(tmp, "repo")
	cwd := filepath.Join(tmp, "cwd")
	if err := os.MkdirAll(filepath.Join(repoRoot, "scripts"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(cwd, 0o755); err != nil {
		t.Fatal(err)
	}
	repoAction := filepath.Join(repoRoot, "scripts", "act.sh")
	if err := os.WriteFile(repoAction, []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := ResolveActionPath("act.sh", repoRoot, cwd)
	if err != nil {
		t.Fatal(err)
	}
	if got != repoAction {
		t.Fatalf("expected repo scripts resolution, got %q", got)
	}
}

// TestResolveActionPathVariants covers empty, absolute, cwd-relative, and missing action paths.
func TestResolveActionPathVariants(t *testing.T) {
	tmp := t.TempDir()
	repoRoot := filepath.Join(tmp, "repo")
	cwd := filepath.Join(tmp, "cwd")
	if err := os.MkdirAll(repoRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(cwd, 0o755); err != nil {
		t.Fatal(err)
	}

	got, err := ResolveActionPath("", repoRoot, cwd)
	if err != nil || got != "" {
		t.Fatalf("empty action should return empty path, got=%q err=%v", got, err)
	}

	absAction := filepath.Join(tmp, "abs.sh")
	if err := os.WriteFile(absAction, []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err = ResolveActionPath(absAction, repoRoot, cwd)
	if err != nil || got != absAction {
		t.Fatalf("abs action should pass through, got=%q err=%v", got, err)
	}

	cwdAction := filepath.Join(cwd, "in-cwd.sh")
	if err := os.WriteFile(cwdAction, []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err = ResolveActionPath("in-cwd.sh", repoRoot, cwd)
	if err != nil {
		t.Fatal(err)
	}
	if got != cwdAction {
		t.Fatalf("expected cwd fallback, got %q", got)
	}

	got, err = ResolveActionPath("missing.sh", repoRoot, cwd)
	if err != nil {
		t.Fatal(err)
	}
	want, _ := filepath.Abs(filepath.Join(cwd, "missing.sh"))
	if got != want {
		t.Fatalf("missing action should return cwd abs candidate, got=%q want=%q", got, want)
	}
}

// TestIsBundledCommitWorktree matches only the bundled commit-worktree.sh next to repo scripts.
func TestIsBundledCommitWorktree(t *testing.T) {
	tmp := t.TempDir()
	repoRoot := filepath.Join(tmp, "repo")
	if err := os.MkdirAll(filepath.Join(repoRoot, "scripts"), 0o755); err != nil {
		t.Fatal(err)
	}
	bundled := filepath.Join(repoRoot, "scripts", "commit-worktree.sh")
	if err := os.WriteFile(bundled, []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}

	if !IsBundledCommitWorktree(bundled, repoRoot) {
		t.Fatalf("expected bundled action to match")
	}
	other := filepath.Join(repoRoot, "scripts", "print-summary.sh")
	if IsBundledCommitWorktree(other, repoRoot) {
		t.Fatalf("unexpected match for non-bundled action")
	}
}

