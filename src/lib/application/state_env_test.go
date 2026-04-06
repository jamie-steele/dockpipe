package application

import (
	"path/filepath"
	"testing"

	"dockpipe/src/lib/infrastructure"
)

func TestApplyDockpipeStateEnv(t *testing.T) {
	wd := t.TempDir()
	envMap := map[string]string{}
	if err := applyDockpipeStateEnv(envMap, wd, "Pipeon Dev/Stack"); err != nil {
		t.Fatal(err)
	}
	if got, want := envMap[infrastructure.EnvStateDir], filepath.Join(wd, "bin", ".dockpipe"); got != want {
		t.Fatalf("state dir = %q want %q", got, want)
	}
	if got, want := envMap[infrastructure.EnvPackageID], "pipeon-dev-stack"; got != want {
		t.Fatalf("package id = %q want %q", got, want)
	}
	if got, want := envMap[infrastructure.EnvPackageStateDir], filepath.Join(wd, "bin", ".dockpipe", "packages", "pipeon-dev-stack"); got != want {
		t.Fatalf("package state dir = %q want %q", got, want)
	}
}
