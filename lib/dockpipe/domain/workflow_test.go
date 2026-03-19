package domain

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseWorkflowYAMLSteps(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "config.yml")
	content := `
name: t
isolate: alpine
steps:
  - isolate: alpine
    cmd: echo hi
  - cmd: echo bye
`
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	w, err := ParseWorkflowYAML(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(w.Steps) != 2 {
		t.Fatalf("steps: got %d", len(w.Steps))
	}
	if w.Steps[0].CmdLine() != "echo hi" {
		t.Fatalf("cmd0: %q", w.Steps[0].CmdLine())
	}
}
