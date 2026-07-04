package infrastructure

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBundleCompileRootsCachedNoImplicitDefault(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	// No dockpipe.config.json — bundle roots must stay empty (no hardcoded maintainer paths).
	if err := os.WriteFile(filepath.Join(repo, "README.md"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := BundleCompileRootsCached(repo)
	if len(got) != 0 {
		t.Fatalf("expected no bundle roots without compile.bundles in config, got %v", got)
	}
}
