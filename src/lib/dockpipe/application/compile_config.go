package application

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"dockpipe/src/lib/dockpipe/domain"
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

// effectiveWorkflowCompileRoots merges dockpipe.config.json compile.workflows with CLI defaults.
// noStaging drops paths whose clean path starts with ".staging".
func effectiveWorkflowCompileRoots(cfg *domain.DockpipeProjectConfig, repoRoot string, noStaging bool) []string {
	if cfg != nil && cfg.Compile.Workflows != nil {
		return resolveCompilePathList(repoRoot, *cfg.Compile.Workflows, noStaging)
	}
	return defaultWorkflowRoots(repoRoot, noStaging)
}

// effectiveResolverCompileRoots merges config with defaults: src/core/resolvers then .staging/resolvers when present.
func effectiveResolverCompileRoots(cfg *domain.DockpipeProjectConfig, repoRoot string, noStaging bool) []string {
	if cfg != nil && cfg.Compile.Resolvers != nil {
		return resolveCompilePathList(repoRoot, *cfg.Compile.Resolvers, noStaging)
	}
	var rels []string
	rels = append(rels, filepath.Join("src", "core", "resolvers"))
	rels = append(rels, filepath.Join("templates", "core", "resolvers"))
	if !noStaging {
		rels = append(rels, filepath.Join(".staging", "resolvers"))
	}
	var out []string
	for _, rel := range rels {
		p := filepath.Join(repoRoot, rel)
		if _, err := os.Stat(p); err == nil {
			out = append(out, p)
		}
	}
	return out
}

// effectiveBundleCompileRoots uses config or defaults to .staging/bundles when present.
func effectiveBundleCompileRoots(cfg *domain.DockpipeProjectConfig, repoRoot string, noStaging bool) []string {
	if cfg != nil && cfg.Compile.Bundles != nil {
		return resolveCompilePathList(repoRoot, *cfg.Compile.Bundles, noStaging)
	}
	if noStaging {
		return nil
	}
	p := filepath.Join(repoRoot, ".staging", "bundles")
	if _, err := os.Stat(p); err == nil {
		return []string{p}
	}
	return nil
}

func resolveCompilePathList(repoRoot string, rels []string, noStaging bool) []string {
	var out []string
	for _, r := range rels {
		r = strings.TrimSpace(r)
		if r == "" {
			continue
		}
		abs := r
		if !filepath.IsAbs(r) {
			abs = filepath.Join(repoRoot, filepath.Clean(r))
		} else {
			abs = filepath.Clean(r)
		}
		if noStaging {
			relToRepo, err := filepath.Rel(repoRoot, abs)
			if err == nil && !strings.HasPrefix(relToRepo, "..") && strings.HasPrefix(relToRepo, ".staging") {
				continue
			}
		}
		if _, err := os.Stat(abs); err == nil {
			out = append(out, abs)
		} else {
			fmt.Fprintf(os.Stderr, "[dockpipe] config: skip missing path %s\n", abs)
		}
	}
	return out
}

func defaultWorkflowRoots(repoRoot string, noStaging bool) []string {
	var out []string
	rels := []string{"workflows", filepath.Join(".staging", "workflows")}
	for _, rel := range rels {
		if noStaging && strings.HasPrefix(filepath.Clean(rel), ".staging") {
			continue
		}
		p := filepath.Join(repoRoot, rel)
		if _, err := os.Stat(p); err == nil {
			out = append(out, p)
		}
	}
	return out
}
