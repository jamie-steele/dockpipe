package application

import (
	"os"
	"path/filepath"
	"testing"
)

// testRepoRoot returns the module root (directory containing go.mod) by walking up from cwd.
// `go test` uses the package directory as cwd, so we walk to the repo root.
func testRepoRoot(t *testing.T) string {
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
