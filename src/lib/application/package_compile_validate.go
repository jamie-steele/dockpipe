package application

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"

	"dockpipe/src/lib/domain"
	"dockpipe/src/lib/infrastructure"
	"dockpipe/src/lib/infrastructure/packagebuild"

	"gopkg.in/yaml.v3"
)

// validateCompileOutputs checks workflow and resolver tarballs under the compiled store for namespace
// and depends closure (compile order: core → resolvers → workflows). Local compiled workflows may omit
// namespace unless requireWorkflowNamespace is true (store/export path).
func validateCompileOutputsForMode(workdir string, requireWorkflowNamespace bool) error {
	return validateCompileOutputsScoped(workdir, requireWorkflowNamespace, nil, nil)
}

func validateCompileOutputsScoped(workdir string, requireWorkflowNamespace bool, workflowNames map[string]bool, resolverNames map[string]bool) error {
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
	mergeCompiledPackageNamesFromCompileRoots(known, repoRoot, cfg)
	mergeCompiledPackageNamesFromConfiguredSources(known, repoRoot, cfg)
	mergeCompiledPackageNamesFromInstalledSources(known)
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
			if !shouldValidateCompiledTarball(kind, tgz, workflowNames, resolverNames) {
				continue
			}
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
				if err := validateWorkflowConfigInTarball(tgz, cfgPath); err != nil {
					return fmt.Errorf("validate %s: %w", tgz, err)
				}
				ns = effectiveWorkflowNamespace(pm, b, cfg)
			case "resolvers":
				for _, m := range members {
					if !compiledPackageWorkflowConfigEntry(m) {
						continue
					}
					if err := validateWorkflowConfigInTarball(tgz, m); err != nil {
						return fmt.Errorf("validate %s: %w", tgz, err)
					}
				}
				ns, err = effectiveResolverNamespaceFromTar(pm, tgz, members, cfg)
				if err != nil {
					return fmt.Errorf("validate %s: %w", tgz, err)
				}
			}
			ns = strings.TrimSpace(ns)
			if ns == "" {
				if kind == "workflows" && !requireWorkflowNamespace {
					// Local compiled workflow tarballs are allowed to omit namespace.
					// Namespace remains relevant for store-facing resolution and packaged workflow selection.
				} else {
					return fmt.Errorf("compiled %s package in %s must set namespace — see %s and docs/packages/package-model.md", kind, filepath.Base(tgz), pmPathInTar)
				}
			} else if err := domain.ValidateNamespace(ns); err != nil {
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

func mergeCompiledPackageNamesFromCompileRoots(out map[string]bool, repoRoot string, cfg *domain.DockpipeProjectConfig) {
	for _, root := range effectiveWorkflowCompileRoots(cfg, repoRoot) {
		_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				if filepath.Clean(path) == filepath.Clean(root) {
					return nil
				}
				return nil
			}
			if d.IsDir() || filepath.Base(path) != infrastructure.PackageManifestFilename {
				return nil
			}
			pm, err := domain.ParsePackageManifest(path)
			if err != nil {
				return nil
			}
			if name := strings.TrimSpace(pm.Name); name != "" {
				out[name] = true
			}
			return nil
		})
	}
}

func mergeCompiledPackageNamesFromConfiguredSources(out map[string]bool, repoRoot string, cfg *domain.DockpipeProjectConfig) {
	if cfg == nil || cfg.Packages.Sources == nil {
		return
	}
	for _, src := range *cfg.Packages.Sources {
		path := strings.TrimSpace(src.Path)
		if path == "" {
			continue
		}
		kind := strings.ToLower(strings.TrimSpace(src.Kind))
		if kind == "" {
			kind = domain.PackageSourceKindStore
		}
		resolved := path
		if !filepath.IsAbs(resolved) {
			resolved = filepath.Join(repoRoot, filepath.Clean(resolved))
		}
		switch kind {
		case domain.PackageSourceKindStore:
			for name := range compiledPackageNamesFromTarballs(resolved) {
				out[name] = true
			}
		case domain.PackageSourceKindTarballDir:
			for name := range compiledPackageNamesFromTarballDir(resolved) {
				out[name] = true
			}
		}
	}
}

func mergeCompiledPackageNamesFromInstalledSources(out map[string]bool) {
	if globalRoot, err := infrastructure.GlobalPackagesRoot(); err == nil {
		for name := range compiledPackageNamesFromTarballs(globalRoot) {
			out[name] = true
		}
	}
	for _, root := range infrastructure.SystemPackagesRoots() {
		for name := range compiledPackageNamesFromTarballs(root) {
			out[name] = true
		}
	}
}

func shouldValidateCompiledTarball(kind, tgz string, workflowNames, resolverNames map[string]bool) bool {
	base := filepath.Base(tgz)
	switch kind {
	case "workflows":
		if len(workflowNames) == 0 {
			return true
		}
		name := strings.TrimSuffix(strings.TrimPrefix(base, "dockpipe-workflow-"), ".tar.gz")
		if idx := strings.LastIndex(name, "-"); idx > 0 {
			name = name[:idx]
		}
		return workflowNames[name]
	case "resolvers":
		if len(resolverNames) == 0 {
			return true
		}
		name := strings.TrimSuffix(strings.TrimPrefix(base, "dockpipe-resolver-"), ".tar.gz")
		if idx := strings.LastIndex(name, "-"); idx > 0 {
			name = name[:idx]
		}
		return resolverNames[name]
	default:
		return true
	}
}

func validateCompileOutputs(workdir string) error {
	return validateCompileOutputsForMode(workdir, false)
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

func compiledPackageNamesFromTarballDir(dir string) map[string]bool {
	out := make(map[string]bool)
	for _, pattern := range []string{
		"dockpipe-core-*.tar.gz",
		"dockpipe-workflow-*.tar.gz",
		"dockpipe-resolver-*.tar.gz",
	} {
		matches, _ := filepath.Glob(filepath.Join(dir, pattern))
		for _, tgz := range matches {
			base := filepath.Base(tgz)
			switch {
			case strings.HasPrefix(base, "dockpipe-core-"):
				b, err := packagebuild.ReadFileFromTarGz(tgz, filepath.ToSlash(filepath.Join("core", infrastructure.PackageManifestFilename)))
				if err != nil {
					continue
				}
				var pm domain.PackageManifest
				if yaml.Unmarshal(b, &pm) == nil {
					if n := strings.TrimSpace(pm.Name); n != "" {
						out[n] = true
					}
				}
			case strings.HasPrefix(base, "dockpipe-workflow-"):
				members, err := packagebuild.ListTarGzMemberPaths(tgz)
				if err != nil {
					continue
				}
				wf, err := packagebuild.WorkflowNameFromTarballMembers(members)
				if err != nil {
					continue
				}
				suffix := filepath.ToSlash(filepath.Join("workflows", wf, infrastructure.PackageManifestFilename))
				b, err := packagebuild.ReadFileFromTarGz(tgz, suffix)
				if err != nil {
					continue
				}
				var pm domain.PackageManifest
				if yaml.Unmarshal(b, &pm) == nil {
					if n := strings.TrimSpace(pm.Name); n != "" {
						out[n] = true
					}
				}
			case strings.HasPrefix(base, "dockpipe-resolver-"):
				members, err := packagebuild.ListTarGzMemberPaths(tgz)
				if err != nil {
					continue
				}
				var suffix string
				for _, m := range members {
					parts := strings.Split(m, "/")
					if len(parts) >= 3 && parts[0] == "resolvers" && parts[2] == infrastructure.PackageManifestFilename {
						suffix = m
						break
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
				if yaml.Unmarshal(b, &pm) == nil {
					if n := strings.TrimSpace(pm.Name); n != "" {
						out[n] = true
					}
				}
			}
		}
	}
	return out
}
