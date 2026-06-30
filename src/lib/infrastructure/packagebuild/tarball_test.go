package packagebuild

import (
	"archive/tar"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteDirTarGzWithPrefix(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "a", "b"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "a", "b", "f.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "p.tar.gz")
	sum, err := WriteDirTarGzWithPrefix(dir, out, "workflows/demo")
	if err != nil {
		t.Fatal(err)
	}
	if len(sum) != 64 {
		t.Fatalf("sha256 len: %s", sum)
	}
	f, err := os.Open(out)
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
		if h.Name == "workflows/demo/a/b/f.txt" {
			b, _ := io.ReadAll(tr)
			if string(b) != "x" {
				t.Fatalf("content %q", b)
			}
		}
	}
	if !strings.Contains(strings.Join(names, ","), "workflows/demo/a/b/f.txt") {
		t.Fatalf("names: %v", names)
	}
}

func TestWriteDirTarGzWithPrefix_ForcesExecutableModesForPackagedPayloads(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "assets", "scripts", "run.sh")
	bin := filepath.Join(dir, "assets", "tooling", "bin", "linux", "dockpipe")
	doc := filepath.Join(dir, "assets", "docs", "readme.txt")
	for _, p := range []string{script, bin, doc} {
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	out := filepath.Join(t.TempDir(), "p.tar.gz")
	if _, err := WriteDirTarGzWithPrefix(dir, out, "workflows/demo"); err != nil {
		t.Fatal(err)
	}

	f, err := os.Open(out)
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
	modes := map[string]int64{}
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		modes[h.Name] = h.Mode
	}

	if got := modes["workflows/demo/assets/scripts/run.sh"]; got != 0o755 {
		t.Fatalf("script mode = %o, want 755", got)
	}
	if got := modes["workflows/demo/assets/tooling/bin/linux/dockpipe"]; got != 0o755 {
		t.Fatalf("binary mode = %o, want 755", got)
	}
	if got := modes["workflows/demo/assets/docs/readme.txt"]; got != 0o644 {
		t.Fatalf("doc mode = %o, want 644", got)
	}
}
