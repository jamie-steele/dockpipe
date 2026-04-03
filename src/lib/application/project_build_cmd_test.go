package application

import (
	"os"
	"path/filepath"
	"testing"

	"dockpipe/src/lib/infrastructure"
)

func TestCmdCleanRemovesPackagesRoot(t *testing.T) {
	dir := t.TempDir()
	pkg := filepath.Join(dir, infrastructure.DockpipeDirRel, "internal", "packages", "core")
	if err := os.MkdirAll(pkg, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkg, "marker"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := cmdClean([]string{"--workdir", dir}); err != nil {
		t.Fatalf("cmdClean: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, infrastructure.DockpipeDirRel, "internal", "packages")); !os.IsNotExist(err) {
		t.Fatalf("expected packages dir removed, stat err=%v", err)
	}
}

func TestCmdBuildDelegatesToCompileAll(t *testing.T) {
	if err := cmdBuild([]string{"--help"}); err != nil {
		t.Fatalf("cmdBuild --help: %v", err)
	}
}

func TestCmdRebuildHelp(t *testing.T) {
	if err := cmdRebuild([]string{"--help"}); err != nil {
		t.Fatalf("cmdRebuild --help: %v", err)
	}
}
