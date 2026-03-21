package infrastructure

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ResolveWorkflowConfigPath returns the first existing workflow config for a bundled or user workflow name.
// Order: workflows root/<name>/config.yml, core/resolvers/<name>/config.yml (authoring: templates/...; materialized bundle: dockpipe/...).
func ResolveWorkflowConfigPath(repoRoot, name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", fmt.Errorf("workflow name is empty")
	}
	candidates := []string{
		filepath.Join(WorkflowsRootDir(repoRoot), name, "config.yml"),
		filepath.Join(CoreDir(repoRoot), "resolvers", name, "config.yml"),
	}
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

// ListWorkflowNamesInRepoRoot returns workflow names from the workflows root (authoring: templates/<name>/ excluding templates/core; bundle: dockpipe/workflows/<name>/).
func ListWorkflowNamesInRepoRoot(repoRoot string) ([]string, error) {
	seen := make(map[string]struct{})
	var out []string

	templatesDir := WorkflowsRootDir(repoRoot)
	entries, err := os.ReadDir(templatesDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if !UsesBundledAssetLayout(repoRoot) && e.Name() == "core" {
			continue
		}
		name := e.Name()
		cfg := filepath.Join(templatesDir, name, "config.yml")
		if st, err := os.Stat(cfg); err == nil && !st.IsDir() {
			if _, ok := seen[name]; !ok {
				seen[name] = struct{}{}
				out = append(out, name)
			}
		}
	}

	sort.Strings(out)
	return out, nil
}
