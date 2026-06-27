package infrastructure

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInvalidateTarballExtractCacheForPackageRemovesMarkerAndLegacyEntries(t *testing.T) {
	dir := t.TempDir()
	cacheRoot := TarballExtractCacheRoot(dir)

	markerCache := filepath.Join(cacheRoot, "marker")
	if err := os.MkdirAll(markerCache, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(
		filepath.Join(markerCache, ".dockpipe-extracted-package.json"),
		[]byte(`{"tarball":"dockpipe-workflow-mywf-0.6.0.tar.gz"}`),
		0o644,
	); err != nil {
		t.Fatal(err)
	}

	legacyCache := filepath.Join(cacheRoot, "legacy", "workflows", "mywf")
	if err := os.MkdirAll(legacyCache, 0o755); err != nil {
		t.Fatal(err)
	}

	otherCache := filepath.Join(cacheRoot, "other", "workflows", "other")
	if err := os.MkdirAll(otherCache, 0o755); err != nil {
		t.Fatal(err)
	}

	n, err := InvalidateTarballExtractCacheForPackage(dir, "workflow", "mywf")
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("removed %d cache entries, want 2", n)
	}
	if _, err := os.Stat(markerCache); !os.IsNotExist(err) {
		t.Fatalf("marker cache still exists or stat failed unexpectedly: %v", err)
	}
	if _, err := os.Stat(filepath.Join(cacheRoot, "legacy")); !os.IsNotExist(err) {
		t.Fatalf("legacy cache still exists or stat failed unexpectedly: %v", err)
	}
	if _, err := os.Stat(otherCache); err != nil {
		t.Fatalf("unrelated cache removed: %v", err)
	}
}
