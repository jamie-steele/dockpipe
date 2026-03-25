package packagebuild

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteCoreRelease(t *testing.T) {
	dir := t.TempDir()
	coreParent := filepath.Join(dir, "src", "templates")
	if err := os.MkdirAll(filepath.Join(coreParent, "core", "nested"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(coreParent, "core", "nested", "a.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "release", "artifacts")
	path, err := WriteCoreRelease(coreParent, out, "9.9.9-test")
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(path) != "templates-core-9.9.9-test.tar.gz" {
		t.Fatalf("basename: %s", path)
	}
	b, err := os.ReadFile(filepath.Join(out, "install-manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var m CoreReleaseManifest
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	if m.Packages.Core.Version != "9.9.9-test" || m.Packages.Core.Tarball != "templates-core-9.9.9-test.tar.gz" {
		t.Fatalf("manifest: %+v", m.Packages.Core)
	}
	if m.Packages.Core.SHA256 == "" {
		t.Fatal("empty sha in manifest")
	}
	shaFile, err := os.ReadFile(path + ".sha256")
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(shaFile)) != m.Packages.Core.SHA256 {
		t.Fatalf("sha file vs manifest: %q vs %q", strings.TrimSpace(string(shaFile)), m.Packages.Core.SHA256)
	}

	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	var names []string
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		names = append(names, h.Name)
		if h.Name == "core/nested/a.txt" {
			body, err := io.ReadAll(tr)
			if err != nil {
				t.Fatal(err)
			}
			if string(body) != "hello" {
				t.Fatalf("content: %q", body)
			}
		}
	}
	want := []string{"core/", "core/nested/", "core/nested/a.txt"}
	if len(names) != len(want) {
		t.Fatalf("got %v want %v", names, want)
	}
	for i := range want {
		if names[i] != want[i] {
			t.Fatalf("names[%d]: got %q want %q", i, names[i], want[i])
		}
	}
}
