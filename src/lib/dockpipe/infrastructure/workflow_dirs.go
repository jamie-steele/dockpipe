package infrastructure

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ResolveWorkflowConfigPath returns the first existing workflow config for a bundled or user workflow name.
// Resolution uses WorkflowsRootDir (repo workflows/, materialized shipyard/workflows/ in bundle cache, or src/core/workflows in dockpipe source when workflows/ is empty).
// Does not consult .dockpipe/internal/packages (use ResolveWorkflowConfigPathWithWorkdir for that).
func ResolveWorkflowConfigPath(repoRoot, name string) (string, error) {
	return ResolveWorkflowConfigPathWithWorkdir(repoRoot, "", name)
}

// ResolveWorkflowConfigPathWithWorkdir is like ResolveWorkflowConfigPath but when workdir is non-empty also checks
// <workdir>/.dockpipe/internal/packages/workflows/<name>/config.yml after WorkflowsRootDir and before legacy templates/.
func ResolveWorkflowConfigPathWithWorkdir(repoRoot, workdir, name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", fmt.Errorf("workflow name is empty")
	}
	var candidates []string
	candidates = append(candidates, filepath.Join(WorkflowsRootDir(repoRoot), name, "config.yml"))
	if !UsesBundledAssetLayout(repoRoot) {
		candidates = append(candidates, filepath.Join(StagingWorkflowsDir(repoRoot), name, "config.yml"))
	}
	if strings.TrimSpace(workdir) != "" {
		if pw, err := PackagesWorkflowsDir(workdir); err == nil {
			candidates = append(candidates, filepath.Join(pw, name, "config.yml"))
		}
		if pc, err := PackagesCoreDir(workdir); err == nil {
			candidates = append(candidates,
				filepath.Join(pc, "workflows", name, "config.yml"),
				filepath.Join(pc, "resolvers", name, "config.yml"),
			)
		}
	}
	if !UsesBundledAssetLayout(repoRoot) && !DockpipeAuthoringSourceTree(repoRoot) {
		candidates = append(candidates, filepath.Join(repoRoot, "templates", name, "config.yml"))
	}
	if !UsesBundledAssetLayout(repoRoot) {
		candidates = append(candidates, filepath.Join(StagingResolversDir(repoRoot), name, "config.yml"))
	}
	candidates = append(candidates, filepath.Join(CoreDir(repoRoot), "resolvers", name, "config.yml"))
	for _, p := range candidates {
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p, nil
		}
	}
	return "", fmt.Errorf("workflow config not found for %q", name)
}

// ResolveEmbeddedResolverWorkflowConfigPath returns delegate YAML for DOCKPIPE_*_WORKFLOW (resolver-driven isolate).
// Does not consult the packages store (use WithWorkdir).
func ResolveEmbeddedResolverWorkflowConfigPath(repoRoot, name string) (string, error) {
	return ResolveEmbeddedResolverWorkflowConfigPathWithWorkdir(repoRoot, "", name)
}

// ResolveEmbeddedResolverWorkflowConfigPathWithWorkdir adds .dockpipe/internal/packages/workflows/<name>/config.yml
// when workdir is set, after the workflows root and before legacy templates/.
func ResolveEmbeddedResolverWorkflowConfigPathWithWorkdir(repoRoot, workdir, name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", fmt.Errorf("embedded resolver workflow name is empty")
	}
	var candidates []string
	if !UsesBundledAssetLayout(repoRoot) {
		candidates = append(candidates, filepath.Join(StagingResolversDir(repoRoot), name, "config.yml"))
	}
	candidates = append(candidates,
		filepath.Join(CoreDir(repoRoot), "resolvers", name, "config.yml"),
		filepath.Join(WorkflowsRootDir(repoRoot), name, "config.yml"),
	)
	if strings.TrimSpace(workdir) != "" {
		if pc, err := PackagesCoreDir(workdir); err == nil {
			candidates = append(candidates, filepath.Join(pc, "resolvers", name, "config.yml"))
		}
		if pw, err := PackagesWorkflowsDir(workdir); err == nil {
			candidates = append(candidates, filepath.Join(pw, name, "config.yml"))
		}
	}
	if !UsesBundledAssetLayout(repoRoot) && !DockpipeAuthoringSourceTree(repoRoot) {
		candidates = append(candidates, filepath.Join(repoRoot, "templates", name, "config.yml"))
	}
	for _, p := range candidates {
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p, nil
		}
	}
	return "", fmt.Errorf("embedded resolver workflow config not found for %q", name)
}

// ListWorkflowNamesInRepoRoot returns workflow names from WorkflowsRootDir and legacy templates/ (user projects).
func ListWorkflowNamesInRepoRoot(repoRoot string) ([]string, error) {
	seen := make(map[string]struct{})
	var out []string

	addDir := func(base string) error {
		entries, err := os.ReadDir(base)
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			if e.Name() == "core" && filepath.Base(base) == "templates" {
				continue
			}
			name := e.Name()
			cfg := filepath.Join(base, name, "config.yml")
			if st, err := os.Stat(cfg); err == nil && !st.IsDir() {
				if _, ok := seen[name]; !ok {
					seen[name] = struct{}{}
					out = append(out, name)
				}
			}
		}
		return nil
	}

	if err := addDir(WorkflowsRootDir(repoRoot)); err != nil {
		return nil, err
	}
	if !UsesBundledAssetLayout(repoRoot) {
		if err := addDir(StagingWorkflowsDir(repoRoot)); err != nil {
			return nil, err
		}
	}
	if !UsesBundledAssetLayout(repoRoot) && !DockpipeAuthoringSourceTree(repoRoot) {
		if err := addDir(filepath.Join(repoRoot, "templates")); err != nil {
			return nil, err
		}
	}

	sort.Strings(out)
	return out, nil
}

// ListWorkflowNamesInPackagesStore returns workflow names under workdir/.dockpipe/internal/packages/workflows/*/config.yml
// and packages/core/workflows/*/config.yml when present.
func ListWorkflowNamesInPackagesStore(workdir string) ([]string, error) {
	root, err := PackagesWorkflowsDir(workdir)
	if err != nil {
		return nil, err
	}
	seen := make(map[string]struct{})
	var out []string
	addDir := func(base string) error {
		entries, err := os.ReadDir(base)
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			name := e.Name()
			cfg := filepath.Join(base, name, "config.yml")
			if st, err := os.Stat(cfg); err == nil && !st.IsDir() {
				if _, ok := seen[name]; !ok {
					seen[name] = struct{}{}
					out = append(out, name)
				}
			}
		}
		return nil
	}
	if err := addDir(root); err != nil {
		return nil, err
	}
	if pc, err := PackagesCoreDir(workdir); err == nil {
		if err := addDir(filepath.Join(pc, "workflows")); err != nil {
			return nil, err
		}
	}
	sort.Strings(out)
	return out, nil
}

// ListWorkflowNamesInRepoRootAndPackages merges ListWorkflowNamesInRepoRoot with ListWorkflowNamesInPackagesStore (deduped).
func ListWorkflowNamesInRepoRootAndPackages(repoRoot, workdir string) ([]string, error) {
	a, err := ListWorkflowNamesInRepoRoot(repoRoot)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(workdir) == "" {
		return a, nil
	}
	b, err := ListWorkflowNamesInPackagesStore(workdir)
	if err != nil {
		return nil, err
	}
	seen := make(map[string]struct{})
	var out []string
	for _, x := range append(append([]string{}, a...), b...) {
		if _, ok := seen[x]; ok {
			continue
		}
		seen[x] = struct{}{}
		out = append(out, x)
	}
	sort.Strings(out)
	return out, nil
}

// WorkflowsDirHasDockpipeWorkflow reports whether dir contains at least one immediate child directory with config.yml.
func WorkflowsDirHasDockpipeWorkflow(dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		cfg := filepath.Join(dir, e.Name(), "config.yml")
		if st, err := os.Stat(cfg); err == nil && !st.IsDir() {
			return true
		}
	}
	return false
}
