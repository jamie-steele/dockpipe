package infrastructure

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveWorkflowYAMLPath_relativeFromSubdir(t *testing.T) {
	root := t.TempDir()
	wfDir := filepath.Join(root, "workflows", "demo")
	if err := os.MkdirAll(wfDir, 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := filepath.Join(wfDir, "config.yml")
	if err := os.WriteFile(cfg, []byte("name: demo\nsteps: []\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	sub := filepath.Join(root, "nested", "deep")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DOCKPIPE_REPO_ROOT", root)
	t.Chdir(sub)

	got, err := ResolveWorkflowYAMLPath("workflows/demo/config.yml")
	if err != nil {
		t.Fatalf("ResolveWorkflowYAMLPath: %v", err)
	}
	want, err := filepath.Abs(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestResolveWorkflowYAMLPath_defaultSingleWorkflow(t *testing.T) {
	root := t.TempDir()
	wfDir := filepath.Join(root, "workflows", "solo")
	if err := os.MkdirAll(wfDir, 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := filepath.Join(wfDir, "config.yml")
	if err := os.WriteFile(cfg, []byte("name: solo\nsteps: []\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DOCKPIPE_REPO_ROOT", root)
	t.Chdir(root)

	got, err := ResolveWorkflowYAMLPath("")
	if err != nil {
		t.Fatalf("ResolveWorkflowYAMLPath: %v", err)
	}
	want, err := filepath.Abs(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}
