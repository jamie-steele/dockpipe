package infrastructure

import (
	"os"
	"path/filepath"
	"strings"
)

// ResolveWorkflowScript resolves run/act path: scripts/* from repo root, else workflow template dir.
func ResolveWorkflowScript(rel, workflowRoot, repoRoot string) string {
	if strings.HasPrefix(rel, "scripts/") {
		return filepath.Join(repoRoot, rel)
	}
	return filepath.Join(workflowRoot, rel)
}

// ResolveActionPath resolves act script like bin/dockpipe.
func ResolveActionPath(action, repoRoot, cwd string) (string, error) {
	if action == "" {
		return "", nil
	}
	if filepath.IsAbs(action) {
		return action, nil
	}
	candidates := []string{
		filepath.Join(repoRoot, action),
		filepath.Join(repoRoot, "scripts", action),
		filepath.Join(cwd, action),
	}
	for _, c := range candidates {
		if st, err := os.Stat(c); err == nil && !st.IsDir() {
			return filepath.Abs(c)
		}
	}
	return filepath.Abs(filepath.Join(cwd, action))
}

// ResolvePreScriptPath resolves --run path.
func ResolvePreScriptPath(p, repoRoot string) string {
	if filepath.IsAbs(p) {
		return p
	}
	c := filepath.Join(repoRoot, p)
	if _, err := os.Stat(c); err == nil {
		return c
	}
	return p
}

// IsBundledCommitWorktree reports whether action is the bundled commit-worktree.sh.
func IsBundledCommitWorktree(actionPath, repoRoot string) bool {
	b := filepath.Join(repoRoot, "scripts/commit-worktree.sh")
	a, err := filepath.Abs(actionPath)
	if err != nil {
		return false
	}
	b, err = filepath.Abs(b)
	if err != nil {
		return false
	}
	return a == b
}
