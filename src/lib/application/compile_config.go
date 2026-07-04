package application

import (
	"os"
	"path/filepath"
	"strings"

	"dockpipe/src/lib/domain"
	"dockpipe/src/lib/infrastructure"
)

// loadDockpipeProjectConfig reads dockpipe.config.json or returns (nil, nil) if absent. Parse errors propagate.
func loadDockpipeProjectConfig(repoRoot string) (*domain.DockpipeProjectConfig, error) {
	return domain.LoadDockpipeProjectConfig(repoRoot)
}

// coreFromConfig returns a core tree path when compile.core_from is set in config; otherwise ("", nil).
func coreFromConfig(cfg *domain.DockpipeProjectConfig, repoRoot string) (string, error) {
	if cfg == nil || cfg.Compile.CoreFrom == nil {
		return "", nil
	}
	p := strings.TrimSpace(*cfg.Compile.CoreFrom)
	if p == "" {
		return "", nil
	}
	if filepath.IsAbs(p) {
		return filepath.Clean(p), nil
	}
	return filepath.Join(repoRoot, filepath.Clean(p)), nil
}

func effectiveWorkflowCompileRoots(cfg *domain.DockpipeProjectConfig, repoRoot string) []string {
	result := domain.EffectiveWorkflowCompileRootsDetailed(cfg, repoRoot)
	logCompilePathWarnings("workflows", result.MissingPaths)
	return result.Paths
}

func effectiveResolverCompileRoots(cfg *domain.DockpipeProjectConfig, repoRoot string) []string {
	result := domain.EffectiveResolverCompileRootsDetailed(cfg, repoRoot)
	logCompilePathWarnings("resolvers", result.MissingPaths)
	return result.Paths
}

func effectiveBundleCompileRoots(cfg *domain.DockpipeProjectConfig, repoRoot string) []string {
	result := domain.EffectiveBundleCompileRootsDetailed(cfg, repoRoot)
	logCompilePathWarnings("bundles", result.MissingPaths)
	return result.Paths
}

func logCompilePathWarnings(rootKind string, missingPaths []string) {
	for _, path := range missingPaths {
		path = strings.TrimSpace(path)
		if path == "" {
			continue
		}
		infrastructure.LogOperationResult(os.Stderr, infrastructure.OperationResult{
			Unit:       "config.compile_path",
			Status:     infrastructure.OperationStatusDone,
			DurationMs: 0,
			IDs: map[string]string{
				"path":        path,
				"result":      "skip",
				"root_kind":   strings.TrimSpace(rootKind),
				"skip_reason": "missing_path",
			},
		})
	}
}
