package mcpbridge

import (
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
	got, err := resolveExecWorkdir("/usr")
	if err != nil {
		t.Fatal(err)
	}
	if got != "/usr" {
		t.Fatalf("got %q", got)
	}
}
