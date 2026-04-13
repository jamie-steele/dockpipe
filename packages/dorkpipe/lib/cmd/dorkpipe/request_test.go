package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLooksLikeInternalArchitectureQuestion(t *testing.T) {
	t.Parallel()
	if !looksLikeInternalArchitectureQuestion("Explain how Ask mode works internally in the extension flow.") {
		t.Fatalf("expected architecture question to be detected")
	}
	if looksLikeInternalArchitectureQuestion("rename this function to better match the UI") {
		t.Fatalf("did not expect ordinary edit request to be detected as architecture question")
	}
}

func TestInferWorkspaceChatTargets_IncludesArchitectureFiles(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	files := []string{
		"packages/dorkpipe/lib/cmd/dorkpipe/request.go",
		"packages/pipeon/resolvers/pipeon/vscode-extension/src/extension.ts",
		"packages/pipeon/resolvers/pipeon/assets/scripts/prompts/system.md",
		"packages/pipeon/resolvers/pipeon/vscode-extension/src/webview/chat.ts",
	}
	for _, rel := range files {
		path := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", rel, err)
		}
		if err := os.WriteFile(path, []byte("content\n"), 0o644); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}

	got := inferWorkspaceChatTargets(root, routeRequest{
		Message: "Explain the current internal flow for ask mode and extension.ts after the refactor.",
	})
	if len(got) == 0 {
		t.Fatalf("expected inferred targets, got none")
	}
	if got[0] != "packages/dorkpipe/lib/cmd/dorkpipe/request.go" {
		t.Fatalf("expected request.go first, got %#v", got)
	}
}

func TestChooseRoute_SlashInspectPrimitives(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		message string
		action  string
	}{
		{message: "/symbol renderMessages", action: "symbol"},
		{message: "/references renderMessages", action: "references"},
		{message: "/callers renderMessages", action: "callers"},
	} {
		got := chooseRoute(routeRequest{Message: tc.message, Mode: "ask"})
		if got.Route != "inspect" || got.Action != tc.action {
			t.Fatalf("chooseRoute(%q) = %#v", tc.message, got)
		}
	}
}

func TestChooseRoute_DoesNotInferInspectFromPlainText(t *testing.T) {
	t.Parallel()
	for _, message := range []string{
		"show definition for renderMessages",
		"where is renderMessages used",
		"who calls renderMessages",
		"show context",
		"status",
		"explain /callers renderMessages and how it works",
	} {
		got := chooseRoute(routeRequest{Message: message, Mode: "ask"})
		if got.Route != "chat" {
			t.Fatalf("chooseRoute(%q) = %#v, want chat route", message, got)
		}
	}
}
