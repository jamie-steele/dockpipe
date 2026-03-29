package infrastructure

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSourceDirNewerThanPath(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := filepath.Join(src, "config.yml")
	if err := os.WriteFile(cfg, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	tarPath := filepath.Join(dir, "out.tar.gz")
	if err := os.WriteFile(tarPath, []byte("gz"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Touch tarball to be newer than source
	old := time.Now().Add(-2 * time.Hour)
	if err := os.Chtimes(cfg, old, old); err != nil {
		t.Fatal(err)
	}
	newer := time.Now()
	if err := os.Chtimes(tarPath, newer, newer); err != nil {
		t.Fatal(err)
	}
	stale, err := SourceDirNewerThanPath(src, tarPath)
	if err != nil {
		t.Fatal(err)
	}
	if stale {
		t.Fatal("expected not stale when tarball is newer")
	}
	// Make source newer
	if err := os.Chtimes(cfg, time.Now(), time.Now()); err != nil {
		t.Fatal(err)
	}
	stale, err = SourceDirNewerThanPath(src, tarPath)
	if err != nil {
		t.Fatal(err)
	}
	if !stale {
		t.Fatal("expected stale when source is newer")
	}
}

func TestPickLatestModTimePath(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a")
	b := filepath.Join(dir, "b")
	_ = os.WriteFile(a, []byte("1"), 0o644)
	_ = os.WriteFile(b, []byte("2"), 0o644)
	old := time.Now().Add(-time.Hour)
	_ = os.Chtimes(a, old, old)
	if got := PickLatestModTimePath([]string{a, b}); got != b {
		t.Fatalf("want newer file b, got %q", got)
	}
}
