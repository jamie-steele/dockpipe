package infrastructure

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ResolveWorkflowConfigPath returns the first existing workflow config for a bundled or user workflow name.
// Authoring checkout: shipyard/workflows/<name>/config.yml first (repo-local), then WorkflowsRootDir/<name>/config.yml
// (workflows/ or src/templates/), then legacy templates/<name>/config.yml for normal projects, then core/resolvers.
// Materialized bundle: shipyard/workflows/ only (same path as WorkflowsRootDir).
func ResolveWorkflowConfigPath(repoRoot, name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", fmt.Errorf("workflow name is empty")
	}
	var candidates []string
	if !UsesBundledAssetLayout(repoRoot) {
		candidates = append(candidates, filepath.Join(AuthoringShipyardWorkflowsDir(repoRoot), name, "config.yml"))
	}
	candidates = append(candidates, filepath.Join(WorkflowsRootDir(repoRoot), name, "config.yml"))
	if !UsesBundledAssetLayout(repoRoot) && !DockpipeAuthoringSourceTree(repoRoot) {
		candidates = append(candidates, filepath.Join(repoRoot, "templates", name, "config.yml"))
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
// Order: core/resolvers/<name>/config.yml, workflows root/<name>/config.yml.
func ResolveEmbeddedResolverWorkflowConfigPath(repoRoot, name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", fmt.Errorf("embedded resolver workflow name is empty")
	}
	candidates := []string{
		filepath.Join(CoreDir(repoRoot), "resolvers", name, "config.yml"),
		filepath.Join(WorkflowsRootDir(repoRoot), name, "config.yml"),
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

// ListWorkflowNamesInRepoRoot returns workflow names from WorkflowsRootDir, legacy templates/ (user projects),
// and (authoring only) shipyard/workflows/.
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
	if !UsesBundledAssetLayout(repoRoot) && !DockpipeAuthoringSourceTree(repoRoot) {
		if err := addDir(filepath.Join(repoRoot, "templates")); err != nil {
			return nil, err
		}
	}
	if !UsesBundledAssetLayout(repoRoot) {
		if err := addDir(AuthoringShipyardWorkflowsDir(repoRoot)); err != nil {
			return nil, err
		}
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
