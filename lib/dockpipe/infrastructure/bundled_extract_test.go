package infrastructure

import (
	"testing"
)

// TestEmbeddedWorkflowConfigExists matches known bundled template names.
func TestEmbeddedWorkflowConfigExists(t *testing.T) {
	if !EmbeddedWorkflowConfigExists("llm-worktree") {
		t.Fatal("expected llm-worktree")
	}
	if !EmbeddedWorkflowConfigExists("vscode") {
		t.Fatal("expected vscode")
	}
	if !EmbeddedWorkflowConfigExists("cursor-dev") {
		t.Fatal("expected cursor-dev")
	}
	if !EmbeddedWorkflowConfigExists("chain-test") {
		t.Fatal("expected chain-test")
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
