package application

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func gitInit(t *testing.T, dir string) {
	t.Helper()
	if out, err := exec.Command("git", "-C", dir, "init").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}
}

func TestAppendDockpipeGitignoreCreatesFile(t *testing.T) {
	repo := t.TempDir()
	gitInit(t, repo)
	sub := filepath.Join(repo, "deep", "nested")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := appendDockpipeGitignore(sub); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(repo, ".gitignore")
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(b, []byte(dockpipeGitignoreBegin)) || !bytes.Contains(b, []byte("bin/.dockpipe/")) {
		t.Fatalf("unexpected .gitignore:\n%s", b)
	}
}

func TestAppendDockpipeGitignoreIdempotent(t *testing.T) {
	repo := t.TempDir()
	gitInit(t, repo)
	if err := appendDockpipeGitignore(repo); err != nil {
		t.Fatal(err)
	}
	first, err := os.ReadFile(filepath.Join(repo, ".gitignore"))
	if err != nil {
		t.Fatal(err)
	}
	if err := appendDockpipeGitignore(repo); err != nil {
		t.Fatal(err)
	}
	second, err := os.ReadFile(filepath.Join(repo, ".gitignore"))
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != string(second) {
		t.Fatalf("second run should not change file")
	}
}

func TestAppendDockpipeGitignoreWithoutGitErrors(t *testing.T) {
	dir := t.TempDir()
	err := appendDockpipeGitignore(dir)
	if err == nil || !strings.Contains(err.Error(), "git working tree") {
		t.Fatalf("expected git working tree error, got %v", err)
	}
}

func TestCmdInitGitignore(t *testing.T) {
	repoRoot := mkRepoRootForSubcmdTests(t)
	t.Setenv("DOCKPIPE_REPO_ROOT", repoRoot)
	project := t.TempDir()
	gitInit(t, project)
	oldWd, _ := os.Getwd()
	if err := os.Chdir(project); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldWd) })

	if err := cmdInit([]string{"--gitignore"}); err != nil {
		t.Fatalf("cmdInit: %v", err)
	}
	b, err := os.ReadFile(filepath.Join(project, ".gitignore"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), ".dorkpipe/") {
		t.Fatalf("expected .dorkpipe/ in .gitignore, got:\n%s", b)
	}
}
