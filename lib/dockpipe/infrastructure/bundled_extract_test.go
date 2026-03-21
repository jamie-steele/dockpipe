package infrastructure

import (
	"testing"
)

// TestEmbeddedWorkflowConfigExists matches known bundled template names.
func TestEmbeddedWorkflowConfigExists(t *testing.T) {
	if !EmbeddedWorkflowConfigExists("test") {
		t.Fatal("expected test")
	}
	if !EmbeddedWorkflowConfigExists("run") {
		t.Fatal("expected run")
	}
	if !EmbeddedWorkflowConfigExists("run-apply-validate") {
		t.Fatal("expected run-apply-validate")
	}
	if !EmbeddedWorkflowConfigExists("init") {
		t.Fatal("expected init")
	}
	for _, name := range []string{"vscode", "cursor-dev", "claude", "codex", "code-server"} {
		if !EmbeddedWorkflowConfigExists(name) {
			t.Fatalf("expected resolver delegate %s", name)
		}
	}
	if EmbeddedWorkflowConfigExists("") {
		t.Fatal("empty name should be false")
	}
	if EmbeddedWorkflowConfigExists("../x") {
		t.Fatal("path traversal should be false")
	}
	if EmbeddedWorkflowConfigExists("nope-not-a-real-template-xyz") {
		t.Fatal("unknown template should be false")
	}
}
