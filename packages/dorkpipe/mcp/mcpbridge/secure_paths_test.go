package mcpbridge

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolvePathUnderRepoRoot(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("DOCKPIPE_REPO_ROOT", tmp)
	sub := filepath.Join(tmp, "w", "f.yml")
	if err := os.MkdirAll(filepath.Dir(sub), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(sub, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	rel := "w/f.yml"
	got, err := ResolvePathUnderRepoRoot(rel)
	if err != nil {
		t.Fatal(err)
	}
	if got != sub {
		t.Fatalf("got %q want %q", got, sub)
	}
	_, err = ResolvePathUnderRepoRoot("../outside")
	if err == nil {
		t.Fatal("expected escape error")
	}

	subAbs := filepath.Join(tmp, "w", "f.yml")
	if err := CheckAbsolutePathUnderRepoRoot(subAbs); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(tmp, "..", "outside")
	if err := CheckAbsolutePathUnderRepoRoot(outside); err == nil {
		t.Fatal("expected outside repo error")
	}
}
