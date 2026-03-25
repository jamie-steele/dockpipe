package infrastructure

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPackagesRootDefault(t *testing.T) {
	dir := t.TempDir()
	got, err := PackagesRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(dir, PackagesDirRel)
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestPackagesRootEnvOverride(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "my-packages")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv(envPackagesRoot, sub)
	got, err := PackagesRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got != sub {
		t.Fatalf("got %q want %q", got, sub)
	}
}

func TestPackagesRootEnvRelative(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(envPackagesRoot, "vendor/dockpipe-packages")
	got, err := PackagesRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(got, filepath.Join("vendor", "dockpipe-packages")) {
		t.Fatalf("got %q", got)
	}
}
