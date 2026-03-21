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

func TestParseWorkflowYAMLAsyncGroupAndID(t *testing.T) {
	y := `
steps:
  - id: a
    cmd: echo a
    is_blocking: false
  - id: b
    cmd: echo b
    is_blocking: true
`
	w, err := ParseWorkflowYAML([]byte(y))
	if err != nil {
		t.Fatal(err)
	}
	if len(w.Steps) != 2 {
		t.Fatalf("steps: %d", len(w.Steps))
	}
	if w.Steps[0].ID != "a" || w.Steps[0].IsBlocking() {
		t.Fatalf("step0: id=%q blocking=%v", w.Steps[0].ID, w.Steps[0].IsBlocking())
	}
	if w.Steps[1].ID != "b" || !w.Steps[1].IsBlocking() {
		t.Fatalf("step1: id=%q blocking=%v", w.Steps[1].ID, w.Steps[1].IsBlocking())
	}
	if w.Steps[0].DisplayName(0) != "a" || w.Steps[1].DisplayName(1) != "b" {
		t.Fatalf("DisplayName: %q %q", w.Steps[0].DisplayName(0), w.Steps[1].DisplayName(1))
	}
}

func TestParseWorkflowYAMLAsyncGroupSugar(t *testing.T) {
	y := `
steps:
  - id: setup
    cmd: echo setup
    is_blocking: true
  - group:
      mode: async
      tasks:
        - id: task_a
          cmd: echo a
        - id: task_b
          cmd: echo b
  - id: aggregate
    cmd: echo agg
    is_blocking: true
`
	w, err := ParseWorkflowYAML([]byte(y))
	if err != nil {
		t.Fatal(err)
	}
	if len(w.Steps) != 4 {
		t.Fatalf("flattened steps: want 4, got %d", len(w.Steps))
	}
	if w.Steps[0].ID != "setup" || !w.Steps[0].IsBlocking() {
		t.Fatalf("step0: %+v", w.Steps[0])
	}
	if w.Steps[1].ID != "task_a" || w.Steps[1].IsBlocking() {
		t.Fatalf("step1 should be non-blocking: %+v", w.Steps[1])
	}
	if w.Steps[2].ID != "task_b" || w.Steps[2].IsBlocking() {
		t.Fatalf("step2 should be non-blocking: %+v", w.Steps[2])
	}
	if w.Steps[3].ID != "aggregate" || !w.Steps[3].IsBlocking() {
		t.Fatalf("step3: %+v", w.Steps[3])
	}
}

func TestParseWorkflowYAMLGroupValidation(t *testing.T) {
	_, err := ParseWorkflowYAML([]byte(`steps:
  - group:
      mode: parallel
      tasks:
        - cmd: x
`))
	if err == nil {
		t.Fatal("expected error for mode != async")
	}
	_, err = ParseWorkflowYAML([]byte(`steps:
  - group:
      mode: async
      tasks: []
`))
	if err == nil {
		t.Fatal("expected error for empty tasks")
	}
	_, err = ParseWorkflowYAML([]byte(`steps:
  - group:
      mode: async
      tasks:
        - cmd: x
          is_blocking: true
`))
	if err == nil {
		t.Fatal("expected error for is_blocking inside group")
	}
	_, err = ParseWorkflowYAML([]byte(`steps:
  - group:
      mode: async
      tasks:
        - cmd: x
    cmd: oops
`))
	if err == nil {
		t.Fatal("expected error for group + extra keys")
	}
}

func TestParseWorkflowYAMLDescription(t *testing.T) {
	y := `name: t
description: Do the task
isolate: alpine
`
	w, err := ParseWorkflowYAML([]byte(y))
	if err != nil {
		t.Fatal(err)
	}
	if w.Description != "Do the task" {
		t.Fatalf("description: %q", w.Description)
	}
}
