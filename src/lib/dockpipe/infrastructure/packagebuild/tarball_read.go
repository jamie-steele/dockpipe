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
// for workflow resolution (dockpipe run) without extracting the archive.
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

// ListTarGzMemberPaths returns regular-file member paths (forward slashes) in a gzip tar.
func ListTarGzMemberPaths(tarGzPath string) ([]string, error) {
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
	var out []string
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if hdr.Typeflag != tar.TypeReg && hdr.Typeflag != tar.TypeRegA {
			continue
		}
		name := strings.TrimSuffix(strings.TrimSpace(hdr.Name), "/")
		name = strings.TrimPrefix(name, "./")
		name = filepath.ToSlash(name)
		if name == "" || strings.Contains(name, "..") {
			if _, err := io.Copy(io.Discard, tr); err != nil {
				return nil, err
			}
			continue
		}
		out = append(out, name)
		if _, err := io.Copy(io.Discard, tr); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// WorkflowNameFromTarballMembers finds workflows/<name>/config.yml and returns name.
func WorkflowNameFromTarballMembers(members []string) (string, error) {
	for _, m := range members {
		parts := strings.Split(m, "/")
		if len(parts) >= 3 && parts[0] == "workflows" && parts[2] == "config.yml" {
			return parts[1], nil
		}
	}
	return "", fmt.Errorf("no workflows/<name>/config.yml in archive")
}
