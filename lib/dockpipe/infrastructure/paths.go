package infrastructure

import (
	"fmt"
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

// ResolveResolverFilePath returns the path to a resolver KEY=value file (workflow or step resolver name).
// Order: workflow-local wfRoot/resolvers/<name>, then templates/core/resolvers/<name>,
// then legacy templates/run-worktree/resolvers/<name> for older workspaces.
func ResolveResolverFilePath(repoRoot, wfRoot, resolverName string) (string, error) {
	resolverName = strings.TrimSpace(resolverName)
	if resolverName == "" {
		return "", fmt.Errorf("resolver name is empty")
	}
	var candidates []string
	if wfRoot != "" {
		candidates = append(candidates, filepath.Join(wfRoot, "resolvers", resolverName))
	}
	candidates = append(candidates, filepath.Join(repoRoot, "templates", "core", "resolvers", resolverName))
	candidates = append(candidates, filepath.Join(repoRoot, "templates", "run-worktree", "resolvers", resolverName))
	for _, p := range candidates {
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p, nil
		}
	}
	return "", fmt.Errorf("resolver file not found for %q (tried %v); shared resolvers live under templates/core/resolvers/ — upgrade dockpipe, re-extract the bundle (bundled format bump), or run `dockpipe doctor` / delete the bundled cache folder if your install is stale", resolverName, candidates)
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
