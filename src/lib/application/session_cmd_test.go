package application

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"dockpipe/src/lib/infrastructure"
)

func TestSessionCommandsListInspectSwitch(t *testing.T) {
	repo := initSessionCommandRepo(t)
	session, err := infrastructure.CreateSessionBranch(infrastructure.GitSessionRequest{
		WorkspaceID:  "demo",
		SourceDir:    repo,
		Mode:         "managed",
		BranchPrefix: "ai",
		SessionID:    "cmd-session",
	})
	if err != nil {
		t.Fatalf("CreateSessionBranch: %v", err)
	}
	defer removeSessionCommandWorktree(t, repo, session.Storage.Workspace)

	listOut, err := captureStdout(t, func() error {
		return cmdSession([]string{"list", "--workdir", repo})
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(listOut, "cmd-session") || !strings.Contains(listOut, session.Repo.SessionRef) {
		t.Fatalf("list output missing session: %q", listOut)
	}

	inspectOut, err := captureStdout(t, func() error {
		return cmdSession([]string{"inspect", "cmd", "--workdir", repo})
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(inspectOut, "Session:") || !strings.Contains(inspectOut, session.Storage.Workspace) {
		t.Fatalf("inspect output missing details: %q", inspectOut)
	}

	switchOut, err := captureStdout(t, func() error {
		return cmdSession([]string{"switch", "cmd-session", "--workdir", repo})
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(switchOut, session.Storage.Workspace) || !strings.Contains(switchOut, "Branch:") {
		t.Fatalf("switch output missing handoff: %q", switchOut)
	}
}

func initSessionCommandRepo(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	repo := t.TempDir()
	git := func(args ...string) {
		t.Helper()
		out, err := exec.Command("git", append([]string{"-C", repo}, args...)...).CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	git("init")
	git("config", "user.email", "test@example.invalid")
	git("config", "user.name", "DockPipe Test")
	if err := os.WriteFile(filepath.Join(repo, "README.md"), []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git("add", "README.md")
	git("commit", "-m", "init")
	return repo
}

func removeSessionCommandWorktree(t *testing.T, repo, workspace string) {
	t.Helper()
	out, err := exec.Command("git", "-C", repo, "worktree", "remove", "--force", workspace).CombinedOutput()
	if err != nil {
		t.Fatalf("git worktree remove: %v\n%s", err, out)
	}
}
