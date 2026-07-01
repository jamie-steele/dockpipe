package infrastructure

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type GitSessionRequest struct {
	WorkspaceID  string
	SourceDir    string
	Mode         string
	Storage      string
	BaseRef      string
	BranchPrefix string
	BranchName   string
	SessionID    string
	Checkpoint   string
	Publish      string
}

type GitSession struct {
	Schema      int               `json:"schema"`
	SessionID   string            `json:"session_id"`
	WorkspaceID string            `json:"workspace_id"`
	Repo        GitSessionRepo    `json:"repo"`
	Storage     GitSessionStorage `json:"storage"`
	Status      string            `json:"status"`
	CreatedAt   string            `json:"created_at"`
	UpdatedAt   string            `json:"updated_at"`
	Policy      GitSessionPolicy  `json:"policy"`
}

type GitSessionRepo struct {
	LogicalID  string `json:"logical_id"`
	Source     string `json:"source"`
	BaseRef    string `json:"base_ref"`
	SessionRef string `json:"session_ref"`
}

type GitSessionStorage struct {
	Mode      string `json:"mode"`
	Backend   string `json:"backend"`
	Workspace string `json:"workspace"`
	Metadata  string `json:"metadata"`
	Volume    string `json:"volume,omitempty"`
}

type GitSessionPolicy struct {
	Checkpoint    string `json:"checkpoint"`
	Publish       string `json:"publish"`
	AllowAgentGit bool   `json:"allow_agent_git"`
}

type GitCheckpoint struct {
	Schema       int    `json:"schema"`
	CheckpointID string `json:"checkpoint_id"`
	SessionID    string `json:"session_id"`
	Commit       string `json:"commit,omitempty"`
	Reason       string `json:"reason"`
	DirtyBefore  bool   `json:"dirty_before"`
	Status       string `json:"status"`
	CreatedAt    string `json:"created_at"`
}

type GitSyncResult struct {
	Schema    int    `json:"schema"`
	SessionID string `json:"session_id"`
	BaseRef   string `json:"base_ref"`
	Status    string `json:"status"`
	Commit    string `json:"commit,omitempty"`
	Message   string `json:"message,omitempty"`
	CreatedAt string `json:"created_at"`
}

type GitPublishResult struct {
	Schema    int    `json:"schema"`
	SessionID string `json:"session_id"`
	Remote    string `json:"remote"`
	Branch    string `json:"branch"`
	Status    string `json:"status"`
	Message   string `json:"message,omitempty"`
	CreatedAt string `json:"created_at"`
}

type GitWorkerLeaseRequest struct {
	WorkerID   string
	Role       string
	Branch     bool
	TTLSeconds int
}

type GitWorkerLease struct {
	Schema     int    `json:"schema"`
	LeaseID    string `json:"lease_id"`
	SessionID  string `json:"session_id"`
	WorkerID   string `json:"worker_id"`
	Role       string `json:"role,omitempty"`
	Status     string `json:"status"`
	Workspace  string `json:"workspace"`
	Branch     string `json:"branch,omitempty"`
	CreatedAt  string `json:"created_at"`
	UpdatedAt  string `json:"updated_at"`
	ExpiresAt  string `json:"expires_at,omitempty"`
	ReleasedAt string `json:"released_at,omitempty"`
}

func CreateSessionBranch(req GitSessionRequest) (*GitSession, error) {
	sourceDir := strings.TrimSpace(req.SourceDir)
	if sourceDir == "" {
		return nil, fmt.Errorf("git session source dir is empty")
	}
	top, err := GitTopLevel(sourceDir)
	if err != nil {
		return nil, fmt.Errorf("workspace source must be a git work tree: %w", err)
	}
	mode := strings.TrimSpace(req.Mode)
	if mode == "" {
		mode = "managed"
	}
	switch mode {
	case "managed", "bind":
	default:
		return nil, fmt.Errorf("workspace mode must be managed or bind, got %q", mode)
	}
	storage := strings.TrimSpace(req.Storage)
	if storage == "" {
		if mode == "managed" {
			storage = "worktree"
		} else {
			storage = "bind"
		}
	}
	switch storage {
	case "bind", "worktree", "volume", "clone":
	default:
		return nil, fmt.Errorf("workspace storage must be bind, worktree, volume, or clone, got %q", storage)
	}
	if mode == "bind" && storage != "bind" {
		return nil, fmt.Errorf("workspace mode bind requires storage bind")
	}
	if mode == "managed" && storage == "bind" {
		return nil, fmt.Errorf("workspace mode managed cannot use storage bind")
	}
	if storage == "clone" {
		return nil, fmt.Errorf("workspace storage clone is reserved for distributed runtime support")
	}
	workspaceID := sanitizeSessionSegment(firstNonEmptyString(req.WorkspaceID, filepath.Base(top)))
	sessionID := sanitizeSessionID(req.SessionID)
	if sessionID == "" {
		sessionID = "run-" + time.Now().UTC().Format("20060102-150405")
	}
	prefix := strings.Trim(strings.TrimSpace(req.BranchPrefix), "/")
	if prefix == "" {
		prefix = "ai"
	}
	branch := prefix + "/session-" + sessionID
	if explicit := strings.Trim(strings.TrimSpace(req.BranchName), "/"); explicit != "" {
		branch = explicit
	}
	baseRef := strings.TrimSpace(req.BaseRef)
	if baseRef == "" {
		baseRef = "HEAD"
	}
	if err := validateSessionBranchNamespace(top, branch); err != nil {
		return nil, err
	}
	if mode == "managed" {
		if existing, err := findReusableManagedGitSession(top, branch); err != nil {
			return nil, err
		} else if existing != nil {
			return existing, nil
		}
	}

	sessionDir, err := GitSessionRoot(top, sessionID)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(sessionDir, 0o755); err != nil {
		return nil, err
	}

	workspace := top
	backend := "bind"
	volumeName := ""
	if mode == "managed" {
		backend = "worktree"
		workspace = filepath.Join(sessionDir, "workspace")
		if _, err := os.Stat(workspace); os.IsNotExist(err) {
			if err := ensureManagedSessionWorktree(top, workspace, branch, baseRef); err != nil {
				return nil, err
			}
		}
		if storage == "volume" {
			backend = "docker_volume"
			volumeName = dockerWorkspaceVolumeName(workspaceID, sessionID)
			if err := DockerVolumeCreate(volumeName); err != nil {
				return nil, fmt.Errorf("create workspace docker volume %q: %w", volumeName, err)
			}
		}
	} else {
		if err := ensureBindSessionBranch(top, branch, baseRef); err != nil {
			return nil, err
		}
	}

	now := time.Now().UTC().Format(time.RFC3339)
	remote, _ := GitRemoteGetURL(top, "origin")
	if strings.TrimSpace(remote) == "" {
		remote = top
	}
	s := &GitSession{
		Schema:      1,
		SessionID:   sessionID,
		WorkspaceID: workspaceID,
		Repo: GitSessionRepo{
			LogicalID:  workspaceID,
			Source:     remote,
			BaseRef:    baseRef,
			SessionRef: branch,
		},
		Storage: GitSessionStorage{
			Mode:      mode,
			Backend:   backend,
			Workspace: workspace,
			Metadata:  sessionDir,
			Volume:    volumeName,
		},
		Status:    "active",
		CreatedAt: now,
		UpdatedAt: now,
		Policy: GitSessionPolicy{
			Checkpoint:    firstNonEmptyString(req.Checkpoint, "auto"),
			Publish:       firstNonEmptyString(req.Publish, "review"),
			AllowAgentGit: false,
		},
	}
	if err := writeGitSession(s, top); err != nil {
		return nil, err
	}
	_ = appendGitSessionEvent(s, map[string]string{
		"type":        "session.created",
		"actor":       "runtime",
		"session_id":  sessionID,
		"workspace":   workspace,
		"session_ref": branch,
	})
	return s, nil
}

func ensureManagedSessionWorktree(repo, workspace, branch, baseRef string) error {
	if gitRun(repo, "show-ref", "--verify", "--quiet", "refs/heads/"+branch) == nil {
		return gitRun(repo, "worktree", "add", workspace, branch)
	}
	return gitRun(repo, "worktree", "add", "-b", branch, workspace, baseRef)
}

func findReusableManagedGitSession(repo, branch string) (*GitSession, error) {
	sessions, err := ListGitSessions(repo)
	if err != nil {
		return nil, err
	}
	for _, session := range sessions {
		if session == nil {
			continue
		}
		if strings.TrimSpace(session.Storage.Mode) != "managed" {
			continue
		}
		if strings.TrimSpace(session.Repo.SessionRef) != branch {
			continue
		}
		workspace := strings.TrimSpace(session.Storage.Workspace)
		if workspace == "" {
			continue
		}
		if st, err := os.Stat(workspace); err != nil || !st.IsDir() {
			continue
		}
		if got := strings.TrimSpace(mustGitBranchNameOrEmpty(workspace)); got != branch {
			continue
		}
		return session, nil
	}
	return nil, nil
}

func mustGitBranchNameOrEmpty(workdir string) string {
	out, err := gitOutput(workdir, "branch", "--show-current")
	if err != nil {
		return ""
	}
	return out
}

func CheckpointSession(session *GitSession, reason string) (*GitCheckpoint, error) {
	if session == nil {
		return nil, fmt.Errorf("session is nil")
	}
	workspace := strings.TrimSpace(session.Storage.Workspace)
	if workspace == "" {
		return nil, fmt.Errorf("session workspace is empty")
	}
	top, err := GitTopLevel(workspace)
	if err != nil {
		return nil, err
	}
	status, err := gitOutput(top, "status", "--porcelain")
	if err != nil {
		return nil, err
	}
	dirty := strings.TrimSpace(status) != ""
	cp := &GitCheckpoint{
		Schema:       1,
		CheckpointID: "cp-" + time.Now().UTC().Format("20060102-150405"),
		SessionID:    session.SessionID,
		Reason:       firstNonEmptyString(reason, "runtime checkpoint"),
		DirtyBefore:  dirty,
		Status:       "skipped_clean",
		CreatedAt:    time.Now().UTC().Format(time.RFC3339),
	}
	if dirty {
		if err := gitRun(top, "add", "-A"); err != nil {
			return nil, err
		}
		msg := fmt.Sprintf("checkpoint(runtime): %s\n\nDockPipe-Session: %s\nDockPipe-Checkpoint: %s\nDockPipe-Reason: runtime\n", cp.Reason, session.SessionID, cp.CheckpointID)
		if err := gitRun(top, "commit", "-m", msg); err != nil {
			return nil, err
		}
		commit, err := GitRevParse(top, "HEAD")
		if err != nil {
			return nil, err
		}
		cp.Commit = commit
		cp.Status = "created"
	}
	if err := writeGitCheckpoint(session, cp); err != nil {
		return nil, err
	}
	_ = appendGitSessionEvent(session, map[string]string{
		"type":          "checkpoint." + cp.Status,
		"actor":         "runtime",
		"session_id":    session.SessionID,
		"checkpoint_id": cp.CheckpointID,
		"commit":        cp.Commit,
	})
	return cp, nil
}

func SyncSession(session *GitSession) (*GitSyncResult, error) {
	if session == nil {
		return nil, fmt.Errorf("session is nil")
	}
	top, err := sessionGitTop(session)
	if err != nil {
		return nil, err
	}
	baseRef := strings.TrimSpace(session.Repo.BaseRef)
	if baseRef == "" {
		baseRef = "HEAD"
	}
	res := &GitSyncResult{
		Schema:    1,
		SessionID: session.SessionID,
		BaseRef:   baseRef,
		Status:    "synced",
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	}
	_ = gitRun(top, "fetch", "--all", "--prune")
	if _, err := CheckpointSession(session, "pre-sync checkpoint"); err != nil {
		return nil, err
	}
	out, mergeErr := gitCombined(top, "merge", "--no-edit", baseRef)
	if mergeErr != nil {
		session.Status = "conflict"
		session.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
		_ = writeGitSession(session, top)
		res.Status = "conflict"
		res.Message = strings.TrimSpace(out)
		_ = writeGitSyncResult(session, res)
		_ = appendGitSessionEvent(session, map[string]string{
			"type":       "session.sync.conflict",
			"actor":      "runtime",
			"session_id": session.SessionID,
			"base_ref":   baseRef,
		})
		return res, mergeErr
	}
	commit, _ := GitRevParse(top, "HEAD")
	res.Commit = commit
	session.Status = "active"
	session.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	if err := writeGitSession(session, top); err != nil {
		return nil, err
	}
	if err := writeGitSyncResult(session, res); err != nil {
		return nil, err
	}
	_ = appendGitSessionEvent(session, map[string]string{
		"type":       "session.synced",
		"actor":      "runtime",
		"session_id": session.SessionID,
		"base_ref":   baseRef,
		"commit":     commit,
	})
	return res, nil
}

func PublishSession(session *GitSession, remote string) (*GitPublishResult, error) {
	if session == nil {
		return nil, fmt.Errorf("session is nil")
	}
	top, err := sessionGitTop(session)
	if err != nil {
		return nil, err
	}
	remote = firstNonEmptyString(remote, "origin")
	branch := strings.TrimSpace(session.Repo.SessionRef)
	if branch == "" {
		return nil, fmt.Errorf("session branch is empty")
	}
	res := &GitPublishResult{
		Schema:    1,
		SessionID: session.SessionID,
		Remote:    remote,
		Branch:    branch,
		Status:    "published",
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	}
	out, pushErr := gitCombined(top, "push", "-u", remote, branch)
	if pushErr != nil {
		res.Status = "failed"
		res.Message = strings.TrimSpace(out)
		_ = writeGitPublishResult(session, res)
		_ = appendGitSessionEvent(session, map[string]string{
			"type":       "session.publish.failed",
			"actor":      "runtime",
			"session_id": session.SessionID,
			"remote":     remote,
			"branch":     branch,
		})
		return res, pushErr
	}
	session.Status = "published"
	session.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	if err := writeGitSession(session, top); err != nil {
		return nil, err
	}
	if err := writeGitPublishResult(session, res); err != nil {
		return nil, err
	}
	_ = appendGitSessionEvent(session, map[string]string{
		"type":       "session.published",
		"actor":      "runtime",
		"session_id": session.SessionID,
		"remote":     remote,
		"branch":     branch,
	})
	return res, nil
}

func ArchiveSession(session *GitSession) error {
	if session == nil {
		return fmt.Errorf("session is nil")
	}
	top, err := sessionGitTop(session)
	if err != nil {
		return err
	}
	session.Status = "archived"
	session.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	if err := writeGitSession(session, top); err != nil {
		return err
	}
	return appendGitSessionEvent(session, map[string]string{
		"type":       "session.archived",
		"actor":      "runtime",
		"session_id": session.SessionID,
	})
}

func CreateWorkerLease(session *GitSession, req GitWorkerLeaseRequest) (*GitWorkerLease, error) {
	if session == nil {
		return nil, fmt.Errorf("session is nil")
	}
	top, err := sessionGitTop(session)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(req.WorkerID) == "" {
		return nil, fmt.Errorf("worker id is empty")
	}
	workerID := sanitizeSessionSegment(req.WorkerID)
	now := time.Now().UTC()
	lease := &GitWorkerLease{
		Schema:    1,
		LeaseID:   "lease-" + now.Format("20060102-150405") + "-" + workerID,
		SessionID: session.SessionID,
		WorkerID:  workerID,
		Role:      strings.TrimSpace(req.Role),
		Status:    "active",
		Workspace: session.Storage.Workspace,
		CreatedAt: now.Format(time.RFC3339),
		UpdatedAt: now.Format(time.RFC3339),
	}
	if req.TTLSeconds > 0 {
		lease.ExpiresAt = now.Add(time.Duration(req.TTLSeconds) * time.Second).Format(time.RFC3339)
	}
	if req.Branch {
		lease.Branch = strings.Trim(strings.TrimSpace(session.Repo.SessionRef), "/") + "-worker-" + workerID
		if err := gitRun(top, "branch", lease.Branch, "HEAD"); err != nil {
			return nil, err
		}
	}
	if err := writeGitWorkerLease(session, lease); err != nil {
		return nil, err
	}
	_ = appendGitSessionEvent(session, map[string]string{
		"type":       "worker.lease.created",
		"actor":      "runtime",
		"session_id": session.SessionID,
		"worker_id":  workerID,
		"lease_id":   lease.LeaseID,
		"branch":     lease.Branch,
	})
	return lease, nil
}

func ReleaseWorkerLease(session *GitSession, workerID, status string) (*GitWorkerLease, error) {
	if session == nil {
		return nil, fmt.Errorf("session is nil")
	}
	workerID = sanitizeSessionSegment(workerID)
	dir, err := gitSessionMetadataDir(session, session.Storage.Workspace)
	if err != nil {
		return nil, err
	}
	path := filepath.Join(dir, "workers", workerID+".json")
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var lease GitWorkerLease
	if err := json.Unmarshal(b, &lease); err != nil {
		return nil, err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	lease.Status = firstNonEmptyString(status, "released")
	lease.UpdatedAt = now
	lease.ReleasedAt = now
	if err := writeGitWorkerLease(session, &lease); err != nil {
		return nil, err
	}
	_ = appendGitSessionEvent(session, map[string]string{
		"type":       "worker.lease." + lease.Status,
		"actor":      "runtime",
		"session_id": session.SessionID,
		"worker_id":  lease.WorkerID,
		"lease_id":   lease.LeaseID,
	})
	return &lease, nil
}

func ListGitSessions(workdir string) ([]*GitSession, error) {
	roots, err := gitSessionRootsForWorkdir(workdir)
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	sessions := []*GitSession{}
	for _, root := range roots {
		entries, err := os.ReadDir(root)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return nil, err
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			path := filepath.Join(root, entry.Name(), "session.json")
			session, err := ReadGitSessionFile(path)
			if err != nil {
				continue
			}
			key := filepath.Clean(session.Storage.Metadata)
			if key == "" {
				key = session.SessionID
			}
			if seen[key] {
				continue
			}
			seen[key] = true
			sessions = append(sessions, session)
		}
	}
	sort.SliceStable(sessions, func(i, j int) bool {
		return sessionSortTime(sessions[i]).After(sessionSortTime(sessions[j]))
	})
	return sessions, nil
}

func LoadGitSession(workdir, selector string) (*GitSession, error) {
	selector = strings.TrimSpace(selector)
	if selector == "" {
		return nil, fmt.Errorf("session id is empty")
	}
	sessions, err := ListGitSessions(workdir)
	if err != nil {
		return nil, err
	}
	if len(sessions) == 0 {
		return nil, fmt.Errorf("no sessions found under %s", workdir)
	}
	if selector == "latest" {
		return sessions[0], nil
	}
	var matches []*GitSession
	for _, session := range sessions {
		if session == nil {
			continue
		}
		switch {
		case session.SessionID == selector:
			return session, nil
		case strings.HasPrefix(session.SessionID, selector):
			matches = append(matches, session)
		case session.Repo.SessionRef == selector:
			return session, nil
		case strings.HasSuffix(session.Repo.SessionRef, "/"+selector):
			matches = append(matches, session)
		}
	}
	if len(matches) == 1 {
		return matches[0], nil
	}
	if len(matches) > 1 {
		var ids []string
		for _, match := range matches {
			ids = append(ids, match.SessionID)
		}
		return nil, fmt.Errorf("session selector %q is ambiguous: %s", selector, strings.Join(ids, ", "))
	}
	return nil, fmt.Errorf("session %q not found", selector)
}

func ReadGitSessionFile(path string) (*GitSession, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var session GitSession
	if err := json.Unmarshal(b, &session); err != nil {
		return nil, err
	}
	if strings.TrimSpace(session.Storage.Metadata) == "" {
		session.Storage.Metadata = filepath.Dir(path)
	}
	return &session, nil
}

func GitSessionRoot(workdir, sessionID string) (string, error) {
	root, err := StateRoot(workdir)
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "sessions", sanitizeSessionID(sessionID)), nil
}

func GitSessionsRoot(workdir string) (string, error) {
	root, err := StateRoot(workdir)
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "sessions"), nil
}

func gitSessionRootsForWorkdir(workdir string) ([]string, error) {
	if strings.TrimSpace(workdir) == "" {
		wd, err := os.Getwd()
		if err != nil {
			return nil, err
		}
		workdir = wd
	}
	workdir = HostPathForGit(workdir)
	abs, err := filepath.Abs(workdir)
	if err != nil {
		return nil, err
	}
	var roots []string
	addRoot := func(root string) {
		root = filepath.Clean(root)
		for _, existing := range roots {
			if existing == root {
				return
			}
		}
		roots = append(roots, root)
	}
	if root, err := GitSessionsRoot(abs); err == nil {
		addRoot(root)
	}
	if top, err := GitTopLevel(abs); err == nil {
		if root, err := GitSessionsRoot(top); err == nil {
			addRoot(root)
		}
	}
	for dir := abs; ; dir = filepath.Dir(dir) {
		if filepath.Base(dir) == "workspace" {
			sessionDir := filepath.Dir(dir)
			sessionsRoot := filepath.Dir(sessionDir)
			if filepath.Base(sessionsRoot) == "sessions" {
				addRoot(sessionsRoot)
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
	}
	return roots, nil
}

func sessionSortTime(session *GitSession) time.Time {
	if session == nil {
		return time.Time{}
	}
	for _, raw := range []string{session.UpdatedAt, session.CreatedAt} {
		if t, err := time.Parse(time.RFC3339, strings.TrimSpace(raw)); err == nil {
			return t
		}
	}
	return time.Time{}
}

func DockerVolumeCreate(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("docker volume name is empty")
	}
	dockerCmd := dockerCommandName()
	cmd := execCommandFn(dockerCmd, "volume", "create", name)
	cmd.Env = dockerCommandEnv(os.Environ(), dockerCmd)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("docker volume create %s: %w\n%s", name, err, out)
	}
	return nil
}

func ensureBindSessionBranch(workdir, branch, baseRef string) error {
	current, _ := GitBranchShowCurrent(workdir)
	if current == branch {
		return nil
	}
	if err := validateSessionBranchNamespace(workdir, branch); err != nil {
		return err
	}
	if gitRun(workdir, "show-ref", "--verify", "--quiet", "refs/heads/"+branch) == nil {
		return gitRun(workdir, "checkout", branch)
	}
	return gitRun(workdir, "checkout", "-b", branch, baseRef)
}

func validateSessionBranchNamespace(workdir, branch string) error {
	branch = strings.Trim(strings.TrimSpace(branch), "/")
	if branch == "" {
		return fmt.Errorf("session branch is empty")
	}
	if gitRun(workdir, "show-ref", "--verify", "--quiet", "refs/heads/"+branch) == nil {
		return nil
	}
	parts := strings.Split(branch, "/")
	for i := 1; i < len(parts); i++ {
		parent := strings.Join(parts[:i], "/")
		if gitRun(workdir, "show-ref", "--verify", "--quiet", "refs/heads/"+parent) == nil {
			return fmt.Errorf("session branch %q conflicts with existing branch %q; Git cannot create nested refs under an existing branch name. Use a sibling name like %q or set workspace.lifecycle.branch_prefix instead", branch, parent, parent+"-"+parts[i])
		}
	}
	out, err := gitOutput(workdir, "for-each-ref", "--format=%(refname:strip=2)", "refs/heads/"+branch+"/*")
	if err != nil {
		return err
	}
	if child := strings.TrimSpace(out); child != "" {
		first := strings.Split(child, "\n")[0]
		return fmt.Errorf("session branch %q conflicts with existing nested branch %q; Git cannot create a parent branch when child refs already exist. Use a different session branch name", branch, strings.TrimSpace(first))
	}
	return nil
}

func writeGitSession(session *GitSession, workdir string) error {
	dir, err := gitSessionMetadataDir(session, workdir)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(dir, "checkpoints"), 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(dir, "workers"), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "session.json"), append(b, '\n'), 0o644)
}

func writeGitSyncResult(session *GitSession, res *GitSyncResult) error {
	dir, err := gitSessionMetadataDir(session, session.Storage.Workspace)
	if err != nil {
		return err
	}
	b, err := json.MarshalIndent(res, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "last-sync.json"), append(b, '\n'), 0o644)
}

func writeGitPublishResult(session *GitSession, res *GitPublishResult) error {
	dir, err := gitSessionMetadataDir(session, session.Storage.Workspace)
	if err != nil {
		return err
	}
	b, err := json.MarshalIndent(res, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "publish.json"), append(b, '\n'), 0o644)
}

func writeGitWorkerLease(session *GitSession, lease *GitWorkerLease) error {
	dir, err := gitSessionMetadataDir(session, session.Storage.Workspace)
	if err != nil {
		return err
	}
	workerDir := filepath.Join(dir, "workers")
	if err := os.MkdirAll(workerDir, 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(lease, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(workerDir, lease.WorkerID+".json"), append(b, '\n'), 0o644)
}

func writeGitCheckpoint(session *GitSession, cp *GitCheckpoint) error {
	dir, err := gitSessionMetadataDir(session, session.Storage.Workspace)
	if err != nil {
		return err
	}
	cpDir := filepath.Join(dir, "checkpoints")
	if err := os.MkdirAll(cpDir, 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(cp, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(cpDir, cp.CheckpointID+".json"), append(b, '\n'), 0o644)
}

func appendGitSessionEvent(session *GitSession, fields map[string]string) error {
	dir, err := gitSessionMetadataDir(session, session.Storage.Workspace)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	ev := map[string]string{"ts": time.Now().UTC().Format(time.RFC3339)}
	for k, v := range fields {
		if strings.TrimSpace(v) != "" {
			ev[k] = v
		}
	}
	b, err := json.Marshal(ev)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(filepath.Join(dir, "events.jsonl"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(append(b, '\n'))
	return err
}

func gitSessionMetadataDir(session *GitSession, fallbackWorkdir string) (string, error) {
	if session != nil && strings.TrimSpace(session.Storage.Metadata) != "" {
		return filepath.Clean(session.Storage.Metadata), nil
	}
	sessionID := ""
	if session != nil {
		sessionID = session.SessionID
	}
	return GitSessionRoot(fallbackWorkdir, sessionID)
}

func sessionGitTop(session *GitSession) (string, error) {
	workspace := strings.TrimSpace(session.Storage.Workspace)
	if workspace == "" {
		return "", fmt.Errorf("session workspace is empty")
	}
	return GitTopLevel(workspace)
}

func dockerWorkspaceVolumeName(workspaceID, sessionID string) string {
	return "dockpipe-ws-" + sanitizeSessionSegment(workspaceID) + "-" + sanitizeSessionID(sessionID)
}

func gitRun(workdir string, args ...string) error {
	_, err := gitCombined(workdir, args...)
	return err
}

func gitOutput(workdir string, args ...string) (string, error) {
	d, err := gitDir(workdir)
	if err != nil {
		return "", err
	}
	cmd := exec.Command("git", append([]string{"-C", d}, args...)...)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return string(out), nil
}

func gitCombined(workdir string, args ...string) (string, error) {
	d, err := gitDir(workdir)
	if err != nil {
		return "", err
	}
	cmd := exec.Command("git", append([]string{"-C", d}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("git %s: %w\n%s", strings.Join(args, " "), err, out)
	}
	return string(out), nil
}

func sanitizeSessionID(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	parts := strings.FieldsFunc(s, func(r rune) bool {
		return !(r >= 'a' && r <= 'z') && !(r >= 'A' && r <= 'Z') && !(r >= '0' && r <= '9') && r != '-' && r != '_' && r != '.'
	})
	return strings.Trim(strings.ToLower(strings.Join(parts, "-")), "-_.")
}

func sanitizeSessionSegment(s string) string {
	if out := sanitizeSessionID(s); out != "" {
		return out
	}
	return "workspace"
}

func firstNonEmptyString(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}
