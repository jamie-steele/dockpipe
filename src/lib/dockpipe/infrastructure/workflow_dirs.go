package infrastructure

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ResolveWorkflowConfigPath returns the first existing workflow config for a bundled or user workflow name.
// Authoring checkout: shipyard/workflows/<name>/config.yml first (repo-local), then src/templates/<name>/ or templates/<name>/config.yml, then core/resolvers.
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
	candidates = append(candidates,
		filepath.Join(WorkflowsRootDir(repoRoot), name, "config.yml"),
		filepath.Join(CoreDir(repoRoot), "resolvers", name, "config.yml"),
	)
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
	for _, p := range candidates {
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p, nil
		}
	}
	return "", fmt.Errorf("embedded resolver workflow config not found for %q", name)
}

// ListWorkflowNamesInRepoRoot returns workflow names from the authoring templates root/<name>/ and (authoring only) shipyard/workflows/<name>/.
func ListWorkflowNamesInRepoRoot(repoRoot string) ([]string, error) {
	seen := make(map[string]struct{})
	var out []string

	templatesDir := WorkflowsRootDir(repoRoot)
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
			if base == templatesDir && e.Name() == "core" {
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

	if err := addDir(templatesDir); err != nil {
		return nil, err
	}
	if !UsesBundledAssetLayout(repoRoot) {
		if err := addDir(AuthoringShipyardWorkflowsDir(repoRoot)); err != nil {
			return nil, err
		}
	}

	sort.Strings(out)
	return out, nil
}
