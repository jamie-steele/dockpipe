package application

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"dockpipe/src/lib/dockpipe/domain"
	"dockpipe/src/lib/dockpipe/infrastructure"

	"gopkg.in/yaml.v3"
)

// validateCompileOutputs checks that compiled workflow and resolver packages resolve a valid
// namespace (package.yml, config.yml / resolver.yaml, or repo-root dockpipe.config.json packages.namespace)
// and that package.yml depends entries refer to packages present in the compiled store.
// Compile order (core → resolvers → workflows) must be respected so dependencies exist before dependents.
func validateCompileOutputs(workdir string) error {
	repoRoot, err := filepath.Abs(workdir)
	if err != nil {
		return err
	}
	cfg, err := loadDockpipeProjectConfig(repoRoot)
	if err != nil {
		return err
	}
	pkgs, err := infrastructure.PackagesRoot(workdir)
	if err != nil {
		return err
	}
	known := compiledPackageNames(pkgs)
	for _, kind := range []string{"workflows", "resolvers"} {
		base := filepath.Join(pkgs, kind)
		entries, err := os.ReadDir(base)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return err
		}
		for _, e := range entries {
			if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
				continue
			}
			dir := filepath.Join(base, e.Name())
			pmPath := filepath.Join(dir, infrastructure.PackageManifestFilename)
			pm, err := domain.ParsePackageManifest(pmPath)
			if err != nil {
				return fmt.Errorf("validate %s: %w", pmPath, err)
			}
			var ns string
			switch kind {
			case "workflows":
				ns = effectiveWorkflowNamespace(pm, dir, cfg)
			case "resolvers":
				ns, err = effectiveResolverNamespace(pm, dir, cfg)
				if err != nil {
					return fmt.Errorf("resolver %q: %w", e.Name(), err)
				}
			}
			if strings.TrimSpace(ns) == "" {
				return fmt.Errorf("compiled %s package %q must set namespace (package.yml, config.yml or resolver.yaml, or packages.namespace in %s) — see docs/package-model.md", kind, e.Name(), domain.DockpipeProjectConfigFileName)
			}
			if err := domain.ValidateNamespace(ns); err != nil {
				return fmt.Errorf("%s: %w", pmPath, err)
			}
			for _, dep := range pm.Depends {
				dep = strings.TrimSpace(dep)
				if dep == "" {
					continue
				}
				if !known[dep] {
					return fmt.Errorf("package %q depends on %q, which is not present in the compiled store — build core → resolvers → workflows (and bundles if needed) so dependencies exist first", pm.Name, dep)
				}
			}
		}
	}
	return nil
}

func effectiveWorkflowNamespace(pm *domain.PackageManifest, pkgDir string, cfg *domain.DockpipeProjectConfig) string {
	if s := strings.TrimSpace(pm.Namespace); s != "" {
		return s
	}
	if s := namespaceFromWorkflowConfig(filepath.Join(pkgDir, "config.yml")); s != "" {
		return s
	}
	return defaultNamespaceFromProjectConfig(cfg)
}

func effectiveResolverNamespace(pm *domain.PackageManifest, pkgDir string, cfg *domain.DockpipeProjectConfig) (string, error) {
	if s := strings.TrimSpace(pm.Namespace); s != "" {
		return s, nil
	}
	ns, err := readResolverNamespaceYAML(pkgDir)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(ns) != "" {
		return ns, nil
	}
	return defaultNamespaceFromProjectConfig(cfg), nil
}

func defaultNamespaceFromProjectConfig(cfg *domain.DockpipeProjectConfig) string {
	if cfg == nil || cfg.Packages.Namespace == nil {
		return ""
	}
	return strings.TrimSpace(*cfg.Packages.Namespace)
}

func namespaceFromWorkflowConfig(cfgPath string) string {
	b, err := os.ReadFile(cfgPath)
	if err != nil {
		return ""
	}
	var top struct {
		Namespace string `yaml:"namespace"`
	}
	if err := yaml.Unmarshal(b, &top); err != nil {
		return ""
	}
	return strings.TrimSpace(top.Namespace)
}

func compiledPackageNames(pkgsRoot string) map[string]bool {
	out := make(map[string]bool)
	corePath := filepath.Join(pkgsRoot, "core", infrastructure.PackageManifestFilename)
	if pm, err := domain.ParsePackageManifest(corePath); err == nil && strings.TrimSpace(pm.Name) != "" {
		out[strings.TrimSpace(pm.Name)] = true
	}
	for _, kind := range []string{"workflows", "resolvers", "bundles"} {
		base := filepath.Join(pkgsRoot, kind)
		entries, err := os.ReadDir(base)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			pmPath := filepath.Join(base, e.Name(), infrastructure.PackageManifestFilename)
			pm, err := domain.ParsePackageManifest(pmPath)
			if err != nil {
				continue
			}
			if n := strings.TrimSpace(pm.Name); n != "" {
				out[n] = true
			}
		}
	}
	return out
}
