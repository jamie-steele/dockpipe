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
	if bytes.Contains(b, []byte(".tmp/")) {
		t.Fatalf("did not expect repo-root .tmp/ to be ignored:\n%s", b)
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

func TestAppendDockpipeGitignoreRemovesLegacyRootTmpFromManagedBlock(t *testing.T) {
	repo := t.TempDir()
	gitInit(t, repo)
	path := filepath.Join(repo, ".gitignore")
	legacy := "# User-owned rule\ncustom/\n" + dockpipeGitignoreBegin + "\n.tmp/\n" + dockpipeGitignoreEnd + "\n"
	if err := os.WriteFile(path, []byte(legacy), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := appendDockpipeGitignore(repo); err != nil {
		t.Fatal(err)
	}
	actual, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(actual), "\n.tmp/\n") {
		t.Fatalf("expected legacy root .tmp/ entry to be removed from managed block:\n%s", actual)
	}
	if !strings.Contains(string(actual), "# User-owned rule\ncustom/\n") {
		t.Fatalf("expected user-owned entries outside managed block to remain:\n%s", actual)
	}
}

func TestAppendDockpipeGitignoreWithoutGitErrors(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("GIT_CEILING_DIRECTORIES", filepath.Dir(dir))
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

	stderr := captureStderr(t, func() {
		if err := cmdInit([]string{"--gitignore"}); err != nil {
			t.Fatalf("cmdInit: %v", err)
		}
	})
	b, err := os.ReadFile(filepath.Join(project, ".gitignore"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "bin/.dockpipe/") {
		t.Fatalf("expected bin/.dockpipe/ in .gitignore, got:\n%s", b)
	}
	if strings.Contains(string(b), ".tmp/") {
		t.Fatalf("did not expect repo-root .tmp/ to be ignored, got:\n%s", b)
	}
	if strings.Contains(string(b), ".dorkpipe/") {
		t.Fatalf("did not expect legacy .dorkpipe/ entry in .gitignore, got:\n%s", b)
	}
	for _, want := range []string{"unit=init.project", "unit=init.gitignore", "status=done"} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("expected init stderr to contain %q, got:\n%s", want, stderr)
		}
	}
}
