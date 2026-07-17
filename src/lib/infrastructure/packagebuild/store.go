package packagebuild

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"dockpipe/src/lib/domain"
	"gopkg.in/yaml.v3"
)

// StoreBuildManifest is written next to compiled-package tarballs (packages-store-manifest.json).
type StoreBuildManifest struct {
	Schema    int    `json:"schema"`
	StoreRoot string `json:"store_root,omitempty"`
	Packages  struct {
		Core      *StoreArtifact  `json:"core,omitempty"`
		Workflows []StoreArtifact `json:"workflows,omitempty"`
		Resolvers []StoreArtifact `json:"resolvers,omitempty"`
	} `json:"packages"`
}

// StoreArtifact names one gzip tarball built from a compiled package directory.
type StoreArtifact struct {
	Name                 string   `json:"name"`
	Version              string   `json:"version"`
	Tarball              string   `json:"tarball"`
	SHA256               string   `json:"sha256"`
	Provider             string   `json:"provider,omitempty"`
	Capability           string   `json:"capability,omitempty"`
	RequiresCapabilities []string `json:"requires_capabilities,omitempty"`
}

// BuildCompiledStore writes dockpipe-*.tar.gz (+ .sha256) for each slice under packagesRoot and
// packages-store-manifest.json under outDir. fallbackVersion is used when package.yml omits version.
// only is "all" (core + workflows + resolvers) or one of: core, workflows, resolvers.
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
			if artifact, err := buildOrCopyCoreArtifact(coreDir, outDir, fallbackVersion); err != nil {
				return nil, fmt.Errorf("core: %w", err)
			} else if artifact != nil {
				m.Packages.Core = artifact
			}
		}
	}

	wfDir := filepath.Join(packagesRoot, "workflows")
	if only == "all" || only == "workflows" {
		artifacts, err := buildOrCopyWorkflowArtifacts(wfDir, outDir, fallbackVersion)
		if err != nil {
			return nil, err
		}
		m.Packages.Workflows = append(m.Packages.Workflows, artifacts...)
		sort.Slice(m.Packages.Workflows, func(i, j int) bool { return m.Packages.Workflows[i].Name < m.Packages.Workflows[j].Name })
	}

	resDir := filepath.Join(packagesRoot, "resolvers")
	if only == "all" || only == "resolvers" {
		artifacts, err := buildOrCopyResolverArtifacts(resDir, outDir, fallbackVersion)
		if err != nil {
			return nil, err
		}
		m.Packages.Resolvers = append(m.Packages.Resolvers, artifacts...)
		sort.Slice(m.Packages.Resolvers, func(i, j int) bool { return m.Packages.Resolvers[i].Name < m.Packages.Resolvers[j].Name })
	}

	if m.Packages.Core == nil && len(m.Packages.Workflows) == 0 && len(m.Packages.Resolvers) == 0 {
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

// packageManifestMeta aggregates package.yml fields for store manifests and tooling.
type packageManifestMeta struct {
	Version              string
	Provider             string
	Capability           string
	RequiresCapabilities []string
}

func readPackageManifestMeta(dir, fallback string) packageManifestMeta {
	var out packageManifestMeta
	out.Version = fallback
	p := filepath.Join(dir, "package.yml")
	pm, err := domain.ParsePackageManifest(p)
	if err != nil {
		return out
	}
	if strings.TrimSpace(pm.Version) != "" {
		out.Version = strings.TrimSpace(pm.Version)
	}
	out.Provider = strings.TrimSpace(pm.Provider)
	out.Capability = strings.TrimSpace(pm.Capability)
	if len(pm.RequiresCapabilities) > 0 {
		out.RequiresCapabilities = append([]string(nil), pm.RequiresCapabilities...)
	}
	return out
}

func buildOrCopyCoreArtifact(coreDir, outDir, fallback string) (*StoreArtifact, error) {
	if paths, err := listTarballs(coreDir, "dockpipe-core-*.tar.gz"); err != nil {
		return nil, err
	} else if len(paths) > 0 {
		return copyTarballArtifact(paths[0], outDir, fallback, tarballKindCore)
	}
	meta := readPackageManifestMeta(coreDir, fallback)
	ver := meta.Version
	base := fmt.Sprintf("dockpipe-core-%s.tar.gz", SafeTarballToken(ver))
	outPath := filepath.Join(outDir, base)
	sum, err := WriteDirTarGzWithPrefix(coreDir, outPath, "core")
	if err != nil {
		return nil, err
	}
	return &StoreArtifact{Name: "core", Version: ver, Tarball: base, SHA256: sum, Provider: meta.Provider, Capability: meta.Capability}, nil
}

func buildOrCopyWorkflowArtifacts(root, outDir, fallback string) ([]StoreArtifact, error) {
	if paths, err := listTarballs(root, "dockpipe-workflow-*.tar.gz"); err != nil {
		return nil, err
	} else if len(paths) > 0 {
		var out []StoreArtifact
		for _, path := range paths {
			artifact, err := copyTarballArtifact(path, outDir, fallback, tarballKindWorkflow)
			if err != nil {
				return nil, fmt.Errorf("workflow tarball %q: %w", filepath.Base(path), err)
			}
			out = append(out, *artifact)
		}
		return out, nil
	}
	names, err := listPackageSubdirs(root)
	if err != nil {
		return nil, err
	}
	var out []StoreArtifact
	for _, name := range names {
		dir := filepath.Join(root, name)
		meta := readPackageManifestMeta(dir, fallback)
		ver := meta.Version
		base := fmt.Sprintf("dockpipe-workflow-%s-%s.tar.gz", SafeTarballToken(name), SafeTarballToken(ver))
		outPath := filepath.Join(outDir, base)
		prefix := "workflows/" + name
		sum, err := WriteDirTarGzWithPrefix(dir, outPath, prefix)
		if err != nil {
			return nil, fmt.Errorf("workflow %q: %w", name, err)
		}
		out = append(out, StoreArtifact{
			Name: name, Version: ver, Tarball: base, SHA256: sum,
			Provider: meta.Provider, Capability: meta.Capability, RequiresCapabilities: meta.RequiresCapabilities,
		})
	}
	return out, nil
}

func buildOrCopyResolverArtifacts(root, outDir, fallback string) ([]StoreArtifact, error) {
	if paths, err := listTarballs(root, "dockpipe-resolver-*.tar.gz"); err != nil {
		return nil, err
	} else if len(paths) > 0 {
		var out []StoreArtifact
		for _, path := range paths {
			artifact, err := copyTarballArtifact(path, outDir, fallback, tarballKindResolver)
			if err != nil {
				return nil, fmt.Errorf("resolver tarball %q: %w", filepath.Base(path), err)
			}
			out = append(out, *artifact)
		}
		return out, nil
	}
	names, err := listPackageSubdirs(root)
	if err != nil {
		return nil, err
	}
	var out []StoreArtifact
	for _, name := range names {
		dir := filepath.Join(root, name)
		meta := readPackageManifestMeta(dir, fallback)
		ver := meta.Version
		base := fmt.Sprintf("dockpipe-resolver-%s-%s.tar.gz", SafeTarballToken(name), SafeTarballToken(ver))
		outPath := filepath.Join(outDir, base)
		prefix := "resolvers/" + name
		sum, err := WriteDirTarGzWithPrefix(dir, outPath, prefix)
		if err != nil {
			return nil, fmt.Errorf("resolver %q: %w", name, err)
		}
		out = append(out, StoreArtifact{
			Name: name, Version: ver, Tarball: base, SHA256: sum,
			Provider: meta.Provider, Capability: meta.Capability,
		})
	}
	return out, nil
}

type tarballKind string

const (
	tarballKindCore     tarballKind = "core"
	tarballKindWorkflow tarballKind = "workflow"
	tarballKindResolver tarballKind = "resolver"
)

func copyTarballArtifact(srcPath, outDir, fallback string, kind tarballKind) (*StoreArtifact, error) {
	meta, name, err := readPackageManifestMetaFromTarball(srcPath, fallback, kind)
	if err != nil {
		return nil, err
	}
	base := filepath.Base(srcPath)
	outPath := filepath.Join(outDir, base)
	sum, err := copyTarballWithSHA(srcPath, outPath)
	if err != nil {
		return nil, err
	}
	return &StoreArtifact{
		Name:                 name,
		Version:              meta.Version,
		Tarball:              base,
		SHA256:               sum,
		Provider:             meta.Provider,
		Capability:           meta.Capability,
		RequiresCapabilities: meta.RequiresCapabilities,
	}, nil
}

func readPackageManifestMetaFromTarball(tarGzPath, fallback string, kind tarballKind) (packageManifestMeta, string, error) {
	var out packageManifestMeta
	out.Version = fallback
	members, err := ListTarGzMemberPaths(tarGzPath)
	if err != nil {
		return out, "", err
	}
	manifestPath, name, err := packageManifestPathForTarball(kind, members)
	if err != nil {
		return out, "", err
	}
	b, err := ReadFileFromTarGz(tarGzPath, manifestPath)
	if err != nil {
		return out, "", err
	}
	var pm domain.PackageManifest
	if err := yaml.Unmarshal(b, &pm); err != nil {
		return out, "", err
	}
	if strings.TrimSpace(pm.Version) != "" {
		out.Version = strings.TrimSpace(pm.Version)
	}
	out.Provider = strings.TrimSpace(pm.Provider)
	out.Capability = strings.TrimSpace(pm.Capability)
	if len(pm.RequiresCapabilities) > 0 {
		out.RequiresCapabilities = append([]string(nil), pm.RequiresCapabilities...)
	}
	if strings.TrimSpace(pm.Name) != "" && kind == tarballKindCore {
		name = strings.TrimSpace(pm.Name)
	}
	return out, name, nil
}

func packageManifestPathForTarball(kind tarballKind, members []string) (string, string, error) {
	switch kind {
	case tarballKindCore:
		for _, member := range members {
			if member == "core/package.yml" {
				return member, "core", nil
			}
		}
	case tarballKindWorkflow:
		for _, member := range members {
			if strings.HasPrefix(member, "workflows/") && strings.HasSuffix(member, "/package.yml") {
				parts := strings.Split(member, "/")
				if len(parts) >= 3 {
					return member, parts[1], nil
				}
			}
		}
	case tarballKindResolver:
		for _, member := range members {
			if strings.HasPrefix(member, "resolvers/") && strings.HasSuffix(member, "/package.yml") {
				parts := strings.Split(member, "/")
				if len(parts) >= 3 {
					return member, parts[1], nil
				}
			}
		}
	}
	return "", "", fmt.Errorf("no package manifest found in %s tarball", kind)
}

func copyTarballWithSHA(srcPath, outPath string) (string, error) {
	b, err := os.ReadFile(srcPath)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	sumHex := hex.EncodeToString(sum[:])
	if err := os.WriteFile(outPath, b, 0o644); err != nil {
		return "", err
	}
	if err := os.WriteFile(outPath+".sha256", []byte(sumHex+"\n"), 0o644); err != nil {
		return "", err
	}
	return sumHex, nil
}

func listTarballs(root, pattern string) ([]string, error) {
	matches, err := filepath.Glob(filepath.Join(root, pattern))
	if err != nil {
		return nil, err
	}
	sort.Strings(matches)
	return matches, nil
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

// UnmarshalJSON accepts legacy "primitive" / "requires_primitives" keys for older packages-store-manifest.json.
func (a *StoreArtifact) UnmarshalJSON(data []byte) error {
	var x struct {
		Name                 string   `json:"name"`
		Version              string   `json:"version"`
		Tarball              string   `json:"tarball"`
		SHA256               string   `json:"sha256"`
		Provider             string   `json:"provider,omitempty"`
		Capability           string   `json:"capability,omitempty"`
		RequiresCapabilities []string `json:"requires_capabilities,omitempty"`
		Primitive            string   `json:"primitive,omitempty"`
		RequiresPrimitives   []string `json:"requires_primitives,omitempty"`
	}
	if err := json.Unmarshal(data, &x); err != nil {
		return err
	}
	a.Name = x.Name
	a.Version = x.Version
	a.Tarball = x.Tarball
	a.SHA256 = x.SHA256
	a.Provider = x.Provider
	a.Capability = strings.TrimSpace(x.Capability)
	if a.Capability == "" {
		a.Capability = strings.TrimSpace(x.Primitive)
	}
	a.RequiresCapabilities = x.RequiresCapabilities
	if len(a.RequiresCapabilities) == 0 && len(x.RequiresPrimitives) > 0 {
		a.RequiresCapabilities = append([]string(nil), x.RequiresPrimitives...)
	}
	return nil
}

// SafeTarballToken replaces characters unsafe in filenames for release artifacts.
func SafeTarballToken(s string) string {
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
