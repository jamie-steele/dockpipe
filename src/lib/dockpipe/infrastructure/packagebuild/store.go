package packagebuild

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"dockpipe/src/lib/dockpipe/domain"
)

// StoreBuildManifest is written next to compiled-package tarballs (packages-store-manifest.json).
type StoreBuildManifest struct {
	Schema    int    `json:"schema"`
	StoreRoot string `json:"store_root,omitempty"`
	Packages  struct {
		Core      *StoreArtifact   `json:"core,omitempty"`
		Workflows []StoreArtifact  `json:"workflows,omitempty"`
		Resolvers []StoreArtifact  `json:"resolvers,omitempty"`
		Bundles   []StoreArtifact  `json:"bundles,omitempty"`
	} `json:"packages"`
}

// StoreArtifact names one gzip tarball built from a compiled package directory.
type StoreArtifact struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Tarball string `json:"tarball"`
	SHA256  string `json:"sha256"`
}

// BuildCompiledStore writes dockpipe-*.tar.gz (+ .sha256) for each slice under packagesRoot and
// packages-store-manifest.json under outDir. fallbackVersion is used when package.yml omits version.
// only is "all" or one of: core, workflows, resolvers, bundles.
func BuildCompiledStore(packagesRoot, outDir, fallbackVersion, only string) (*StoreBuildManifest, error) {
	packagesRoot = filepath.Clean(packagesRoot)
	outDir = filepath.Clean(outDir)
	if fallbackVersion == "" {
		fallbackVersion = "0.0.0"
	}
	only = strings.TrimSpace(strings.ToLower(only))
	if only == "" {
		only = "all"
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return nil, err
	}
	m := &StoreBuildManifest{Schema: 1}
	m.StoreRoot = packagesRoot

	if only == "all" || only == "core" {
		coreDir := filepath.Join(packagesRoot, "core")
		if st, err := os.Stat(coreDir); err == nil && st.IsDir() {
			ver := fallbackVersion
			if pm, err := domain.ParsePackageManifest(filepath.Join(coreDir, "package.yml")); err == nil && strings.TrimSpace(pm.Version) != "" {
				ver = strings.TrimSpace(pm.Version)
			}
			base := fmt.Sprintf("dockpipe-core-%s.tar.gz", safeTarballToken(ver))
			outPath := filepath.Join(outDir, base)
			sum, err := WriteDirTarGzWithPrefix(coreDir, outPath, "core")
			if err != nil {
				return nil, fmt.Errorf("core: %w", err)
			}
			m.Packages.Core = &StoreArtifact{Name: "core", Version: ver, Tarball: base, SHA256: sum}
		}
	}

	wfDir := filepath.Join(packagesRoot, "workflows")
	if only == "all" || only == "workflows" {
		names, err := listPackageSubdirs(wfDir)
		if err != nil {
			return nil, err
		}
		for _, name := range names {
			dir := filepath.Join(wfDir, name)
			ver := readPackageVersion(dir, fallbackVersion)
			base := fmt.Sprintf("dockpipe-workflow-%s-%s.tar.gz", safeTarballToken(name), safeTarballToken(ver))
			outPath := filepath.Join(outDir, base)
			prefix := "workflows/" + name
			sum, err := WriteDirTarGzWithPrefix(dir, outPath, prefix)
			if err != nil {
				return nil, fmt.Errorf("workflow %q: %w", name, err)
			}
			m.Packages.Workflows = append(m.Packages.Workflows, StoreArtifact{Name: name, Version: ver, Tarball: base, SHA256: sum})
		}
		sort.Slice(m.Packages.Workflows, func(i, j int) bool { return m.Packages.Workflows[i].Name < m.Packages.Workflows[j].Name })
	}

	resDir := filepath.Join(packagesRoot, "resolvers")
	if only == "all" || only == "resolvers" {
		names, err := listPackageSubdirs(resDir)
		if err != nil {
			return nil, err
		}
		for _, name := range names {
			dir := filepath.Join(resDir, name)
			ver := readPackageVersion(dir, fallbackVersion)
			base := fmt.Sprintf("dockpipe-resolver-%s-%s.tar.gz", safeTarballToken(name), safeTarballToken(ver))
			outPath := filepath.Join(outDir, base)
			prefix := "resolvers/" + name
			sum, err := WriteDirTarGzWithPrefix(dir, outPath, prefix)
			if err != nil {
				return nil, fmt.Errorf("resolver %q: %w", name, err)
			}
			m.Packages.Resolvers = append(m.Packages.Resolvers, StoreArtifact{Name: name, Version: ver, Tarball: base, SHA256: sum})
		}
		sort.Slice(m.Packages.Resolvers, func(i, j int) bool { return m.Packages.Resolvers[i].Name < m.Packages.Resolvers[j].Name })
	}

	bunDir := filepath.Join(packagesRoot, "bundles")
	if only == "all" || only == "bundles" {
		names, err := listPackageSubdirs(bunDir)
		if err != nil {
			return nil, err
		}
		for _, name := range names {
			dir := filepath.Join(bunDir, name)
			ver := readPackageVersion(dir, fallbackVersion)
			base := fmt.Sprintf("dockpipe-bundle-%s-%s.tar.gz", safeTarballToken(name), safeTarballToken(ver))
			outPath := filepath.Join(outDir, base)
			prefix := "bundles/" + name
			sum, err := WriteDirTarGzWithPrefix(dir, outPath, prefix)
			if err != nil {
				return nil, fmt.Errorf("bundle %q: %w", name, err)
			}
			m.Packages.Bundles = append(m.Packages.Bundles, StoreArtifact{Name: name, Version: ver, Tarball: base, SHA256: sum})
		}
		sort.Slice(m.Packages.Bundles, func(i, j int) bool { return m.Packages.Bundles[i].Name < m.Packages.Bundles[j].Name })
	}

	if m.Packages.Core == nil && len(m.Packages.Workflows) == 0 && len(m.Packages.Resolvers) == 0 && len(m.Packages.Bundles) == 0 {
		return nil, fmt.Errorf("no compiled packages under %s — run `dockpipe build` (or `dockpipe package compile all`) first", packagesRoot)
	}

	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return nil, err
	}
	b = append(b, '\n')
	manifestPath := filepath.Join(outDir, "packages-store-manifest.json")
	if err := os.WriteFile(manifestPath, b, 0o644); err != nil {
		return nil, err
	}
	return m, nil
}

func readPackageVersion(dir, fallback string) string {
	p := filepath.Join(dir, "package.yml")
	pm, err := domain.ParsePackageManifest(p)
	if err != nil {
		return fallback
	}
	v := strings.TrimSpace(pm.Version)
	if v == "" {
		return fallback
	}
	return v
}

func listPackageSubdirs(root string) ([]string, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		names = append(names, e.Name())
	}
	sort.Strings(names)
	return names, nil
}

// safeTarballToken replaces characters unsafe in filenames for release artifacts.
func safeTarballToken(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '.', r == '_', r == '-':
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
	}
	out := b.String()
	if out == "" {
		return "unknown"
	}
	return out
}
