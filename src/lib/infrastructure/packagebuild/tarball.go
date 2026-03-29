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

// WriteDirTarGzWithPrefix packs absSrcDir into a gzip tar at outPath. Archive paths are
// prefixInArchive/rel (forward slashes). prefixInArchive must be non-empty and must not contain "..".
func WriteDirTarGzWithPrefix(absSrcDir, outPath, prefixInArchive string) (sha256Hex string, err error) {
	absSrcDir = filepath.Clean(absSrcDir)
	prefixInArchive = strings.Trim(prefixInArchive, "/\\")
	prefixInArchive = filepath.ToSlash(filepath.Clean(prefixInArchive))
	if prefixInArchive == "" || prefixInArchive == "." || strings.Contains(prefixInArchive, "..") {
		return "", fmt.Errorf("invalid archive prefix %q", prefixInArchive)
	}
	st, err := os.Stat(absSrcDir)
	if err != nil {
		return "", err
	}
	if !st.IsDir() {
		return "", fmt.Errorf("not a directory: %s", absSrcDir)
	}
	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		return "", err
	}
	w, err := os.Create(outPath)
	if err != nil {
		return "", err
	}
	gz := gzip.NewWriter(w)
	tw := tar.NewWriter(gz)
	walkErr := filepath.WalkDir(absSrcDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(absSrcDir, path)
		if err != nil {
			return err
		}
		if rel == "." {
			rel = ""
		}
		if strings.Contains(rel, "..") {
			return fmt.Errorf("unsafe path %q", rel)
		}
		nameInTar := prefixInArchive + "/"
		if rel != "" {
			nameInTar = prefixInArchive + "/" + filepath.ToSlash(rel)
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
	if walkErr != nil {
		_ = tw.Close()
		_ = gz.Close()
		_ = w.Close()
		_ = os.Remove(outPath)
		return "", walkErr
	}
	if err := tw.Close(); err != nil {
		_ = gz.Close()
		_ = w.Close()
		_ = os.Remove(outPath)
		return "", err
	}
	if err := gz.Close(); err != nil {
		_ = w.Close()
		_ = os.Remove(outPath)
		return "", err
	}
	if err := w.Close(); err != nil {
		_ = os.Remove(outPath)
		return "", err
	}
	sum, err := sha256File(outPath)
	if err != nil {
		return "", err
	}
	shaPath := outPath + ".sha256"
	if err := os.WriteFile(shaPath, []byte(sum+"\n"), 0o644); err != nil {
		return "", err
	}
	return sum, nil
}
