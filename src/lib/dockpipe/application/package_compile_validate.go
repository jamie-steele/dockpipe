package application

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"dockpipe/src/lib/dockpipe/domain"
	"dockpipe/src/lib/dockpipe/infrastructure"
	"dockpipe/src/lib/dockpipe/infrastructure/packagebuild"

	"gopkg.in/yaml.v3"
)

// validateCompileOutputs checks workflow and resolver tarballs under the compiled store for namespace
// and depends closure (compile order: core → resolvers → workflows).
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
	known := compiledPackageNamesFromTarballs(pkgs)
	for _, kind := range []string{"workflows", "resolvers"} {
		var pattern string
		switch kind {
		case "workflows":
			pattern = filepath.Join(pkgs, "workflows", "dockpipe-workflow-*.tar.gz")
		case "resolvers":
			pattern = filepath.Join(pkgs, "resolvers", "dockpipe-resolver-*.tar.gz")
		}
		matches, err := filepath.Glob(pattern)
		if err != nil {
			return err
		}
		for _, tgz := range matches {
			members, err := packagebuild.ListTarGzMemberPaths(tgz)
			if err != nil {
				return fmt.Errorf("validate %s: %w", tgz, err)
			}
			pmPathInTar, pm, err := readPackageManifestFromTarballMembers(kind, members, tgz)
			if err != nil {
				return fmt.Errorf("validate %s: %w", tgz, err)
			}
			var ns string
			switch kind {
			case "workflows":
				wfName, err := packagebuild.WorkflowNameFromTarballMembers(members)
				if err != nil {
					return fmt.Errorf("validate %s: %w", tgz, err)
				}
				cfgPath := filepath.ToSlash(filepath.Join("workflows", wfName, "config.yml"))
				b, err := packagebuild.ReadFileFromTarGz(tgz, cfgPath)
				if err != nil {
					return fmt.Errorf("validate %s: %w", tgz, err)
				}
				ns = effectiveWorkflowNamespace(pm, b, cfg)
			case "resolvers":
				ns, err = effectiveResolverNamespaceFromTar(pm, tgz, members, cfg)
				if err != nil {
					return fmt.Errorf("validate %s: %w", tgz, err)
				}
			}
			if strings.TrimSpace(ns) == "" {
				return fmt.Errorf("compiled %s package in %s must set namespace — see %s and docs/package-model.md", kind, filepath.Base(tgz), pmPathInTar)
			}
			if err := domain.ValidateNamespace(ns); err != nil {
				return fmt.Errorf("%s: %w", tgz, err)
			}
			for _, dep := range pm.Depends {
				dep = strings.TrimSpace(dep)
				if dep == "" {
					continue
				}
				if !known[dep] {
					return fmt.Errorf("package %q depends on %q, which is not present in the compiled store — run dockpipe build so dependencies exist first", pm.Name, dep)
				}
			}
		}
	}
	return nil
}

func readPackageManifestFromTarballMembers(kind string, members []string, tgz string) (pathInTar string, pm *domain.PackageManifest, err error) {
	var wantSuffix string
	switch kind {
	case "workflows":
		wf, err := packagebuild.WorkflowNameFromTarballMembers(members)
		if err != nil {
			return "", nil, err
		}
		wantSuffix = filepath.ToSlash(filepath.Join("workflows", wf, infrastructure.PackageManifestFilename))
	case "resolvers":
		var resName string
		for _, m := range members {
			parts := strings.Split(m, "/")
			if len(parts) >= 3 && parts[0] == "resolvers" && parts[2] == infrastructure.PackageManifestFilename {
				resName = parts[1]
				break
			}
		}
		if resName == "" {
			return "", nil, fmt.Errorf("no resolvers/<name>/package.yml in archive")
		}
		wantSuffix = filepath.ToSlash(filepath.Join("resolvers", resName, infrastructure.PackageManifestFilename))
	default:
		return "", nil, fmt.Errorf("unknown kind %q", kind)
	}
	b, err := packagebuild.ReadFileFromTarGz(tgz, wantSuffix)
	if err != nil {
		return "", nil, err
	}
	var m domain.PackageManifest
	if err := yaml.Unmarshal(b, &m); err != nil {
		return "", nil, err
	}
	if err := domain.ValidatePackageManifest(&m); err != nil {
		return "", nil, err
	}
	return wantSuffix, &m, nil
}

func defaultNamespaceFromProjectConfig(cfg *domain.DockpipeProjectConfig) string {
	if cfg == nil || cfg.Packages.Namespace == nil {
		return ""
	}
	return strings.TrimSpace(*cfg.Packages.Namespace)
}

func effectiveWorkflowNamespace(pm *domain.PackageManifest, configYML []byte, cfg *domain.DockpipeProjectConfig) string {
	if s := strings.TrimSpace(pm.Namespace); s != "" {
		return s
	}
	var top struct {
		Namespace string `yaml:"namespace"`
	}
	if err := yaml.Unmarshal(configYML, &top); err == nil {
		if s := strings.TrimSpace(top.Namespace); s != "" {
			return s
		}
	}
	return defaultNamespaceFromProjectConfig(cfg)
}

func effectiveResolverNamespaceFromTar(pm *domain.PackageManifest, tgz string, members []string, cfg *domain.DockpipeProjectConfig) (string, error) {
	if s := strings.TrimSpace(pm.Namespace); s != "" {
		return s, nil
	}
	var resName string
	for _, m := range members {
		parts := strings.Split(m, "/")
		if len(parts) >= 2 && parts[0] == "resolvers" {
			resName = parts[1]
			break
		}
	}
	if resName == "" {
		return defaultNamespaceFromProjectConfig(cfg), nil
	}
	b, err := packagebuild.ReadFileFromTarGz(tgz, filepath.ToSlash(filepath.Join("resolvers", resName, "resolver.yaml")))
	if err != nil || len(b) == 0 {
		return defaultNamespaceFromProjectConfig(cfg), nil
	}
	var aux struct {
		Namespace string `yaml:"namespace"`
	}
	if err := yaml.Unmarshal(b, &aux); err != nil {
		return "", err
	}
	if err := domain.ValidateNamespace(aux.Namespace); err != nil {
		return "", err
	}
	if s := strings.TrimSpace(aux.Namespace); s != "" {
		return s, nil
	}
	return defaultNamespaceFromProjectConfig(cfg), nil
}

func compiledPackageNamesFromTarballs(pkgsRoot string) map[string]bool {
	out := make(map[string]bool)
	coreMatches, _ := filepath.Glob(filepath.Join(pkgsRoot, "core", "dockpipe-core-*.tar.gz"))
	sort.Strings(coreMatches)
	if len(coreMatches) > 0 {
		coreTar := coreMatches[len(coreMatches)-1]
		b, err := packagebuild.ReadFileFromTarGz(coreTar, filepath.ToSlash(filepath.Join("core", infrastructure.PackageManifestFilename)))
		if err == nil {
			var m domain.PackageManifest
			if yaml.Unmarshal(b, &m) == nil && strings.TrimSpace(m.Name) != "" {
				out[strings.TrimSpace(m.Name)] = true
			}
		}
	}
	for _, kind := range []string{"workflows", "resolvers"} {
		var pattern string
		switch kind {
		case "workflows":
			pattern = filepath.Join(pkgsRoot, "workflows", "dockpipe-workflow-*.tar.gz")
		case "resolvers":
			pattern = filepath.Join(pkgsRoot, "resolvers", "dockpipe-resolver-*.tar.gz")
		}
		matches, _ := filepath.Glob(pattern)
		for _, tgz := range matches {
			members, err := packagebuild.ListTarGzMemberPaths(tgz)
			if err != nil {
				continue
			}
			var suffix string
			switch kind {
			case "workflows":
				wf, err := packagebuild.WorkflowNameFromTarballMembers(members)
				if err != nil {
					continue
				}
				suffix = filepath.ToSlash(filepath.Join("workflows", wf, infrastructure.PackageManifestFilename))
			case "resolvers":
				for _, m := range members {
					parts := strings.Split(m, "/")
					if len(parts) >= 3 && parts[0] == "resolvers" && parts[2] == infrastructure.PackageManifestFilename {
						suffix = m
						break
					}
				}
			}
			if suffix == "" {
				continue
			}
			b, err := packagebuild.ReadFileFromTarGz(tgz, suffix)
			if err != nil {
				continue
			}
			var pm domain.PackageManifest
			if yaml.Unmarshal(b, &pm) != nil {
				continue
			}
			if n := strings.TrimSpace(pm.Name); n != "" {
				out[n] = true
			}
		}
	}
	return out
}
