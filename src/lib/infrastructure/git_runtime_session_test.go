package infrastructure

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func initGitSessionTestRepo(t *testing.T) string {
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

func TestCreateSessionBranchManagedWorktreeAndCheckpoint(t *testing.T) {
	repo := initGitSessionTestRepo(t)
	git := func(args ...string) {
		t.Helper()
		out, err := exec.Command("git", append([]string{"-C", repo}, args...)...).CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}

	session, err := CreateSessionBranch(GitSessionRequest{
		WorkspaceID:  "demo",
		SourceDir:    repo,
		Mode:         "managed",
		BranchPrefix: "ai",
		SessionID:    "test-session",
	})
	if err != nil {
		t.Fatalf("CreateSessionBranch: %v", err)
	}
	if session.Storage.Mode != "managed" || session.Storage.Backend != "worktree" {
		t.Fatalf("unexpected storage: %+v", session.Storage)
	}
	if !strings.HasPrefix(session.Repo.SessionRef, "ai/session-test-session") {
		t.Fatalf("session ref = %q", session.Repo.SessionRef)
	}
	if _, err := os.Stat(filepath.Join(session.Storage.Workspace, ".git")); err != nil {
		t.Fatalf("managed workspace not created: %v", err)
	}
	if _, err := os.Stat(filepath.Join(repo, "bin", ".dockpipe", "sessions", "test-session", "session.json")); err != nil {
		t.Fatalf("session metadata missing: %v", err)
	}

	if err := os.WriteFile(filepath.Join(session.Storage.Workspace, "generated.txt"), []byte("generated\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cp, err := CheckpointSession(session, "test checkpoint")
	if err != nil {
		t.Fatalf("CheckpointSession: %v", err)
	}
	if cp.Status != "created" || cp.Commit == "" {
		t.Fatalf("unexpected checkpoint: %+v", cp)
	}
	if _, err := os.Stat(filepath.Join(repo, "bin", ".dockpipe", "sessions", "test-session", "checkpoints", cp.CheckpointID+".json")); err != nil {
		t.Fatalf("checkpoint metadata missing from session root: %v", err)
	}
	git("worktree", "remove", "--force", session.Storage.Workspace)
}

func TestCreateSessionBranchPreservesExplicitBranchName(t *testing.T) {
	repo := initGitSessionTestRepo(t)
	session, err := CreateSessionBranch(GitSessionRequest{
		WorkspaceID:  "demo",
		SourceDir:    repo,
		Mode:         "managed",
		BranchPrefix: "ai",
		BranchName:   "js/features/spnext/reporting/worktree-report-poc",
		SessionID:    "report-poc",
	})
	if err != nil {
		t.Fatalf("CreateSessionBranch: %v", err)
	}
	if got := session.Repo.SessionRef; got != "js/features/spnext/reporting/worktree-report-poc" {
		t.Fatalf("session ref = %q", got)
	}
	if got := strings.TrimSpace(mustGitOutput(t, session.Storage.Workspace, "branch", "--show-current")); got != "js/features/spnext/reporting/worktree-report-poc" {
		t.Fatalf("workspace branch = %q", got)
	}
	gitRemoveWorktree(t, repo, session.Storage.Workspace)
}

func TestCreateSessionBranchManagedWorktreeReusesExistingBranch(t *testing.T) {
	repo := initGitSessionTestRepo(t)
	out, err := exec.Command("git", "-C", repo, "checkout", "-b", "js/features/spnext/reporting-worktree-001").CombinedOutput()
	if err != nil {
		t.Fatalf("git checkout -b existing branch: %v\n%s", err, out)
	}
	out, err = exec.Command("git", "-C", repo, "checkout", "-").CombinedOutput()
	if err != nil {
		t.Fatalf("git checkout -: %v\n%s", err, out)
	}

	session, err := CreateSessionBranch(GitSessionRequest{
		WorkspaceID: "demo",
		SourceDir:   repo,
		Mode:        "managed",
		BranchName:  "js/features/spnext/reporting-worktree-001",
		SessionID:   "reporting-worktree-001",
	})
	if err != nil {
		t.Fatalf("CreateSessionBranch: %v", err)
	}
	if got := session.Repo.SessionRef; got != "js/features/spnext/reporting-worktree-001" {
		t.Fatalf("session ref = %q", got)
	}
	if got := strings.TrimSpace(mustGitOutput(t, session.Storage.Workspace, "branch", "--show-current")); got != "js/features/spnext/reporting-worktree-001" {
		t.Fatalf("workspace branch = %q", got)
	}
	gitRemoveWorktree(t, repo, session.Storage.Workspace)
}

func TestCreateSessionBranchManagedReusesExistingSessionWorktree(t *testing.T) {
	repo := initGitSessionTestRepo(t)
	first, err := CreateSessionBranch(GitSessionRequest{
		WorkspaceID: "demo",
		SourceDir:   repo,
		Mode:        "managed",
		BranchName:  "js/features/spnext/reporting-worktree-001",
		SessionID:   "first-reporting-worktree",
	})
	if err != nil {
		t.Fatalf("CreateSessionBranch first: %v", err)
	}

	second, err := CreateSessionBranch(GitSessionRequest{
		WorkspaceID: "demo",
		SourceDir:   repo,
		Mode:        "managed",
		BranchName:  "js/features/spnext/reporting-worktree-001",
		SessionID:   "second-reporting-worktree",
	})
	if err != nil {
		t.Fatalf("CreateSessionBranch second: %v", err)
	}
	if second.SessionID != first.SessionID {
		t.Fatalf("expected session reuse, got first=%q second=%q", first.SessionID, second.SessionID)
	}
	if second.Storage.Workspace != first.Storage.Workspace {
		t.Fatalf("expected workspace reuse, got first=%q second=%q", first.Storage.Workspace, second.Storage.Workspace)
	}
	gitRemoveWorktree(t, repo, first.Storage.Workspace)
}

func TestCreateSessionBranchManagedVolumeAllocatesDockerVolumeMetadata(t *testing.T) {
	repo := initGitSessionTestRepo(t)
	oldCreate := execCommandFn
	t.Cleanup(func() {
		execCommandFn = oldCreate
	})
	execCommandFn = func(name string, args ...string) *exec.Cmd {
		if strings.Contains(strings.ToLower(filepath.Base(name)), "docker") {
			return helperExitCommand(0)
		}
		return exec.Command(name, args...)
	}
	session, err := CreateSessionBranch(GitSessionRequest{
		WorkspaceID:  "demo",
		SourceDir:    repo,
		Mode:         "managed",
		Storage:      "volume",
		BranchPrefix: "ai",
		SessionID:    "volume-session",
	})
	if err != nil {
		t.Fatalf("CreateSessionBranch: %v", err)
	}
	if session.Storage.Backend != "docker_volume" {
		t.Fatalf("unexpected backend: %+v", session.Storage)
	}
	if session.Storage.Volume == "" {
		t.Fatalf("expected volume name in session storage: %+v", session.Storage)
	}
	if _, err := os.Stat(filepath.Join(session.Storage.Workspace, ".git")); err != nil {
		t.Fatalf("managed workspace not created: %v", err)
	}
	gitRemoveWorktree(t, repo, session.Storage.Workspace)
}

func TestCreateSessionBranchErrorsOnExistingParentBranchNamespace(t *testing.T) {
	repo := initGitSessionTestRepo(t)
	out, err := exec.Command("git", "-C", repo, "checkout", "-b", "js/features/spnext/reporting").CombinedOutput()
	if err != nil {
		t.Fatalf("git checkout -b parent: %v\n%s", err, out)
	}
	_, err = CreateSessionBranch(GitSessionRequest{
		WorkspaceID: "demo",
		SourceDir:   repo,
		Mode:        "managed",
		BranchName:  "js/features/spnext/reporting/worktree-001",
		SessionID:   "report-poc",
	})
	if err == nil {
		t.Fatal("expected namespace collision error")
	}
	if !strings.Contains(err.Error(), `conflicts with existing branch "js/features/spnext/reporting"`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCreateSessionBranchErrorsOnExistingChildBranchNamespace(t *testing.T) {
	repo := initGitSessionTestRepo(t)
	out, err := exec.Command("git", "-C", repo, "checkout", "-b", "js/features/spnext/reporting/worktree-001").CombinedOutput()
	if err != nil {
		t.Fatalf("git checkout -b child: %v\n%s", err, out)
	}
	_, err = CreateSessionBranch(GitSessionRequest{
		WorkspaceID: "demo",
		SourceDir:   repo,
		Mode:        "managed",
		BranchName:  "js/features/spnext/reporting",
		SessionID:   "report-poc",
	})
	if err == nil {
		t.Fatal("expected namespace collision error")
	}
	if !strings.Contains(err.Error(), `conflicts with existing nested branch "js/features/spnext/reporting/worktree-001"`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGitSessionLifecycleSyncPublishArchiveAndLease(t *testing.T) {
	repo := initGitSessionTestRepo(t)
	git := func(dir string, args ...string) {
		t.Helper()
		out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput()
		if err != nil {
			t.Fatalf("git -C %s %v: %v\n%s", dir, args, err, out)
		}
	}
	base := strings.TrimSpace(mustGitOutput(t, repo, "branch", "--show-current"))
	remote := filepath.Join(t.TempDir(), "remote.git")
	out, err := exec.Command("git", "init", "--bare", remote).CombinedOutput()
	if err != nil {
		t.Fatalf("git init --bare: %v\n%s", err, out)
	}
	git(repo, "remote", "add", "origin", remote)
	git(repo, "push", "-u", "origin", base)

	session, err := CreateSessionBranch(GitSessionRequest{
		WorkspaceID:  "demo",
		SourceDir:    repo,
		Mode:         "managed",
		BaseRef:      base,
		BranchPrefix: "ai",
		SessionID:    "lifecycle",
	})
	if err != nil {
		t.Fatalf("CreateSessionBranch: %v", err)
	}
	if err := os.WriteFile(filepath.Join(session.Storage.Workspace, "generated.txt"), []byte("generated\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := CheckpointSession(session, "before publish"); err != nil {
		t.Fatalf("CheckpointSession: %v", err)
	}
	pub, err := PublishSession(session, "origin")
	if err != nil {
		t.Fatalf("PublishSession: %v result=%+v", err, pub)
	}
	if pub.Status != "published" {
		t.Fatalf("publish status = %q", pub.Status)
	}
	out, err = exec.Command("git", "--git-dir", remote, "rev-parse", "--verify", "refs/heads/"+session.Repo.SessionRef).CombinedOutput()
	if err != nil {
		t.Fatalf("published branch missing: %v\n%s", err, out)
	}

	if err := os.WriteFile(filepath.Join(repo, "base.txt"), []byte("base\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git(repo, "add", "base.txt")
	git(repo, "commit", "-m", "base update")
	syncRes, err := SyncSession(session)
	if err != nil {
		t.Fatalf("SyncSession: %v result=%+v", err, syncRes)
	}
	if syncRes.Status != "synced" {
		t.Fatalf("sync status = %q", syncRes.Status)
	}
	if _, err := os.Stat(filepath.Join(session.Storage.Workspace, "base.txt")); err != nil {
		t.Fatalf("synced base file missing: %v", err)
	}

	lease, err := CreateWorkerLease(session, GitWorkerLeaseRequest{WorkerID: "validator", Role: "validation", Branch: true, TTLSeconds: 60})
	if err != nil {
		t.Fatalf("CreateWorkerLease: %v", err)
	}
	if lease.Branch == "" || lease.Status != "active" {
		t.Fatalf("unexpected lease: %+v", lease)
	}
	released, err := ReleaseWorkerLease(session, "validator", "released")
	if err != nil {
		t.Fatalf("ReleaseWorkerLease: %v", err)
	}
	if released.Status != "released" || released.ReleasedAt == "" {
		t.Fatalf("unexpected released lease: %+v", released)
	}

	if err := ArchiveSession(session); err != nil {
		t.Fatalf("ArchiveSession: %v", err)
	}
	if session.Status != "archived" {
		t.Fatalf("session status = %q", session.Status)
	}
	git(repo, "worktree", "remove", "--force", session.Storage.Workspace)
}

func TestListAndLoadGitSessions(t *testing.T) {
	repo := initGitSessionTestRepo(t)
	first, err := CreateSessionBranch(GitSessionRequest{
		WorkspaceID:  "demo",
		SourceDir:    repo,
		Mode:         "managed",
		BranchPrefix: "ai",
		SessionID:    "first-session",
	})
	if err != nil {
		t.Fatalf("CreateSessionBranch first: %v", err)
	}
	second, err := CreateSessionBranch(GitSessionRequest{
		WorkspaceID:  "demo",
		SourceDir:    repo,
		Mode:         "managed",
		BranchPrefix: "ai",
		SessionID:    "second-session",
	})
	if err != nil {
		t.Fatalf("CreateSessionBranch second: %v", err)
	}
	sessions, err := ListGitSessions(repo)
	if err != nil {
		t.Fatalf("ListGitSessions: %v", err)
	}
	if len(sessions) != 2 {
		t.Fatalf("expected 2 sessions, got %d", len(sessions))
	}
	loaded, err := LoadGitSession(repo, "second")
	if err != nil {
		t.Fatalf("LoadGitSession prefix: %v", err)
	}
	if loaded.SessionID != "second-session" {
		t.Fatalf("loaded session = %q", loaded.SessionID)
	}
	fromWorkspace, err := ListGitSessions(second.Storage.Workspace)
	if err != nil {
		t.Fatalf("ListGitSessions from workspace: %v", err)
	}
	if len(fromWorkspace) != 2 {
		t.Fatalf("expected workspace lookup to find 2 sessions, got %d", len(fromWorkspace))
	}
	gitRemoveWorktree(t, repo, first.Storage.Workspace)
	gitRemoveWorktree(t, repo, second.Storage.Workspace)
}

func mustGitOutput(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).Output()
	if err != nil {
		t.Fatalf("git -C %s %v: %v", dir, args, err)
	}
	return string(out)
}

func gitRemoveWorktree(t *testing.T, repo, workspace string) {
	t.Helper()
	out, err := exec.Command("git", "-C", repo, "worktree", "remove", "--force", workspace).CombinedOutput()
	if err != nil {
		t.Fatalf("git worktree remove: %v\n%s", err, out)
	}
}
