package infrastructure

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// TestListWorkflowNamesInRepoRoot lists templates/* with config.yml (matches bundled layout).
func TestListWorkflowNamesInRepoRoot(t *testing.T) {
	tmp := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmp, "templates", "a"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "templates", "a", "config.yml"), []byte("name: a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(tmp, "templates", "emptydir"), 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := ListWorkflowNamesInRepoRoot(tmp)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(got, []string{"a"}) {
		t.Fatalf("got %#v", got)
	}
}
