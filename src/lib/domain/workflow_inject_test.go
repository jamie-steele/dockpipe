package domain

import (
	"testing"
)

func TestWorkflowInjectListYAML(t *testing.T) {
	y := `
name: t
inject:
  - base-wf
  - workflow: other-wf
  - resolver: codex
  - package: pkg-id
steps: []
`
	w, err := ParseWorkflowYAML([]byte(y))
	if err != nil {
		t.Fatal(err)
	}
	if len(w.Inject) != 4 {
		t.Fatalf("inject len: got %d %+v", len(w.Inject), w.Inject)
	}
	if w.Inject[0].Workflow != "base-wf" || w.Inject[0].WorkflowManifestName() != "base-wf" {
		t.Fatalf("entry0: %+v", w.Inject[0])
	}
	if w.Inject[1].Workflow != "other-wf" {
		t.Fatalf("entry1: %+v", w.Inject[1])
	}
	if w.Inject[2].Resolver != "codex" || w.Inject[2].WorkflowManifestName() != "" {
		t.Fatalf("entry2: %+v", w.Inject[2])
	}
	if w.Inject[3].Package != "pkg-id" || w.Inject[3].WorkflowManifestName() != "pkg-id" {
		t.Fatalf("entry3: %+v", w.Inject[3])
	}
}

func TestWorkflowInjectListYAMLInvalid(t *testing.T) {
	y := `name: t
inject: not-a-seq
steps: []
`
	_, err := ParseWorkflowYAML([]byte(y))
	if err == nil {
		t.Fatal("expected error for non-sequence inject")
	}
}
