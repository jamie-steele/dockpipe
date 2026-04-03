package infrastructure

import (
	"path/filepath"
	"sync"

	"dockpipe/src/lib/domain"
)

var (
	wfRootsCache  sync.Map // string (abs repo root) -> []string
	resRootsCache sync.Map
	bunRootsCache sync.Map
)

func absRepoKey(repoRoot string) string {
	a, err := filepath.Abs(repoRoot)
	if err != nil {
		return filepath.Clean(repoRoot)
	}
	return filepath.Clean(a)
}

// WorkflowCompileRootsCached returns absolute workflow compile roots from dockpipe.config.json
// (same list as dockpipe package compile workflows). Cached per process.
func WorkflowCompileRootsCached(repoRoot string) []string {
	k := absRepoKey(repoRoot)
	if v, ok := wfRootsCache.Load(k); ok {
		return v.([]string)
	}
	cfg, _ := domain.LoadDockpipeProjectConfig(repoRoot)
	out := domain.EffectiveWorkflowCompileRoots(cfg, repoRoot, false)
	wfRootsCache.Store(k, out)
	return out
}

// ResolverCompileRootsCached returns absolute resolver compile roots from dockpipe.config.json.
func ResolverCompileRootsCached(repoRoot string) []string {
	k := absRepoKey(repoRoot)
	if v, ok := resRootsCache.Load(k); ok {
		return v.([]string)
	}
	cfg, _ := domain.LoadDockpipeProjectConfig(repoRoot)
	out := domain.EffectiveResolverCompileRoots(cfg, repoRoot, false)
	resRootsCache.Store(k, out)
	return out
}

// BundleCompileRootsCached returns compile.bundles paths for DockerfileDir and source script resolution.
// There is no implicit default: list bundle roots in dockpipe.config.json when you need them.
func BundleCompileRootsCached(repoRoot string) []string {
	k := absRepoKey(repoRoot)
	if v, ok := bunRootsCache.Load(k); ok {
		return v.([]string)
	}
	cfg, _ := domain.LoadDockpipeProjectConfig(repoRoot)
	out := domain.EffectiveBundleCompileRoots(cfg, repoRoot, false)
	bunRootsCache.Store(k, out)
	return out
}
