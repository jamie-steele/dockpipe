package domain

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// EffectiveWorkflowCompileRoots merges dockpipe.config.json compile.workflows with CLI defaults.
// compile.bundles (deprecated) entries are appended and deduplicated — same recursive config.yml walk as workflows.
// noStaging drops paths whose repo-relative path starts with ".staging".
func EffectiveWorkflowCompileRoots(cfg *DockpipeProjectConfig, repoRoot string, noStaging bool) []string {
	var out []string
	if cfg != nil && cfg.Compile.Workflows != nil {
		out = resolveCompilePathList(repoRoot, *cfg.Compile.Workflows, noStaging)
	} else {
		out = defaultWorkflowRoots(repoRoot, noStaging)
	}
	if cfg != nil && cfg.Compile.Bundles != nil {
		out = mergeUniqueAbsPaths(out, resolveCompilePathList(repoRoot, *cfg.Compile.Bundles, noStaging))
	}
	return out
}

func mergeUniqueAbsPaths(a, b []string) []string {
	seen := map[string]struct{}{}
	for _, p := range a {
		p = filepath.Clean(p)
		seen[p] = struct{}{}
	}
	for _, p := range b {
		p = filepath.Clean(p)
		if _, ok := seen[p]; ok {
			continue
		}
		seen[p] = struct{}{}
		a = append(a, p)
	}
	return a
}

// EffectiveResolverCompileRoots merges workflow compile roots with flat core resolver dirs.
// Resolver packs under packages/, .staging/packages/, etc. are discovered from the same roots as
// compile.workflows (plus legacy compile.bundles merged into workflows). Flat vendor trees
// src/core/resolvers and templates/core/resolvers are always appended when present.
// Deprecated: compile.resolvers in JSON is still merged when set (for old configs).
func EffectiveResolverCompileRoots(cfg *DockpipeProjectConfig, repoRoot string, noStaging bool) []string {
	wf := EffectiveWorkflowCompileRoots(cfg, repoRoot, noStaging)
	var core []string
	for _, rel := range []string{
		filepath.Join("src", "core", "resolvers"),
		filepath.Join("templates", "core", "resolvers"),
	} {
		if noStaging && strings.HasPrefix(filepath.Clean(rel), ".staging") {
			continue
		}
		p := filepath.Join(repoRoot, rel)
		if _, err := os.Stat(p); err == nil {
			core = append(core, p)
		}
	}
	out := mergeUniqueAbsPaths(wf, core)
	if cfg != nil && cfg.Compile.Resolvers != nil {
		legacy := resolveCompilePathList(repoRoot, *cfg.Compile.Resolvers, noStaging)
		out = mergeUniqueAbsPaths(out, legacy)
	}
	return out
}

// EffectiveBundleCompileRoots returns paths from compile.bundles only (for DockerfileDir and other
// lookups that need the bundle compile root without walking the whole workflows list). Compile itself
// merges these into EffectiveWorkflowCompileRoots — there is no separate bundle compile step.
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
	rels := []string{"workflows"}
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
