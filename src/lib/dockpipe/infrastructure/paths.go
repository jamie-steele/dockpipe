package infrastructure

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ResolveWorkflowScript resolves run/act path: scripts/* from the project (repoRoot/scripts/ if
// present, else repoRoot/src/scripts/ for this dockpipe repo layout), else .staging/resolvers|bundles
// (dockpipe checkout) or templates/core (materialized merge), then templates/core/assets/scripts/;
// other paths are relative to the workflow template dir.
//
// Namespace: scripts/core.<dot.segments> maps to paths under core/ by turning dots into path segments
// (e.g. scripts/core.assets.scripts.foo.sh → core/assets/scripts/foo.sh). Resolution order for that path:
// .dockpipe/core/…, .dockpipe/internal/packages/core/…, then templates/core/… (compiled / materialized layout).
// Uses forward slashes so YAML paths match Linux/container expectations.
func ResolveWorkflowScript(rel, workflowRoot, repoRoot string) string {
	if strings.HasPrefix(rel, "scripts/") {
		return filepath.ToSlash(resolveScriptsPrefixedPath(repoRoot, rel))
	}
	return filepath.ToSlash(filepath.Join(workflowRoot, rel))
}

func scriptFileExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}

// tryBundledAssetsScripts maps rest "domain/tail/path" to <base>/<top>/domain/assets/scripts/tail/path.
func tryBundledAssetsScripts(base, top string, rest string) (string, bool) {
	if !strings.Contains(rest, "/") {
		return "", false
	}
	first, after, ok := strings.Cut(rest, "/")
	if !ok || after == "" {
		return "", false
	}
	p := filepath.Join(base, top, first, "assets", "scripts", after)
	if scriptFileExists(p) {
		return p, true
	}
	return "", false
}

// relFromCoreNamespace parses scripts/ paths after "core." — segments separated by dots become path segments.
// Example: "core.assets.scripts.docker-cache-volumes.sh" → "assets/scripts/docker-cache-volumes.sh".
func relFromCoreNamespace(rest string) (string, bool) {
	if !strings.HasPrefix(rest, "core.") {
		return "", false
	}
	inner := strings.TrimPrefix(rest, "core.")
	if inner == "" {
		return "", false
	}
	parts := strings.Split(inner, ".")
	return filepath.Join(parts...), true
}

// resolveCoreNamespacedAsset resolves scripts/core.* to a file under core/ (compiled overlays first).
func resolveCoreNamespacedAsset(repoRoot, rest string) (string, bool) {
	rel, ok := relFromCoreNamespace(rest)
	if !ok {
		return "", false
	}
	candidates := []string{
		filepath.Join(repoRoot, ".dockpipe", "core", rel),
		filepath.Join(repoRoot, ".dockpipe", "internal", "packages", "core", rel),
		filepath.Join(CoreDir(repoRoot), rel),
	}
	for _, p := range candidates {
		if scriptFileExists(p) {
			return p, true
		}
	}
	return "", false
}

func resolveScriptsPrefixedPath(repoRoot, rel string) string {
	rest := strings.TrimPrefix(rel, "scripts/")
	user := filepath.Join(repoRoot, "scripts", rest)
	if scriptFileExists(user) {
		return user
	}
	srcScripts := filepath.Join(repoRoot, "src", "scripts", rest)
	if scriptFileExists(srcScripts) {
		return srcScripts
	}
	if p, ok := resolveCoreNamespacedAsset(repoRoot, rest); ok {
		return p
	}
	core := CoreDir(repoRoot)
	if UsesBundledAssetLayout(repoRoot) {
		if p, ok := tryBundledAssetsScripts(core, "resolvers", rest); ok {
			return p
		}
		if p, ok := tryBundledAssetsScripts(core, "bundles", rest); ok {
			return p
		}
		resolverPath := filepath.Join(core, "resolvers", rest)
		if scriptFileExists(resolverPath) {
			return resolverPath
		}
		bundlePath := filepath.Join(core, "bundles", rest)
		if scriptFileExists(bundlePath) {
			return bundlePath
		}
		return filepath.Join(core, "assets", "scripts", rest)
	}
	stagingRoot := filepath.Join(repoRoot, ".staging")
	if p, ok := tryBundledAssetsScripts(stagingRoot, "resolvers", rest); ok {
		return p
	}
	if p, ok := tryBundledAssetsScripts(stagingRoot, "bundles", rest); ok {
		return p
	}
	if p, ok := tryBundledAssetsScripts(core, "resolvers", rest); ok {
		return p
	}
	resolverPath := filepath.Join(core, "resolvers", rest)
	if scriptFileExists(resolverPath) {
		return resolverPath
	}
	stagingResolverPath := filepath.Join(StagingResolversDir(repoRoot), rest)
	if scriptFileExists(stagingResolverPath) {
		return stagingResolverPath
	}
	stagingBundlePath := filepath.Join(StagingBundlesDir(repoRoot), rest)
	if scriptFileExists(stagingBundlePath) {
		return stagingBundlePath
	}
	return filepath.Join(core, "assets", "scripts", rest)
}

// ResolveActionPath resolves act script like bin/dockpipe.
func ResolveActionPath(action, repoRoot, cwd string) (string, error) {
	if action == "" {
		return "", nil
	}
	if filepath.IsAbs(action) {
		return action, nil
	}
	candidates := []string{filepath.Join(repoRoot, action)}
	if strings.HasPrefix(action, "scripts/") {
		p := resolveScriptsPrefixedPath(repoRoot, action)
		if scriptFileExists(p) {
			return filepath.Abs(p)
		}
	} else {
		candidates = append(candidates, filepath.Join(repoRoot, "scripts", action))
		candidates = append(candidates, filepath.Join(repoRoot, "src", "scripts", action))
	}
	candidates = append(candidates, filepath.Join(cwd, action))
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
	if strings.HasPrefix(p, "scripts/") {
		return resolveScriptsPrefixedPath(repoRoot, p)
	}
	c := filepath.Join(repoRoot, p)
	if _, err := os.Stat(c); err == nil {
		return c
	}
	return p
}

// ResolveResolverFilePath returns the path to a specific resolver profile (KEY=value) by name.
// Search order (dockpipe checkout): .staging/resolvers → templates/core/resolvers. Materialized bundle: shipyard/core/resolvers only.
func ResolveResolverFilePath(repoRoot, resolverName string) (string, error) {
	resolverName = strings.TrimSpace(resolverName)
	if resolverName == "" {
		return "", fmt.Errorf("resolver profile name is empty")
	}
	var candidates []string
	if !UsesBundledAssetLayout(repoRoot) {
		candidates = append(candidates,
			filepath.Join(StagingResolversDir(repoRoot), resolverName),
			filepath.Join(StagingResolversDir(repoRoot), resolverName, "profile"),
		)
	}
	candidates = append(candidates,
		filepath.Join(CoreDir(repoRoot), "resolvers", resolverName),
		filepath.Join(CoreDir(repoRoot), "resolvers", resolverName, "profile"),
	)
	for _, p := range candidates {
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p, nil
		}
	}
	return "", fmt.Errorf("resolver profile not found for %q (tried %v); shared profiles live under .staging/resolvers/ (dockpipe repo) or templates/core/resolvers/ — use workflow YAML for custom flows — upgrade dockpipe or run `dockpipe doctor` if your install is stale", resolverName, candidates)
}

// IsBundledCommitWorktree reports whether action is the bundled commit-worktree.sh.
func IsBundledCommitWorktree(actionPath, repoRoot string) bool {
	a, err := filepath.Abs(actionPath)
	if err != nil {
		return false
	}
	for _, b := range []string{
		filepath.Join(repoRoot, "scripts", "commit-worktree.sh"),
		filepath.Join(repoRoot, "src", "scripts", "commit-worktree.sh"),
		filepath.Join(CoreDir(repoRoot), "assets", "scripts", "commit-worktree.sh"),
	} {
		bp, err := filepath.Abs(b)
		if err != nil {
			continue
		}
		if a == bp {
			return true
		}
	}
	return false
}
