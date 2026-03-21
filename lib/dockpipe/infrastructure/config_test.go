package infrastructure

import (
	"os"
	"path/filepath"
	"testing"
)

// TestLoadWorkflow reads a YAML file from disk and parses steps (no imports in this minimal case).
func TestLoadWorkflow(t *testing.T) {
	tmp := t.TempDir()
	p := filepath.Join(tmp, "config.yml")
	if err := os.WriteFile(p, []byte("name: demo\nsteps:\n  - cmd: echo hi\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	wf, err := LoadWorkflow(p)
	if err != nil {
		t.Fatalf("LoadWorkflow error: %v", err)
	}
	if wf.Name != "demo" || len(wf.Steps) != 1 || wf.Steps[0].CmdLine() != "echo hi" {
		t.Fatalf("unexpected workflow: %#v", wf)
	}
}

// TestLoadWorkflowReadError returns an error when the workflow file is missing.
func TestLoadWorkflowReadError(t *testing.T) {
	if _, err := LoadWorkflow(filepath.Join(t.TempDir(), "missing.yml")); err == nil {
		t.Fatal("expected read error for missing workflow file")
	}
}
