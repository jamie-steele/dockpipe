package packagebuild

import (
	"archive/tar"
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"
)

func TestExtractTarGzToDir_MarksExecutablePayloads(t *testing.T) {
	tgz := filepath.Join(t.TempDir(), "payload.tar.gz")
	writeTarGzForTest(t, tgz, map[string]struct {
		body string
		mode int64
	}{
		"workflows/demo/assets/scripts/run.sh":         {body: "#!/usr/bin/env bash\n", mode: 0o644},
		"workflows/demo/assets/tooling/bin/linux/tool": {body: "ELF", mode: 0o644},
		"workflows/demo/assets/docs/readme.txt":        {body: "doc", mode: 0o644},
	})

	dest := filepath.Join(t.TempDir(), "out")
	var marked []string
	old := markExecutableOnDisk
	markExecutableOnDisk = func(p string) error {
		marked = append(marked, filepath.ToSlash(p))
		return nil
	}
	defer func() { markExecutableOnDisk = old }()

	if err := ExtractTarGzToDir(tgz, dest); err != nil {
		t.Fatal(err)
	}

	want := map[string]bool{
		filepath.ToSlash(filepath.Join(dest, "workflows", "demo", "assets", "scripts", "run.sh")):               true,
		filepath.ToSlash(filepath.Join(dest, "workflows", "demo", "assets", "tooling", "bin", "linux", "tool")): true,
	}
	if len(marked) != len(want) {
		t.Fatalf("marked %v, want %d executable payloads", marked, len(want))
	}
	for _, p := range marked {
		if !want[p] {
			t.Fatalf("unexpected executable mark for %s", p)
		}
	}
}

func writeTarGzForTest(t *testing.T, out string, files map[string]struct {
	body string
	mode int64
}) {
	t.Helper()
	f, err := os.Create(out)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	gz := gzip.NewWriter(f)
	defer gz.Close()
	tw := tar.NewWriter(gz)
	defer tw.Close()

	for name, file := range files {
		hdr := &tar.Header{
			Name: name,
			Mode: file.mode,
			Size: int64(len(file.body)),
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(file.body)); err != nil {
			t.Fatal(err)
		}
	}
}
