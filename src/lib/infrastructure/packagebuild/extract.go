package packagebuild

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// ExtractTarGzToDir extracts all regular files from a gzip tar into destDir (paths are relative to archive root).
// Rejects member paths containing ".." after cleaning.
func ExtractTarGzToDir(tarGzPath, destDir string) error {
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return err
	}
	f, err := os.Open(tarGzPath)
	if err != nil {
		return err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		name := strings.TrimPrefix(strings.TrimSpace(hdr.Name), "./")
		name = filepath.ToSlash(name)
		if name == "" || strings.Contains(name, "..") {
			continue
		}
		target := filepath.Join(destDir, filepath.FromSlash(name))
		if !strings.HasPrefix(filepath.Clean(target), filepath.Clean(destDir)) {
			return fmt.Errorf("unsafe tar path %q", hdr.Name)
		}
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			out, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, os.FileMode(hdr.Mode&0o777))
			if err != nil {
				return err
			}
			if _, err := io.Copy(out, tr); err != nil {
				_ = out.Close()
				return err
			}
			if err := out.Close(); err != nil {
				return err
			}
		default:
			continue
		}
	}
}

const extractMarkerFile = ".dockpipe-extracted"

// EnsureTarballExtractedCache extracts tarGzPath into cacheRoot/<sha256(tar file)> if not already present.
// The cache directory is keyed by the tarball file digest so content changes invalidate the cache.
func EnsureTarballExtractedCache(tarGzPath, cacheRoot string) (string, error) {
	sum, err := sha256File(tarGzPath)
	if err != nil {
		return "", err
	}
	dest := filepath.Join(cacheRoot, sum)
	marker := filepath.Join(dest, extractMarkerFile)
	if b, err := os.ReadFile(marker); err == nil && strings.TrimSpace(string(b)) == sum {
		if st, err := os.Stat(dest); err == nil && st.IsDir() {
			return dest, nil
		}
	}
	if err := os.RemoveAll(dest); err != nil {
		return "", err
	}
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return "", err
	}
	if err := ExtractTarGzToDir(tarGzPath, dest); err != nil {
		_ = os.RemoveAll(dest)
		return "", err
	}
	if err := os.WriteFile(marker, []byte(sum+"\n"), 0o644); err != nil {
		return "", err
	}
	return dest, nil
}
