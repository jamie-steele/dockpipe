// Package packagebuild authors gzip tarballs for dockpipe install and self-hosted registries.
package packagebuild

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// CoreReleaseManifest is a minimal install-manifest.json fragment for packages.core.
type CoreReleaseManifest struct {
	Schema   int `json:"schema"`
	Packages struct {
		Core struct {
			Version string `json:"version"`
			Tarball string `json:"tarball"`
			SHA256  string `json:"sha256"`
		} `json:"core"`
	} `json:"packages"`
}

// WriteCoreRelease writes templates-core-<ver>.tar.gz (+ .sha256 + install-manifest.json) for dockpipe install core.
// coreParent must be the directory containing a child "core" (e.g. .../src or .../templates).
func WriteCoreRelease(coreParent, outDir, version string) (tarGzPath string, err error) {
	coreParent = filepath.Clean(coreParent)
	coreRoot := filepath.Join(coreParent, "core")
	if st, err := os.Stat(coreRoot); err != nil || !st.IsDir() {
		return "", fmt.Errorf("expected directory %q (child of %q)", coreRoot, coreParent)
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return "", err
	}
	if strings.TrimSpace(version) == "" {
		return "", fmt.Errorf("version is empty")
	}
	baseName := fmt.Sprintf("templates-core-%s.tar.gz", version)
	tarGzPath = filepath.Join(outDir, baseName)

	w, err := os.Create(tarGzPath)
	if err != nil {
		return "", err
	}
	gz := gzip.NewWriter(w)
	tw := tar.NewWriter(gz)
	if err := tarTreeCore(coreRoot, tw); err != nil {
		_ = tw.Close()
		_ = gz.Close()
		_ = w.Close()
		_ = os.Remove(tarGzPath)
		return "", err
	}
	if err := tw.Close(); err != nil {
		_ = gz.Close()
		_ = w.Close()
		_ = os.Remove(tarGzPath)
		return "", err
	}
	if err := gz.Close(); err != nil {
		_ = w.Close()
		_ = os.Remove(tarGzPath)
		return "", err
	}
	if err := w.Close(); err != nil {
		_ = os.Remove(tarGzPath)
		return "", err
	}

	sum, err := sha256File(tarGzPath)
	if err != nil {
		return "", err
	}
	shaPath := tarGzPath + ".sha256"
	if err := os.WriteFile(shaPath, []byte(sum+"\n"), 0o644); err != nil {
		return "", err
	}

	m := CoreReleaseManifest{Schema: 1}
	m.Packages.Core.Version = version
	m.Packages.Core.Tarball = baseName
	m.Packages.Core.SHA256 = sum
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return "", err
	}
	b = append(b, '\n')
	manifestPath := filepath.Join(outDir, "install-manifest.json")
	if err := os.WriteFile(manifestPath, b, 0o644); err != nil {
		return "", err
	}
	return tarGzPath, nil
}

func tarTreeCore(coreRoot string, tw *tar.Writer) error {
	return filepath.WalkDir(coreRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(coreRoot, path)
		if err != nil {
			return err
		}
		// Dockpipe source uses src/core/workflows for bundled examples; omit from templates/core install tarball.
		if rel == "workflows" || strings.HasPrefix(rel, "workflows"+string(filepath.Separator)) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		nameInTar := "core"
		if rel != "." {
			nameInTar = filepath.ToSlash(filepath.Join("core", rel))
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		hdr, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		hdr.Name = nameInTar
		if hdr.Typeflag == tar.TypeDir && !strings.HasSuffix(hdr.Name, "/") {
			hdr.Name += "/"
		}
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if !d.Type().IsRegular() {
			return fmt.Errorf("not a regular file: %s", path)
		}
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		_, err = io.Copy(tw, f)
		_ = f.Close()
		return err
	})
}

func sha256File(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}
