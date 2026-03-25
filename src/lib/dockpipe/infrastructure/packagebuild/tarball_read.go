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

// ReadFileFromTarGz returns the uncompressed body of the first tar member whose path equals entryName.
// entryName uses forward slashes (e.g. workflows/demo/config.yml). Does not extract to disk — suitable
// for streaming inspection of package tarballs (dockpipe package read).
func ReadFileFromTarGz(tarGzPath, entryName string) ([]byte, error) {
	entryName = filepath.ToSlash(strings.TrimSpace(entryName))
	if entryName == "" || strings.Contains(entryName, "..") {
		return nil, fmt.Errorf("invalid entry path %q", entryName)
	}
	f, err := os.Open(tarGzPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return nil, err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return nil, fmt.Errorf("entry %q not found in %s", entryName, tarGzPath)
		}
		if err != nil {
			return nil, err
		}
		if hdr.Typeflag == tar.TypeDir {
			continue
		}
		name := strings.TrimSuffix(hdr.Name, "/")
		name = strings.TrimPrefix(name, "./")
		name = filepath.ToSlash(name)
		if name == entryName {
			return io.ReadAll(tr)
		}
	}
}
