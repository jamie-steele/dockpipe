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

// ResolveResolverFilePath returns the path to a specific resolver profile (KEY=value) by name.
// name selects a concrete profile (claude, codex, …); the agnostic runtime contract is DOCKPIPE_RUNTIME_* / DOCKPIPE_RESOLVER_* inside the file.
// Search order:
//
//	templates/core/resolvers/<name> (file) → templates/core/resolvers/<name>/profile
//
// Custom execution graphs belong in workflow YAML under templates/<workflow>/ (or --workflow-file), not beside it as profile files.
func ResolveResolverFilePath(repoRoot, resolverName string) (string, error) {
	resolverName = strings.TrimSpace(resolverName)
	if resolverName == "" {
		return "", fmt.Errorf("resolver profile name is empty")
	}
	candidates := []string{
		filepath.Join(repoRoot, "templates", "core", "resolvers", resolverName),
		filepath.Join(repoRoot, "templates", "core", "resolvers", resolverName, "profile"),
	}
	for _, p := range candidates {
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p, nil
		}
	}
	return "", fmt.Errorf("resolver profile not found for %q (tried %v); shared profiles live under templates/core/resolvers/ — use workflow YAML for custom flows — upgrade dockpipe or run `dockpipe doctor` if your install is stale", resolverName, candidates)
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
