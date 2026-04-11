package application

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// TestMergeWorktreeGitDockerEnv adds DOCKPIPE_WORKTREE_* from host git when /work is a normal repo checkout.
func TestMergeWorktreeGitDockerEnv(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if out, err := exec.Command("git", "-C", dir, "init").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}
	if out, err := exec.Command("git", "-C", dir, "config", "user.email", "t@t").CombinedOutput(); err != nil {
		t.Fatal(err, string(out))
	}
	if out, err := exec.Command("git", "-C", dir, "config", "user.name", "t").CombinedOutput(); err != nil {
		t.Fatal(err, string(out))
	}
	readme := filepath.Join(dir, "README")
	if err := os.WriteFile(readme, []byte("hi\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("git", "-C", dir, "add", "README").CombinedOutput(); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	if out, err := exec.Command("git", "-C", dir, "commit", "-m", "init").CombinedOutput(); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	if out, err := exec.Command("git", "-C", dir, "branch", "-M", "main").CombinedOutput(); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}

	m := map[string]string{"FOO": "bar"}
	mergeWorktreeGitDockerEnv(m, dir)
	if m["FOO"] != "bar" {
		t.Fatal("lost user env")
	}
	if m["DOCKPIPE_WORKTREE_BRANCH"] != "main" {
		t.Fatalf("branch: got %q", m["DOCKPIPE_WORKTREE_BRANCH"])
	}
	if m["DOCKPIPE_WORKTREE_HEAD"] == "" {
		t.Fatal("missing HEAD")
	}
	if m["DOCKPIPE_WORKTREE_DETACHED"] != "" {
		t.Fatalf("unexpected detached: %q", m["DOCKPIPE_WORKTREE_DETACHED"])
	}
}
