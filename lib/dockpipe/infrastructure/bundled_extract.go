package infrastructure

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"dockpipe"
)

// bundledFormatVersion bumps when extraction rules change (forces re-unpack; see .bundled-format).
const bundledFormatVersion = "6"

var bundledMu sync.Mutex

// MaterializedBundledRoot returns a directory containing templates/, scripts/, images/, lib/, and version.
// It unpacks dockpipe.BundledFS into the user cache (see also DOCKPIPE_REPO_ROOT override in RepoRoot).
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
	dest := filepath.Join(cacheBase, "dockpipe", "bundled-"+ver)
	cfgPath := filepath.Join(dest, "templates", "llm-worktree", "config.yml")
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

// bundledCacheBase is the parent directory for dockpipe/bundled-<version> (default: user cache dir).
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

func copyEmbeddedFS(dstRoot string, versionBytes []byte) error {
	return fs.WalkDir(dockpipe.BundledFS, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == "." {
			return nil
		}
		outRel := path
		if path == "VERSION" {
			outRel = "version"
		}
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
