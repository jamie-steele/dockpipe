package infrastructure

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// TestGitTopLevel resolves the git work tree root from a subdirectory.
func TestGitTopLevel(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if out, err := exec.Command("git", "-C", dir, "init").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}
	sub := filepath.Join(dir, "pkg")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	top, err := GitTopLevel(sub)
	if err != nil {
		t.Fatal(err)
	}
	fiTop, err1 := os.Stat(top)
	fiDir, err2 := os.Stat(dir)
	if err1 != nil || err2 != nil || !os.SameFile(fiTop, fiDir) {
		t.Fatalf("GitTopLevel(subdir) = %q want same dir as %q (stat: %v, %v)", top, dir, err1, err2)
	}
}

// TestGitRemoteGetURL reads origin URL from a temp repo and errors for non-repo paths.
func TestGitRemoteGetURL(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if out, err := exec.Command("git", "-C", dir, "init").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}
	remote := "https://example.com/org/repo.git"
	if out, err := exec.Command("git", "-C", dir, "remote", "add", "origin", remote).CombinedOutput(); err != nil {
		t.Fatalf("git remote add: %v\n%s", err, out)
	}
	got, err := GitRemoteGetURL(dir, "origin")
	if err != nil {
		t.Fatalf("GitRemoteGetURL: %v", err)
	}
	if got != remote {
		t.Fatalf("got %q want %q", got, remote)
	}
	_, err = GitRemoteGetURL(filepath.Join(dir, "missing-subdir"), "origin")
	if err == nil {
		t.Fatal("expected error for non-repo path")
	}
}
