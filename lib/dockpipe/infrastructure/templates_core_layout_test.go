package infrastructure

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// localModuleRoot walks up from the test cwd to the repo root (directory containing go.mod).
func localModuleRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found from test working directory")
		}
		dir = parent
	}
}

// TestBundledTemplatesCoreLayoutEnforcesCategoryDirs fails if templates/core top-level dirs drift
// (assets, bundles, resolvers, runtimes, strategies).
func TestBundledTemplatesCoreLayoutEnforcesCategoryDirs(t *testing.T) {
	root := localModuleRoot(t)
	core := filepath.Join(root, "templates", "core")
	ents, err := os.ReadDir(core)
	if err != nil {
		t.Fatal(err)
	}
	allowed := []string{"assets", "bundles", "resolvers", "runtimes", "strategies"}
	var names []string
	for _, e := range ents {
		if e.IsDir() && e.Name()[0] != '.' {
			names = append(names, e.Name())
		}
	}
	slices.Sort(names)
	if !slices.Equal(names, allowed) {
		t.Fatalf("templates/core must contain exactly %v (got %v)", allowed, names)
	}
}

// TestBundledTemplatesCoreAssetsSubdirsEnforcesScriptsImagesCompose fails if assets/ gains an
// unexpected top-level sibling (e.g. treating a new primitive as an asset category incorrectly).
// Domain docs live only under bundles/.../assets/docs/ — not under core assets/.
func TestBundledTemplatesCoreAssetsSubdirsEnforcesScriptsImagesCompose(t *testing.T) {
	root := localModuleRoot(t)
	assets := filepath.Join(root, "templates", "core", "assets")
	ents, err := os.ReadDir(assets)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"compose", "images", "scripts"}
	var names []string
	for _, e := range ents {
		if e.IsDir() && e.Name()[0] != '.' {
			names = append(names, e.Name())
		}
	}
	slices.Sort(names)
	if !slices.Equal(names, want) {
		t.Fatalf("templates/core/assets must contain exactly %v (got %v)", want, names)
	}
}

// TestBundledAssetsIncludePowerShellExample ensures the reusable script asset remains present.
func TestBundledAssetsIncludePowerShellExample(t *testing.T) {
	root := localModuleRoot(t)
	ps := filepath.Join(root, "templates", "core", "assets", "scripts", "helloworld.ps1")
	st, err := os.Stat(ps)
	if err != nil || st.IsDir() {
		t.Fatalf("expected templates/core/assets/scripts/helloworld.ps1: %v", err)
	}
}
