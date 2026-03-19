package infrastructure

import (
	"os"
	"path/filepath"
	"strings"
)

// ResolveWorkflowScript resolves run/act path: scripts/* from repo root, else workflow template dir.
// Uses forward slashes so YAML workflow paths match Linux/container expectations on every GOOS.
func ResolveWorkflowScript(rel, workflowRoot, repoRoot string) string {
	var joined string
	if strings.HasPrefix(rel, "scripts/") {
		joined = filepath.Join(repoRoot, rel)
	} else {
		joined = filepath.Join(workflowRoot, rel)
	}
	return filepath.ToSlash(joined)
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
