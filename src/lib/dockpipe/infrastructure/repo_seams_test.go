package infrastructure

import (
	"os"
	"path/filepath"
	"testing"
)

// TestRepoRootMaterializesBundledTemplates copies bundled templates into DOCKPIPE_BUNDLED_CACHE and finds config.yml.
func TestRepoRootMaterializesBundledTemplates(t *testing.T) {
	t.Setenv("DOCKPIPE_REPO_ROOT", "")
	t.Setenv("DOCKPIPE_BUNDLED_CACHE", t.TempDir())
	got, err := RepoRoot()
	if err != nil {
		t.Fatalf("RepoRoot: %v", err)
	}
	cfg := filepath.Join(got, "shipyard", "workflows", "test", "config.yml")
	st, err := os.Stat(cfg)
	if err != nil || st.IsDir() {
		t.Fatalf("expected file %s: err=%v isDir=%v", cfg, err, st != nil && st.IsDir())
	}
}
