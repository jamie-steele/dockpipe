package application

import (
	"path/filepath"
	"strings"

	"dockpipe/src/lib/domain"
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
	return domain.EffectiveWorkflowCompileRoots(cfg, repoRoot)
}

func effectiveResolverCompileRoots(cfg *domain.DockpipeProjectConfig, repoRoot string) []string {
	return domain.EffectiveResolverCompileRoots(cfg, repoRoot)
}
