package infrastructure

import (
	"os"
	"path/filepath"
	"testing"
)

// TestRepoRootMaterializesBundledTemplates copies embedded src/core into the bundle cache and
// materializes a workflow config (under ShipyardDir/workflows/ — cache layout, not a dependency on
// git-tracked workflows/ content).
func TestRepoRootMaterializesBundledTemplates(t *testing.T) {
	t.Setenv("DOCKPIPE_REPO_ROOT", "")
	t.Setenv("DOCKPIPE_BUNDLED_CACHE", t.TempDir())
	got, err := RepoRoot()
	if err != nil {
		t.Fatalf("RepoRoot: %v", err)
	}
	cfg := filepath.Join(got, ShipyardDir, "workflows", "run", "config.yml")
	st, err := os.Stat(cfg)
	if err != nil || st.IsDir() {
		t.Fatalf("expected file %s: err=%v isDir=%v", cfg, err, st != nil && st.IsDir())
	}
}
