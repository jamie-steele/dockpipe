package infrastructure

import (
	"path/filepath"
	"strings"

	"dockpipe/src/lib/domain"
)

func packageSourcesConfigRoot(repoRoot, workdir string) string {
	workdir = strings.TrimSpace(workdir)
	if workdir != "" {
		if root, err := domain.FindProjectRootWithDockpipeConfig(workdir); err == nil && strings.TrimSpace(root) != "" {
			return root
		}
	}
	return strings.TrimSpace(repoRoot)
}

type configuredPackageSource struct {
	kind string
	path string
}

func configuredPackageSources(projectRoot string) []configuredPackageSource {
	projectRoot = strings.TrimSpace(projectRoot)
	if projectRoot == "" {
		return nil
	}
	cfg, err := domain.LoadDockpipeProjectConfig(projectRoot)
	if err != nil || cfg == nil || cfg.Packages.Sources == nil {
		return nil
	}
	var out []configuredPackageSource
	seen := make(map[string]struct{})
	for _, src := range *cfg.Packages.Sources {
		path := strings.TrimSpace(src.Path)
		if path == "" {
			continue
		}
		kind := strings.ToLower(strings.TrimSpace(src.Kind))
		if kind == "" {
			kind = domain.PackageSourceKindStore
		}
		resolved := resolveProjectConfigPath(projectRoot, path)
		key := kind + "\x00" + resolved
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, configuredPackageSource{
			kind: kind,
			path: resolved,
		})
	}
	return out
}

func configuredPackageStoreRoots(projectRoot string) []string {
	var out []string
	for _, src := range configuredPackageSources(projectRoot) {
		if src.kind == domain.PackageSourceKindStore {
			out = append(out, src.path)
		}
	}
	return out
}

func configuredPackageTarballDirs(projectRoot string) []string {
	var out []string
	for _, src := range configuredPackageSources(projectRoot) {
		if src.kind == domain.PackageSourceKindTarballDir {
			out = append(out, src.path)
		}
	}
	return out
}

func resolveProjectConfigPath(projectRoot, raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if filepath.IsAbs(raw) {
		return filepath.Clean(raw)
	}
	return filepath.Join(projectRoot, filepath.Clean(raw))
}
