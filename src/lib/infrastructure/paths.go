package infrastructure

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"dockpipe/src/lib/domain"
	"dockpipe/src/lib/infrastructure/packagebuild"
)

var errStopScriptWalk = errors.New("stop script walk")

// ResolveWorkflowScript resolves run/act path: scripts/* from the project (projectRoot/scripts/ if
// present, else projectRoot/src/scripts/ for this dockpipe repo layout), else extra workflow roots
// from dockpipe.config.json compile.workflows (nested trees), bundle roots from compile.bundles,
// then templates/core (materialized merge), then templates/core/assets/scripts/;
// other paths are relative to the workflow template dir.
// repoRoot is the layout root (bundled cache or checkout); projectRoot is the DockPipe project
// directory (--workdir or cwd). Pass projectRoot == "" to treat project as repoRoot.
//
// Namespace: scripts/core.<dot.segments> maps to paths under core/ by turning dots into path segments
// (e.g. scripts/core.assets.scripts.foo.sh → core/assets/scripts/foo.sh). Resolution prefers compiled
// core overlays, then any workflow/package asset that provides the same asset path via declared compile
// roots or installed workflow tarballs, then the core spine itself.
// Uses forward slashes so YAML paths match Linux/container expectations.
func ResolveWorkflowScript(rel, workflowRoot, repoRoot, projectRoot string) string {
	if strings.HasPrefix(rel, "scripts/") {
		return filepath.ToSlash(resolveScriptsPrefixedPath(repoRoot, projectRoot, rel))
	}
	return filepath.ToSlash(filepath.Join(workflowRoot, rel))
}

// ResolveCoreNamespacedScriptPath resolves scripts/core.<dot.segments> the same way as workflow YAML
// (see ResolveWorkflowScript). Pass dotted without the scripts/ prefix, e.g.
// "assets.scripts.terraform-pipeline.sh" or "core.assets.scripts.terraform-pipeline.sh".
// Returns an absolute path if the file exists, otherwise an error.
func ResolveCoreNamespacedScriptPath(repoRoot, projectRoot, dotted string) (string, error) {
	dotted = strings.TrimSpace(dotted)
	if dotted == "" {
		return "", fmt.Errorf("empty core script path")
	}
	rest := dotted
	if !strings.HasPrefix(rest, "core.") {
		rest = "core." + rest
	}
	rel := "scripts/" + rest
	p := resolveScriptsPrefixedPath(repoRoot, projectRoot, rel)
	if !scriptFileExists(p) {
		return "", fmt.Errorf("core script not found for %q (resolved %s)", dotted, p)
	}
	abs, err := filepath.Abs(p)
	if err != nil {
		return "", err
	}
	return abs, nil
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

// tryNestedWorkflowScripts maps rest "domain/tail/path" to
// <workflow-compile-root>/**/<domain>/assets/scripts/tail/path (any namespace depth).
func tryNestedWorkflowScripts(rest string, wfRoots []string) (string, bool) {
	if !strings.Contains(rest, "/") {
		return "", false
	}
	first, after, ok := strings.Cut(rest, "/")
	if !ok || after == "" {
		return "", false
	}
	for _, st := range wfRoots {
		var hit string
		err := filepath.WalkDir(st, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				if os.IsNotExist(err) {
					return nil
				}
				return err
			}
			if !d.IsDir() || filepath.Base(path) != first {
				return nil
			}
			p := filepath.Join(path, "assets", "scripts", after)
			if scriptFileExists(p) {
				hit = p
				return errStopScriptWalk
			}
			return nil
		})
		if err != nil && !errors.Is(err, errStopScriptWalk) {
			continue
		}
		if hit != "" {
			return hit, true
		}
	}
	return "", false
}

// relFromCoreNamespace parses scripts/ paths after "core." — segments separated by dots become path segments.
// The final filename may contain dots (e.g. terraform-pipeline.sh): the last segment is treated as an extension
// when it matches commonExtSegment, and is joined with the previous segment as the basename.
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
	if len(parts) < 2 {
		return "", false
	}
	n := len(parts)
	// foo.tar.gz under dotted path
	if n >= 3 && parts[n-2] == "tar" && parts[n-1] == "gz" {
		file := parts[n-3] + ".tar.gz"
		dir := parts[:n-3]
		return filepath.Join(append(dir, file)...), true
	}
	if n >= 2 && commonExtSegment(parts[n-1]) {
		file := parts[n-2] + "." + parts[n-1]
		dir := parts[:n-2]
		return filepath.Join(append(dir, file)...), true
	}
	return filepath.Join(parts...), true
}

func commonExtSegment(s string) bool {
	switch strings.ToLower(s) {
	case "sh", "bash", "zsh", "ksh",
		"py", "pl", "rb", "lua", "r", "jl", "sql",
		"ps1", "psm1", "psd1",
		"js", "mjs", "cjs", "ts",
		"json", "yaml", "yml", "toml", "xml",
		"md", "txt", "cfg", "conf", "hcl", "tf", "tfvars":
		return true
	default:
		return false
	}
}

// workflowAssetPathFromRoots finds the first workflow-owned asset whose path ends with rel under the
// provided roots (authoring trees such as compile.workflows or bundled bundle/workflows).
func workflowAssetPathFromRoots(rel string, roots []string) (string, bool) {
	rel = filepath.ToSlash(filepath.Clean(rel))
	if rel == "." || rel == "" {
		return "", false
	}
	wantSuffix := "/" + rel
	for _, root := range roots {
		var hit string
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				if os.IsNotExist(err) {
					return nil
				}
				return err
			}
			if d.IsDir() {
				return nil
			}
			if strings.HasSuffix(filepath.ToSlash(path), wantSuffix) && scriptFileExists(path) {
				hit = path
				return errStopScriptWalk
			}
			return nil
		})
		if err != nil && !errors.Is(err, errStopScriptWalk) {
			continue
		}
		if hit != "" {
			return hit, true
		}
	}
	return "", false
}

func workflowAssetPathFromWorkflowTarballs(projectRoot, pkgDir, rel string) (string, bool) {
	if projectRoot == "" {
		return "", false
	}
	pattern := filepath.Join(pkgDir, "dockpipe-workflow-*.tar.gz")
	matches, _ := filepath.Glob(pattern)
	for _, tgz := range matches {
		root, err := packagebuild.EnsureTarballExtractedCache(tgz, TarballExtractCacheRoot(projectRoot))
		if err != nil {
			continue
		}
		if p, ok := workflowAssetPathFromRoots(rel, []string{root}); ok {
			return p, true
		}
	}
	return "", false
}

func workflowAssetPathForLogicalScript(rest string, roots []string) (string, bool) {
	if !strings.Contains(rest, "/") {
		return "", false
	}
	_, after, ok := strings.Cut(rest, "/")
	if !ok || after == "" {
		return "", false
	}
	return workflowAssetPathFromRoots(filepath.Join("assets", "scripts", after), roots)
}

// resolveCoreNamespacedAsset resolves scripts/core.* to a file under core/ (compiled overlays first).
// projectRoot is the DockPipe project directory (bin/.dockpipe, packages/); repoRoot is the layout root for CoreDir / bundle.
func resolveCoreNamespacedAsset(repoRoot, projectRoot, rest string) (string, bool) {
	if projectRoot == "" {
		projectRoot = repoRoot
	}
	rel, ok := relFromCoreNamespace(rest)
	if !ok {
		return "", false
	}
	candidates := []string{
		filepath.Join(projectRoot, DockpipeDirRel, "core", rel),
		filepath.Join(projectRoot, DockpipeDirRel, "internal", "packages", "core", rel),
	}
	pkgWf := filepath.Join(projectRoot, DockpipeDirRel, "internal", "packages", "workflows")
	if p, ok := workflowAssetPathFromWorkflowTarballs(projectRoot, pkgWf, rel); ok {
		return p, true
	}
	if gw, err := GlobalPackagesWorkflowsDir(); err == nil {
		if p, ok := workflowAssetPathFromWorkflowTarballs(projectRoot, gw, rel); ok {
			return p, true
		}
	}
	var wfAssetRoots []string
	if st, err := os.Stat(filepath.Join(repoRoot, BundledLayoutDir, "workflows")); err == nil && st.IsDir() {
		wfAssetRoots = append(wfAssetRoots, filepath.Join(repoRoot, BundledLayoutDir, "workflows"))
	}
	wfAssetRoots = append(wfAssetRoots, WorkflowCompileRootsCached(projectRoot)...)
	if p, ok := workflowAssetPathFromRoots(rel, wfAssetRoots); ok {
		return p, true
	}
	if tarPath, err := FindLatestCoreTarball(projectRoot); err == nil && tarPath != "" {
		if root, err := packagebuild.EnsureTarballExtractedCache(tarPath, TarballExtractCacheRoot(projectRoot)); err == nil {
			candidates = append(candidates, filepath.Join(root, "core", rel))
		}
	}
	candidates = append(candidates, filepath.Join(CoreDir(repoRoot), rel))
	if gd, err := GlobalTemplatesCoreDir(); err == nil {
		candidates = append(candidates, filepath.Join(gd, rel))
	}
	for _, p := range candidates {
		if scriptFileExists(p) {
			return p, true
		}
	}
	return "", false
}

// scriptPathFromResolverTarballs resolves paths under compiled resolver tarballs (dockpipe-resolver-*.tar.gz).
// For rest like "dorkpipe/run.sh" it tries resolvers/dorkpipe/run.sh then
// resolvers/dorkpipe/assets/scripts/run.sh (canonical resolver layout).
func scriptPathFromResolverTarballs(projectRoot, pkgDir, rest string) (string, bool) {
	if projectRoot == "" {
		return "", false
	}
	pattern := filepath.Join(pkgDir, "dockpipe-resolver-*.tar.gz")
	matches, _ := filepath.Glob(pattern)
	for _, tgz := range matches {
		root, err := packagebuild.EnsureTarballExtractedCache(tgz, TarballExtractCacheRoot(projectRoot))
		if err != nil {
			continue
		}
		base := filepath.Join(root, "resolvers")
		p := filepath.Join(base, rest)
		if scriptFileExists(p) {
			return p, true
		}
		if strings.Contains(rest, "/") {
			first, after, ok := strings.Cut(rest, "/")
			if ok && after != "" {
				p2 := filepath.Join(base, first, "assets", "scripts", after)
				if scriptFileExists(p2) {
					return p2, true
				}
			}
		}
	}
	return "", false
}

// scriptPathFromWorkflowTarballs resolves scripts/<domain>/tail to workflows/<domain>/assets/scripts/tail
// inside dockpipe-workflow-*.tar.gz (compiled store).
func scriptPathFromWorkflowTarballs(projectRoot, pkgDir, rest string) (string, bool) {
	if projectRoot == "" {
		return "", false
	}
	if !strings.Contains(rest, "/") {
		return "", false
	}
	first, after, ok := strings.Cut(rest, "/")
	if !ok || after == "" {
		return "", false
	}
	rel := filepath.ToSlash(filepath.Join("workflows", first, "assets", "scripts", after))
	pattern := filepath.Join(pkgDir, "dockpipe-workflow-*.tar.gz")
	matches, _ := filepath.Glob(pattern)
	for _, tgz := range matches {
		root, err := packagebuild.EnsureTarballExtractedCache(tgz, TarballExtractCacheRoot(projectRoot))
		if err != nil {
			continue
		}
		p := filepath.Join(root, filepath.FromSlash(rel))
		if scriptFileExists(p) {
			return p, true
		}
	}
	return "", false
}

// tryBundleRootsAssetsScripts maps rest "bundleName/tail/under/assets/scripts" under each bundle compile root.
func tryBundleRootsAssetsScripts(rest string, bundleRoots []string) (string, bool) {
	if !strings.Contains(rest, "/") {
		return "", false
	}
	first, after, ok := strings.Cut(rest, "/")
	if !ok || after == "" {
		return "", false
	}
	for _, br := range bundleRoots {
		p := filepath.Join(br, first, "assets", "scripts", after)
		if scriptFileExists(p) {
			return p, true
		}
	}
	return "", false
}

func resolveScriptsPrefixedPath(repoRoot, projectRoot, rel string) string {
	if projectRoot == "" {
		projectRoot = repoRoot
	}
	rest := strings.TrimPrefix(rel, "scripts/")
	user := filepath.Join(projectRoot, "scripts", rest)
	if scriptFileExists(user) {
		return user
	}
	srcScripts := filepath.Join(projectRoot, "src", "scripts", rest)
	if scriptFileExists(srcScripts) {
		return srcScripts
	}
	if p, ok := resolveCoreNamespacedAsset(repoRoot, projectRoot, rest); ok {
		return p
	}
	pkgRes := filepath.Join(projectRoot, DockpipeDirRel, "internal", "packages", "resolvers")
	pkgWf := filepath.Join(projectRoot, DockpipeDirRel, "internal", "packages", "workflows")
	// Compiled local store first (package compile resolvers → tarballs under pkgRes).
	if p, ok := tryBundledAssetsScripts(pkgRes, "", rest); ok {
		return p
	}
	if p, ok := scriptPathFromResolverTarballs(projectRoot, pkgRes, rest); ok {
		return p
	}
	if p, ok := tryBundledAssetsScripts(pkgWf, "", rest); ok {
		return p
	}
	if p, ok := scriptPathFromWorkflowTarballs(projectRoot, pkgWf, rest); ok {
		return p
	}
	if scriptFileExists(filepath.Join(pkgRes, rest)) {
		return filepath.Join(pkgRes, rest)
	}
	if scriptFileExists(filepath.Join(pkgWf, rest)) {
		return filepath.Join(pkgWf, rest)
	}
	core := CoreDir(repoRoot)
	wfRoots := WorkflowCompileRootsCached(projectRoot)
	bundleRoots := BundleCompileRootsCached(projectRoot)
	// Bundled cache layout: prefer spine resolvers/bundles under core/, then project compile roots
	// (packages/, workflows/, …) so nested resolver trees resolve even when repoRoot is the
	// materialized bundle and the real workflow lives under projectRoot.
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
	}
	for _, root := range wfRoots {
		if p, ok := tryBundledAssetsScripts(root, "", rest); ok {
			return p
		}
	}
	if p, ok := tryNestedWorkflowScripts(rest, wfRoots); ok {
		return p
	}
	if p, ok := workflowAssetPathForLogicalScript(rest, wfRoots); ok {
		return p
	}
	if p, ok := tryBundleRootsAssetsScripts(rest, bundleRoots); ok {
		return p
	}
	if p, ok := tryBundledAssetsScripts(core, "resolvers", rest); ok {
		return p
	}
	resolverPath := filepath.Join(core, "resolvers", rest)
	if scriptFileExists(resolverPath) {
		return resolverPath
	}
	for _, root := range wfRoots {
		p := filepath.Join(root, rest)
		if scriptFileExists(p) {
			return p
		}
	}
	for _, br := range bundleRoots {
		p := filepath.Join(br, rest)
		if scriptFileExists(p) {
			return p
		}
	}
	return filepath.Join(core, "assets", "scripts", rest)
}

// ResolveActionPath resolves act script like bin/dockpipe.
// projectRoot is the DockPipe project directory; pass "" to use repoRoot.
func ResolveActionPath(action, repoRoot, cwd, projectRoot string) (string, error) {
	if action == "" {
		return "", nil
	}
	if filepath.IsAbs(action) {
		return action, nil
	}
	candidates := []string{filepath.Join(repoRoot, action)}
	if strings.HasPrefix(action, "scripts/") {
		p := resolveScriptsPrefixedPath(repoRoot, projectRoot, action)
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
func ResolvePreScriptPath(p, repoRoot, projectRoot string) string {
	if filepath.IsAbs(p) {
		return p
	}
	if strings.HasPrefix(p, "scripts/") {
		return resolveScriptsPrefixedPath(repoRoot, projectRoot, p)
	}
	c := filepath.Join(repoRoot, p)
	if _, err := os.Stat(c); err == nil {
		return c
	}
	return p
}

// ResolveResolverFilePath returns the path to a specific resolver profile (KEY=value) by name.
// Search order: compiled packages/resolvers first, then compile.workflows roots (nested **/<name>/profile), then templates/core (or src/core).
func ResolveResolverFilePath(repoRoot, resolverName string) (string, error) {
	resolverName = strings.TrimSpace(resolverName)
	if resolverName == "" {
		return "", fmt.Errorf("resolver profile name is empty")
	}
	var candidates []string
	if pr, err := PackagesResolversDir(repoRoot); err == nil {
		candidates = append(candidates,
			filepath.Join(pr, resolverName),
			filepath.Join(pr, resolverName, "profile"),
		)
	}
	if !UsesBundledAssetLayout(repoRoot) {
		candidates = append(candidates, nestedResolverProfileCandidates(repoRoot, resolverName, ResolverCompileRootsCached(repoRoot))...)
	}
	candidates = append(candidates,
		filepath.Join(CoreDir(repoRoot), "resolvers", resolverName),
		filepath.Join(CoreDir(repoRoot), "resolvers", resolverName, "profile"),
	)
	if gr, err := GlobalPackagesResolversDir(); err == nil {
		candidates = append(candidates,
			filepath.Join(gr, resolverName),
			filepath.Join(gr, resolverName, "profile"),
		)
	}
	if tgz, err := FindLatestResolverTarball(repoRoot, resolverName); err == nil && tgz != "" {
		if root, err := packagebuild.EnsureTarballExtractedCache(tgz, TarballExtractCacheRoot(repoRoot)); err == nil {
			candidates = append(candidates,
				filepath.Join(root, "resolvers", resolverName),
				filepath.Join(root, "resolvers", resolverName, "profile"),
			)
		}
	}
	for _, p := range candidates {
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p, nil
		}
	}
	return "", fmt.Errorf("resolver profile not found for %q (tried %v); list maintainer trees under compile.workflows in %s (and ensure src/core/resolvers or templates/core/resolvers for bundled profiles) — use workflow YAML for custom flows — upgrade dockpipe or run `dockpipe doctor` if your install is stale", resolverName, candidates, domain.DockpipeProjectConfigFileName)
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
