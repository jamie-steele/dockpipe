package infrastructure

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"dockpipe"
)

// bundledFormatVersion bumps when extraction rules change (forces re-unpack; see .bundled-format).
const bundledFormatVersion = "105"

var bundledMu sync.Mutex

// EmbeddedWorkflowConfigExists reports whether a bundled workflow or resolver-delegate config exists for name.
// Checks embed paths src/core/workflows/<name>/config.yml and nested src/core/workflows/**/<name>/config.yml, src/core/resolvers/<name>/config.yml (plus workflows/ and embedded maintainer packages).
func EmbeddedWorkflowConfigExists(name string) bool {
	if name == "" {
		return false
	}
	for _, r := range name {
		isAlnum := (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')
		if !isAlnum && r != '-' && r != '_' && r != '.' {
			return false
		}
	}
	for _, p := range []string{
		EmbeddedTemplatesPrefix + "/workflows/" + name + "/config.yml",
		"workflows/" + name + "/config.yml",
		DorkpipeLibraryWorkflowsDirRel + "/" + name + "/config.yml",
		embeddedFSPackagesPrefix + "/" + name + "/config.yml",
		EmbeddedTemplatesPrefix + "/resolvers/" + name + "/config.yml",
	} {
		if _, err := fs.Stat(dockpipe.BundledFS, p); err == nil {
			return true
		}
	}
	if embeddedStagingWorkflowConfigExists(name) {
		return true
	}
	if embeddedBundledWorkflowsNestedConfigExists(name) {
		return true
	}
	return false
}

func embeddedBundledWorkflowsNestedConfigExists(name string) bool {
	found := false
	_ = fs.WalkDir(dockpipe.BundledFS, EmbeddedTemplatesPrefix+"/workflows", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		if d.IsDir() || d.Name() != "config.yml" {
			return nil
		}
		if filepath.Base(filepath.Dir(path)) != name {
			return nil
		}
		found = true
		return fs.SkipAll
	})
	return found
}

func embeddedStagingWorkflowConfigExists(name string) bool {
	found := false
	_ = fs.WalkDir(dockpipe.BundledFS, embeddedFSPackagesPrefix, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		if d.IsDir() || d.Name() != "config.yml" {
			return nil
		}
		if filepath.Base(filepath.Dir(path)) != name {
			return nil
		}
		found = true
		return fs.SkipAll
	})
	return found
}

// InvalidateBundledCache removes the on-disk bundle for this binary's VERSION so it will re-extract.
func InvalidateBundledCache() error {
	bundledMu.Lock()
	defer bundledMu.Unlock()
	versionBytes, err := fs.ReadFile(dockpipe.BundledFS, "VERSION")
	if err != nil {
		return fmt.Errorf("read embedded VERSION: %w", err)
	}
	ver := strings.TrimSpace(string(versionBytes))
	if ver == "" {
		return fmt.Errorf("embedded VERSION is empty")
	}
	cacheBase, err := bundledCacheBase()
	if err != nil {
		return err
	}
	dest := filepath.Join(cacheBase, ShipyardDir, "bundled-"+ver)
	return os.RemoveAll(dest)
}

// MaterializedBundledRoot returns a directory containing the unpacked bundle: <ShipyardDir>/core/
// (mirrors bundled category dirs: resolvers, runtimes, strategies, …), workflows/, assets/entrypoint.sh, and version.
// Embedded source uses src/core/...; copyEmbeddedFS maps that to the layout above on disk.
// See also DOCKPIPE_REPO_ROOT override in RepoRoot.
func MaterializedBundledRoot() (string, error) {
	bundledMu.Lock()
	defer bundledMu.Unlock()
	return extractBundledToCache()
}

func extractBundledToCache() (string, error) {
	versionBytes, err := fs.ReadFile(dockpipe.BundledFS, "VERSION")
	if err != nil {
		return "", fmt.Errorf("read embedded VERSION: %w", err)
	}
	ver := strings.TrimSpace(string(versionBytes))
	if ver == "" {
		return "", fmt.Errorf("embedded VERSION is empty")
	}

	cacheBase, err := bundledCacheBase()
	if err != nil {
		return "", err
	}
	dest := filepath.Join(cacheBase, ShipyardDir, "bundled-"+ver)
	cfgPath := filepath.Join(dest, ShipyardDir, "workflows", "run", "config.yml")
	formatPath := filepath.Join(dest, ".bundled-format")
	if st, err := os.Stat(cfgPath); err == nil && !st.IsDir() {
		if b, err := os.ReadFile(filepath.Join(dest, "version")); err == nil && strings.TrimSpace(string(b)) == ver {
			if fb, err := os.ReadFile(formatPath); err == nil && strings.TrimSpace(string(fb)) == bundledFormatVersion {
				return dest, nil
			}
		}
	}

	tmp := dest + ".tmp"
	_ = os.RemoveAll(tmp)
	if err := os.MkdirAll(tmp, 0o755); err != nil {
		return "", err
	}
	if err := copyEmbeddedFS(tmp, versionBytes); err != nil {
		_ = os.RemoveAll(tmp)
		return "", err
	}
	_ = os.RemoveAll(dest)
	if err := os.Rename(tmp, dest); err != nil {
		_ = os.RemoveAll(tmp)
		return "", err
	}
	if err := os.WriteFile(formatPath, []byte(bundledFormatVersion+"\n"), 0o644); err != nil {
		return "", err
	}
	return dest, nil
}

// bundledCacheBase is the parent directory for <ShipyardDir>/bundled-<version> (default: user cache dir).
// Set DOCKPIPE_BUNDLED_CACHE to override (tests, read-only home, etc.).
func bundledCacheBase() (string, error) {
	if v := os.Getenv("DOCKPIPE_BUNDLED_CACHE"); v != "" {
		return filepath.Abs(v)
	}
	cacheBase, err := os.UserCacheDir()
	if err != nil {
		return os.TempDir(), nil
	}
	return cacheBase, nil
}

// embedWorkflowRoot records a workflow directory under embeddedFSPackagesPrefix (any depth) for material
// shipyard/workflows/<name>/… mapping.
type embedWorkflowRoot struct {
	prefix string // e.g. dockpipe/ide/resolvers/vscode (no leading embedded packages prefix)
	name   string // workflow leaf basename (e.g. codex)
}

var (
	stagingEmbedRootsOnce sync.Once
	stagingEmbedRoots     []embedWorkflowRoot
)

func initStagingEmbedRoots() {
	stagingEmbedRootsOnce.Do(func() {
		seen := map[string]struct{}{}
		_ = fs.WalkDir(dockpipe.BundledFS, embeddedFSPackagesPrefix, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				if os.IsNotExist(err) {
					return nil
				}
				return err
			}
			if d.IsDir() || (d.Name() != "config.yml" && d.Name() != "profile") {
				return nil
			}
			rel := strings.TrimPrefix(path, embeddedFSPackagesPrefix)
			rel = strings.TrimPrefix(rel, "/")
			parent := filepath.Dir(rel)
			if parent == "." || parent == "" {
				return nil
			}
			name := filepath.Base(parent)
			if _, ok := seen[parent]; ok {
				return nil
			}
			seen[parent] = struct{}{}
			stagingEmbedRoots = append(stagingEmbedRoots, embedWorkflowRoot{prefix: parent, name: name})
			return nil
		})
		sort.Slice(stagingEmbedRoots, func(i, j int) bool {
			return len(stagingEmbedRoots[i].prefix) > len(stagingEmbedRoots[j].prefix)
		})
	})
}

// mapEmbeddedStagingWorkflowRel maps embedded packages/** to shipyard/workflows/<workflow>/… using
// discovered config.yml / profile roots (namespace nesting of any depth).
func mapEmbeddedStagingWorkflowRel(rel string) (string, bool) {
	pfx := embeddedFSPackagesPrefix
	if rel != pfx && !strings.HasPrefix(rel, pfx+"/") {
		return "", false
	}
	normalized := strings.TrimPrefix(rel, pfx)
	normalized = strings.TrimPrefix(normalized, "/")
	if normalized == "" {
		return filepath.Join(ShipyardDir, "workflows"), true
	}
	initStagingEmbedRoots()
	for _, r := range stagingEmbedRoots {
		if normalized == r.prefix || strings.HasPrefix(normalized, r.prefix+"/") {
			suffix := strings.TrimPrefix(normalized, r.prefix)
			suffix = strings.TrimPrefix(suffix, "/")
			if suffix == "" {
				return filepath.Join(ShipyardDir, "workflows", r.name), true
			}
			return filepath.Join(ShipyardDir, "workflows", r.name, filepath.FromSlash(suffix)), true
		}
	}
	return "", false
}

// mapEmbeddedToMaterializedPath maps embed paths (src/core/..., lib/..., VERSION) to the on-disk
// materialized layout: <ShipyardDir>/core/..., workflows/..., lib/, version.
func mapEmbeddedToMaterializedPath(rel string) string {
	wfUnderCore := EmbeddedTemplatesPrefix + "/workflows"
	switch {
	case rel == "VERSION":
		return "version"
	case rel == wfUnderCore || strings.HasPrefix(rel, wfUnderCore+"/"):
		rest := strings.TrimPrefix(rel, wfUnderCore)
		rest = strings.TrimPrefix(rest, "/")
		if rest == "" {
			return filepath.Join(ShipyardDir, "workflows")
		}
		return filepath.Join(ShipyardDir, "workflows", filepath.FromSlash(rest))
	case rel == EmbeddedTemplatesPrefix || strings.HasPrefix(rel, EmbeddedTemplatesPrefix+"/"):
		suffix := strings.TrimPrefix(rel, EmbeddedTemplatesPrefix)
		suffix = strings.TrimPrefix(suffix, "/")
		if suffix == "" {
			return filepath.Join(ShipyardDir, "core")
		}
		return filepath.Join(ShipyardDir, "core", filepath.FromSlash(suffix))
	case rel == "workflows" || strings.HasPrefix(rel, "workflows/"):
		rest := strings.TrimPrefix(rel, "workflows")
		rest = strings.TrimPrefix(rest, "/")
		if rest == "" {
			return filepath.Join(ShipyardDir, "workflows")
		}
		return filepath.Join(ShipyardDir, "workflows", rest)
	case rel == embeddedFSPackagesPrefix || strings.HasPrefix(rel, embeddedFSPackagesPrefix+"/"):
		if out, ok := mapEmbeddedStagingWorkflowRel(rel); ok {
			return out
		}
		rest := strings.TrimPrefix(rel, embeddedFSPackagesPrefix)
		rest = strings.TrimPrefix(rest, "/")
		if rest == "" {
			return filepath.Join(ShipyardDir, "workflows")
		}
		return filepath.Join(ShipyardDir, "workflows", filepath.FromSlash(rest))
	case rel == DorkpipeLibraryWorkflowsDirRel || strings.HasPrefix(rel, DorkpipeLibraryWorkflowsDirRel+"/"):
		rest := strings.TrimPrefix(rel, DorkpipeLibraryWorkflowsDirRel)
		rest = strings.TrimPrefix(rest, "/")
		if rest == "" {
			return filepath.Join(ShipyardDir, "workflows")
		}
		return filepath.Join(ShipyardDir, "workflows", filepath.FromSlash(rest))
	default:
		return rel
	}
}

func copyEmbeddedFS(dstRoot string, versionBytes []byte) error {
	return fs.WalkDir(dockpipe.BundledFS, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == "." {
			return nil
		}
		outRel := mapEmbeddedToMaterializedPath(path)
		out := filepath.Join(dstRoot, filepath.FromSlash(outRel))
		if d.IsDir() {
			return os.MkdirAll(out, 0o755)
		}
		data, err := fs.ReadFile(dockpipe.BundledFS, path)
		if err != nil {
			return err
		}
		if path == "VERSION" {
			data = versionBytes
		}
		if strings.HasSuffix(path, ".sh") {
			data = normalizeShellScript(data)
		}
		if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
			return err
		}
		mode := fs.FileMode(0o644)
		if strings.HasSuffix(path, ".sh") {
			mode = 0o755
		}
		return os.WriteFile(out, data, mode)
	})
}

// normalizeShellScript strips UTF-8 BOM and CRLF so Git-Bash can source scripts on Windows.
func normalizeShellScript(data []byte) []byte {
	data = bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})
	data = bytes.ReplaceAll(data, []byte("\r\n"), []byte("\n"))
	data = bytes.ReplaceAll(data, []byte("\r"), []byte("\n"))
	return data
}
