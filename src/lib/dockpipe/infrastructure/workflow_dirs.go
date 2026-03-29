package infrastructure

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"dockpipe/src/lib/dockpipe/infrastructure/packagebuild"
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
		candidates = append(candidates, nestedWorkflowConfigCandidates(repoRoot, name, WorkflowCompileRootsCached(repoRoot))...)
	}
	if u, err := tryResolveWorkflowTarballURI(repoRoot, workdir, name); err != nil {
		return "", err
	} else if u != "" {
		return u, nil
	}
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
		candidates = append(candidates, nestedWorkflowConfigCandidates(repoRoot, name, WorkflowCompileRootsCached(repoRoot))...)
	}
	candidates = append(candidates,
		filepath.Join(CoreDir(repoRoot), "resolvers", name, "config.yml"),
		filepath.Join(WorkflowsRootDir(repoRoot), name, "config.yml"),
	)
	if u, err := tryResolveWorkflowTarballURI(repoRoot, workdir, name); err != nil {
		return "", err
	} else if u != "" {
		return u, nil
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
		if bw := BundledWorkflowsAuthoringDir(repoRoot); bw != "" {
			_ = filepath.WalkDir(bw, func(path string, d os.DirEntry, err error) error {
				if err != nil {
					if os.IsNotExist(err) {
						return nil
					}
					return err
				}
				if d.IsDir() || d.Name() != "config.yml" {
					return nil
				}
				name := filepath.Base(filepath.Dir(path))
				if _, ok := seen[name]; !ok {
					seen[name] = struct{}{}
					out = append(out, name)
				}
				return nil
			})
		}
		if err := addDir(DorkpipeLibraryWorkflowsDir(repoRoot)); err != nil {
			return nil, err
		}
		for _, wfRoot := range WorkflowCompileRootsCached(repoRoot) {
			if err := addDir(wfRoot); err != nil {
				return nil, err
			}
			_ = filepath.WalkDir(wfRoot, func(path string, d os.DirEntry, err error) error {
				if err != nil {
					if os.IsNotExist(err) {
						return nil
					}
					return err
				}
				if d.IsDir() || d.Name() != "config.yml" {
					return nil
				}
				name := filepath.Base(filepath.Dir(path))
				if _, ok := seen[name]; !ok {
					seen[name] = struct{}{}
					out = append(out, name)
				}
				return nil
			})
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

// ListWorkflowNamesInPackagesStore returns workflow names from dockpipe-workflow-*.tar.gz under packages/workflows (streamable blobs only).
func ListWorkflowNamesInPackagesStore(workdir string) ([]string, error) {
	root, err := PackagesWorkflowsDir(workdir)
	if err != nil {
		return nil, err
	}
	matches, err := filepath.Glob(filepath.Join(root, "dockpipe-workflow-*.tar.gz"))
	if err != nil {
		return nil, err
	}
	var out []string
	for _, tgz := range matches {
		members, err := packagebuild.ListTarGzMemberPaths(tgz)
		if err != nil {
			continue
		}
		name, err := packagebuild.WorkflowNameFromTarballMembers(members)
		if err != nil {
			continue
		}
		out = append(out, name)
	}
	sort.Strings(out)
	return out, nil
}

// ListWorkflowNamesInGlobalPackagesStore lists workflows from dockpipe-workflow-*.tar.gz under the global packages/workflows dir.
func ListWorkflowNamesInGlobalPackagesStore() ([]string, error) {
	root, err := GlobalPackagesWorkflowsDir()
	if err != nil {
		return nil, err
	}
	matches, err := filepath.Glob(filepath.Join(root, "dockpipe-workflow-*.tar.gz"))
	if err != nil {
		return nil, err
	}
	if len(matches) == 0 {
		return nil, nil
	}
	var out []string
	for _, tgz := range matches {
		members, err := packagebuild.ListTarGzMemberPaths(tgz)
		if err != nil {
			continue
		}
		name, err := packagebuild.WorkflowNameFromTarballMembers(members)
		if err != nil {
			continue
		}
		out = append(out, name)
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
	var b []string
	if strings.TrimSpace(workdir) != "" {
		b, err = ListWorkflowNamesInPackagesStore(workdir)
		if err != nil {
			return nil, err
		}
	}
	g, err := ListWorkflowNamesInGlobalPackagesStore()
	if err != nil {
		return nil, err
	}
	seen := make(map[string]struct{})
	var out []string
	for _, x := range append(append(append([]string{}, a...), b...), g...) {
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
