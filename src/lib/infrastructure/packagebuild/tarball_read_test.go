package packagebuild

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadFileFromTarGz(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "wf")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "config.yml"), []byte("name: x\nrun: y\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	tgz := filepath.Join(dir, "out.tar.gz")
	if _, err := WriteDirTarGzWithPrefix(src, tgz, "workflows/demo"); err != nil {
		t.Fatal(err)
	}
	b, err := ReadFileFromTarGz(tgz, "workflows/demo/config.yml")
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "name: x\nrun: y\n" {
		t.Fatalf("got %q", b)
	}
}
