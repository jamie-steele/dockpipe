package application

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveStrategyFilePathPrefersPerWorkflowStrategiesDir(t *testing.T) {
	repo := t.TempDir()
	wfRoot := filepath.Join(repo, "templates", "myflow")
	_ = os.MkdirAll(filepath.Join(wfRoot, "strategies"), 0o755)
	local := filepath.Join(wfRoot, "strategies", "mine")
	if err := os.WriteFile(local, []byte("DOCKPIPE_STRATEGY_BEFORE=a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	core := filepath.Join(repo, "templates", "core", "strategies", "mine")
	if err := os.MkdirAll(filepath.Dir(core), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(core, []byte("DOCKPIPE_STRATEGY_BEFORE=core\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := ResolveStrategyFilePath(repo, wfRoot, "mine")
	if err != nil {
		t.Fatal(err)
	}
	if p != local {
		t.Fatalf("want per-workflow strategy %s got %s", local, p)
	}
}

func TestResolveStrategyFilePathUsesTemplatesCoreStrategies(t *testing.T) {
	repo := t.TempDir()
	wfRoot := filepath.Join(repo, "templates", "myflow")
	_ = os.MkdirAll(wfRoot, 0o755)
	core := filepath.Join(repo, "templates", "core", "strategies", "worktree")
	if err := os.MkdirAll(filepath.Dir(core), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(core, []byte("DOCKPIPE_STRATEGY_KIND=git\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := ResolveStrategyFilePath(repo, wfRoot, "worktree")
	if err != nil {
		t.Fatal(err)
	}
	if p != core {
		t.Fatalf("want core strategy %s got %s", core, p)
	}
}

// TestResolveStrategyFilePathDoesNotSearchAssets verifies strategies are not resolved from
// templates/core/assets (support files only — not lifecycle wrappers).
func TestResolveStrategyFilePathDoesNotSearchAssets(t *testing.T) {
	repo := t.TempDir()
	wfRoot := filepath.Join(repo, "templates", "myflow")
	_ = os.MkdirAll(filepath.Join(repo, "templates", "core", "assets", "scripts"), 0o755)
	decoy := filepath.Join(repo, "templates", "core", "assets", "scripts", "worktree")
	if err := os.WriteFile(decoy, []byte("DOCKPIPE_STRATEGY_BEFORE=x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := ResolveStrategyFilePath(repo, wfRoot, "worktree")
	if err == nil {
		t.Fatal("expected error: strategy must not be loaded from assets/")
	}
}
