package infrastructure

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// tryResolveRuntime returns the path to templates/core/runtimes/<name> or .../<name>/profile if either exists as a file.
func tryResolveRuntime(repoRoot, name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	candidates := []string{
		filepath.Join(CoreDir(repoRoot), "runtimes", name),
		filepath.Join(CoreDir(repoRoot), "runtimes", name, "profile"),
	}
	for _, p := range candidates {
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p
		}
	}
	return ""
}

// tryResolveResolver returns the path to a resolver profile file if it exists.
func tryResolveResolver(repoRoot, name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	var candidates []string
	if !UsesBundledAssetLayout(repoRoot) {
		candidates = append(candidates, nestedResolverProfileCandidates(repoRoot, name, ResolverCompileRootsCached(repoRoot))...)
	}
	candidates = append(candidates,
		filepath.Join(CoreDir(repoRoot), "resolvers", name),
		filepath.Join(CoreDir(repoRoot), "resolvers", name, "profile"),
	)
	for _, p := range candidates {
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p
		}
	}
	return ""
}

// LoadIsolationProfile loads and merges KEY=value maps from runtime and resolver profiles.
// Runtime keys (DOCKPIPE_RUNTIME_*) belong under templates/core/runtimes/<name>;
// resolver keys (DOCKPIPE_RESOLVER_*) under templates/core/resolvers/<name> (see docs/architecture-model.md).
//
// Merge order: runtime file(s) first, then resolver file(s); later keys overlay earlier for duplicate keys.
//
// When only one of (runtimeName, resolverName) is set, also tries the other path with the same basename
// so paired runtimes/foo + resolvers/foo work with a single CLI flag.
//
// When both names are set and differ, only templates/core/runtimes/<runtimeName> and
// templates/core/resolvers/<resolverName> are loaded (no cross-name fallback).
func LoadIsolationProfile(repoRoot, runtimeName, resolverName string) (map[string]string, error) {
	runtimeName = NormalizeRuntimeProfileName(strings.TrimSpace(runtimeName))
	resolverName = strings.TrimSpace(resolverName)
	explicitPair := runtimeName != "" && resolverName != "" && runtimeName != resolverName

	m := make(map[string]string)
	merge := func(path string) error {
		if path == "" {
			return nil
		}
		rm, err := LoadResolverFile(path)
		if err != nil {
			return err
		}
		for k, v := range rm {
			m[k] = v
		}
		return nil
	}

	if runtimeName != "" {
		if err := merge(tryResolveRuntime(repoRoot, runtimeName)); err != nil {
			return nil, err
		}
		if !explicitPair && resolverName == "" {
			if err := merge(tryResolveResolver(repoRoot, runtimeName)); err != nil {
				return nil, err
			}
		}
	}
	if resolverName != "" {
		if !explicitPair && runtimeName == "" {
			if err := merge(tryResolveRuntime(repoRoot, resolverName)); err != nil {
				return nil, err
			}
		}
		if err := merge(tryResolveResolver(repoRoot, resolverName)); err != nil {
			return nil, err
		}
	}

	if len(m) == 0 {
		return nil, fmt.Errorf("no isolation profile files found for runtime=%q resolver=%q", runtimeName, resolverName)
	}
	return m, nil
}
