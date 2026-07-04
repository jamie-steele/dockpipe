package mcpbridge

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveExecWorkdirRestrict(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("DOCKPIPE_REPO_ROOT", tmp)
	// Restriction defaults to on; do not opt out.

	got, err := resolveExecWorkdir("")
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Clean(got) != filepath.Clean(tmp) {
		t.Fatalf("empty workdir: got %q want %q", got, tmp)
	}

	_, err = resolveExecWorkdir("/tmp")
	if err == nil {
		t.Fatal("expected error for workdir outside repo")
	}
}

func TestResolveExecWorkdirOptOut(t *testing.T) {
	t.Setenv("DOCKPIPE_MCP_RESTRICT_WORKDIR", "0")
	tmp := t.TempDir()
	t.Setenv("DOCKPIPE_REPO_ROOT", tmp)
	t.Chdir(tmp)
	// /usr is absolute on Unix but not on Windows (filepath.IsAbs); use a path outside the repo on every OS.
	outside, err := os.MkdirTemp(filepath.Dir(tmp), "mcp-workdir-outside-*")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(outside) })
	want, err := filepath.Abs(outside)
	if err != nil {
		t.Fatal(err)
	}
	got, err := resolveExecWorkdir(want)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Clean(got) != filepath.Clean(want) {
		t.Fatalf("got %q want %q", got, want)
	}
}
