package domain

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// EffectiveWorkflowCompileRoots merges dockpipe.config.json compile.workflows with CLI defaults.
// noStaging drops paths whose repo-relative path starts with ".staging".
func EffectiveWorkflowCompileRoots(cfg *DockpipeProjectConfig, repoRoot string, noStaging bool) []string {
	var out []string
	if cfg != nil && cfg.Compile.Workflows != nil {
		out = resolveCompilePathList(repoRoot, *cfg.Compile.Workflows, noStaging)
	} else {
		out = defaultWorkflowRoots(repoRoot, noStaging)
	}
	return out
}

// EffectiveResolverCompileRoots merges config with defaults: src/core/resolvers, templates/core/resolvers.
func EffectiveResolverCompileRoots(cfg *DockpipeProjectConfig, repoRoot string, noStaging bool) []string {
	if cfg != nil && cfg.Compile.Resolvers != nil {
		return resolveCompilePathList(repoRoot, *cfg.Compile.Resolvers, noStaging)
	}
	var out []string
	rels := []string{
		filepath.Join("src", "core", "resolvers"),
		filepath.Join("templates", "core", "resolvers"),
	}
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

// EffectiveBundleCompileRoots uses compile.bundles when set; otherwise no default (empty slice).
func EffectiveBundleCompileRoots(cfg *DockpipeProjectConfig, repoRoot string, noStaging bool) []string {
	if cfg != nil && cfg.Compile.Bundles != nil {
		return resolveCompilePathList(repoRoot, *cfg.Compile.Bundles, noStaging)
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
	rels := []string{"workflows", filepath.Join("src", "lib", "dorkpipe", "workflows")}
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
