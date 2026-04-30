package infrastructure

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	pathpkg "path"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"dockpipe"
)

// bundledFormatVersion bumps when extraction rules or embedded workflow wiring change
// and a stale on-disk bundle must be re-extracted (see .bundled-format).
const bundledFormatVersion = "114"

var bundledMu sync.Mutex

// EmbeddedWorkflowConfigExists reports whether a bundled workflow or resolver-delegate config exists for name.
// Checks embed paths src/core/workflows/<name>/config.yml and nested src/core/workflows/**/<name>/config.yml, src/core/resolvers/<name>/config.yml (plus workflows/ and embedded maintainer roots — see embeddedPackageRootsPrefixes).
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
		EmbeddedTemplatesPrefix + "/resolvers/" + name + "/config.yml",
	} {
		if _, err := fs.Stat(dockpipe.BundledFS, p); err == nil {
			return true
		}
	}
	for _, pfx := range embeddedPackageRootsPrefixes {
		if _, err := fs.Stat(dockpipe.BundledFS, pfx+"/"+name+"/config.yml"); err == nil {
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
		if pathpkg.Base(pathpkg.Dir(path)) != name {
			return nil
		}
		found = true
		return fs.SkipAll
	})
	return found
}

func embeddedStagingWorkflowConfigExists(name string) bool {
	found := false
	_ = fs.WalkDir(dockpipe.BundledFS, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		if !embeddedPathHasPackageRootPrefix(path) {
			return nil
		}
		if d.IsDir() || d.Name() != "config.yml" {
			return nil
		}
		if pathpkg.Base(pathpkg.Dir(path)) != name {
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
	dest := filepath.Join(cacheBase, BundledCacheParentDir, "bundled-"+ver)
	return os.RemoveAll(dest)
}

// MaterializedBundledRoot returns a directory containing the unpacked bundle: <BundledLayoutDir>/core/
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
	dest := filepath.Join(cacheBase, BundledCacheParentDir, "bundled-"+ver)
	cfgPath := filepath.Join(dest, BundledLayoutDir, "workflows", "run", "config.yml")
	formatPath := filepath.Join(dest, ".bundled-format")
	fingerprintPath := filepath.Join(dest, ".bundled-fingerprint")
	fingerprint, err := embeddedBundleFingerprint()
	if err != nil {
		return "", err
	}
	if st, err := os.Stat(cfgPath); err == nil && !st.IsDir() {
		if b, err := os.ReadFile(filepath.Join(dest, "version")); err == nil && strings.TrimSpace(string(b)) == ver {
			if fb, err := os.ReadFile(formatPath); err == nil && strings.TrimSpace(string(fb)) == bundledFormatVersion {
				if fpb, err := os.ReadFile(fingerprintPath); err == nil && strings.TrimSpace(string(fpb)) == fingerprint {
					return dest, nil
				}
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
	if err := os.WriteFile(fingerprintPath, []byte(fingerprint+"\n"), 0o644); err != nil {
		return "", err
	}
	return dest, nil
}

func embeddedBundleFingerprint() (string, error) {
	var paths []string
	if err := fs.WalkDir(dockpipe.BundledFS, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		paths = append(paths, path)
		return nil
	}); err != nil {
		return "", fmt.Errorf("walk embedded bundle: %w", err)
	}
	sort.Strings(paths)
	h := sha256.New()
	for _, path := range paths {
		b, err := fs.ReadFile(dockpipe.BundledFS, path)
		if err != nil {
			return "", fmt.Errorf("read embedded bundle %s: %w", path, err)
		}
		_, _ = h.Write([]byte(path))
		_, _ = h.Write([]byte{0})
		_, _ = h.Write(b)
		_, _ = h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// bundledCacheBase is the parent directory for <cache>/<BundledCacheParentDir>/bundled-<version> (default: user cache dir).
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

// embedWorkflowRoot records a workflow directory under embeddedPackageRootsPrefixes (any depth) for material
// bundle/workflows/<name>/… mapping.
type embedWorkflowRoot struct {
	prefix string // e.g. ide/resolvers/vscode under an embedded package root (no leading embed root)
	name   string // workflow leaf basename (e.g. codex)
}

var (
	embeddedWorkflowRootsOnce sync.Once
	embeddedWorkflowRoots     []embedWorkflowRoot
)

func initEmbeddedWorkflowRoots() {
	embeddedWorkflowRootsOnce.Do(func() {
		seen := map[string]struct{}{}
		_ = fs.WalkDir(dockpipe.BundledFS, ".", func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				if os.IsNotExist(err) {
					return nil
				}
				return err
			}
			if !embeddedPathHasPackageRootPrefix(path) {
				return nil
			}
			if d.IsDir() || (d.Name() != "config.yml" && d.Name() != "profile") {
				return nil
			}
			rootPfx, ok := embeddedPackageRootPrefixForPath(path)
			if !ok {
				return nil
			}
			rel := strings.TrimPrefix(path, rootPfx)
			rel = strings.TrimPrefix(rel, "/")
			parent := pathpkg.Dir(rel)
			if parent == "." || parent == "" {
				return nil
			}
			name := pathpkg.Base(parent)
			if _, ok := seen[parent]; ok {
				return nil
			}
			seen[parent] = struct{}{}
			embeddedWorkflowRoots = append(embeddedWorkflowRoots, embedWorkflowRoot{prefix: parent, name: name})
			return nil
		})
		sort.Slice(embeddedWorkflowRoots, func(i, j int) bool {
			return len(embeddedWorkflowRoots[i].prefix) > len(embeddedWorkflowRoots[j].prefix)
		})
	})
}

func embeddedPathHasPackageRootPrefix(path string) bool {
	_, ok := embeddedPackageRootPrefixForPath(path)
	return ok
}

func embeddedPackageRootPrefixForPath(path string) (string, bool) {
	for _, pfx := range embeddedPackageRootsPrefixes {
		if path == pfx || strings.HasPrefix(path, pfx+"/") {
			return pfx, true
		}
	}
	return "", false
}

// mapEmbeddedWorkflowRel maps paths under embeddedPackageRootsPrefixes to bundle/workflows/<workflow>/… using
// discovered config.yml / profile roots (namespace nesting of any depth).
func mapEmbeddedWorkflowRel(rel string) (string, bool) {
	for _, pfx := range embeddedPackageRootsPrefixes {
		if rel != pfx && !strings.HasPrefix(rel, pfx+"/") {
			continue
		}
		normalized := strings.TrimPrefix(rel, pfx)
		normalized = strings.TrimPrefix(normalized, "/")
		if normalized == "" {
			return filepath.Join(BundledLayoutDir, "workflows"), true
		}
		initEmbeddedWorkflowRoots()
		for _, r := range embeddedWorkflowRoots {
			if normalized == r.prefix || strings.HasPrefix(normalized, r.prefix+"/") {
				suffix := strings.TrimPrefix(normalized, r.prefix)
				suffix = strings.TrimPrefix(suffix, "/")
				if suffix == "" {
					return filepath.Join(BundledLayoutDir, "workflows", r.name), true
				}
				return filepath.Join(BundledLayoutDir, "workflows", r.name, filepath.FromSlash(suffix)), true
			}
		}
		rest := normalized
		if rest == "" {
			return filepath.Join(BundledLayoutDir, "workflows"), true
		}
		return filepath.Join(BundledLayoutDir, "workflows", filepath.FromSlash(rest)), true
	}
	return "", false
}

// mapEmbeddedToMaterializedPath maps embed paths (src/core/..., lib/..., VERSION) to the on-disk
// materialized layout: <BundledLayoutDir>/core/..., workflows/..., lib/, version.
func mapEmbeddedToMaterializedPath(rel string) string {
	wfUnderCore := EmbeddedTemplatesPrefix + "/workflows"
	switch {
	case rel == "VERSION":
		return "version"
	case rel == wfUnderCore || strings.HasPrefix(rel, wfUnderCore+"/"):
		rest := strings.TrimPrefix(rel, wfUnderCore)
		rest = strings.TrimPrefix(rest, "/")
		if rest == "" {
			return filepath.Join(BundledLayoutDir, "workflows")
		}
		return filepath.Join(BundledLayoutDir, "workflows", filepath.FromSlash(rest))
	case rel == EmbeddedTemplatesPrefix || strings.HasPrefix(rel, EmbeddedTemplatesPrefix+"/"):
		suffix := strings.TrimPrefix(rel, EmbeddedTemplatesPrefix)
		suffix = strings.TrimPrefix(suffix, "/")
		if suffix == "" {
			return filepath.Join(BundledLayoutDir, "core")
		}
		return filepath.Join(BundledLayoutDir, "core", filepath.FromSlash(suffix))
	case rel == "workflows" || strings.HasPrefix(rel, "workflows/"):
		rest := strings.TrimPrefix(rel, "workflows")
		rest = strings.TrimPrefix(rest, "/")
		if rest == "" {
			return filepath.Join(BundledLayoutDir, "workflows")
		}
		return filepath.Join(BundledLayoutDir, "workflows", rest)
	default:
		for _, pfx := range embeddedPackageRootsPrefixes {
			if rel == pfx || strings.HasPrefix(rel, pfx+"/") {
				if out, ok := mapEmbeddedWorkflowRel(rel); ok {
					return out
				}
				rest := strings.TrimPrefix(rel, pfx)
				rest = strings.TrimPrefix(rest, "/")
				if rest == "" {
					return filepath.Join(BundledLayoutDir, "workflows")
				}
				return filepath.Join(BundledLayoutDir, "workflows", filepath.FromSlash(rest))
			}
		}
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
