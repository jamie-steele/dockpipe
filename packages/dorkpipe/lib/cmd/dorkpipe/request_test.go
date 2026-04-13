package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInferWorkspaceChatTargets_FindsMentionedBasename(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	target := filepath.Join(root, "src", "request.go")
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(target, []byte("package main\n"), 0o644); err != nil {
		t.Fatalf("write target: %v", err)
	}

	got := inferWorkspaceChatTargets(root, routeRequest{
		Message: "Explain how request.go currently works.",
	})
	if len(got) == 0 || got[0] != "src/request.go" {
		t.Fatalf("inferWorkspaceChatTargets() = %#v", got)
	}
}

func TestInferWorkspaceChatTargets_UsesGenericWorkspaceSearch(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	files := map[string]string{
		"docs/notes.md":                        "General notes.\n",
		"src/router/chat_flow.go":             "package flow\n\nfunc chooseRoute() {}\n// ask mode chat route context gathering\n",
		"src/web/extension_bridge.ts":         "export function collectWorkspaceSignals() {}\n",
		"src/other/unrelated.go":              "package other\n",
		"src/router/secondary_handler.go":     "package flow\n// mode route\n",
		"src/infra/generated.json":            "{\"route\":\"chat\"}\n",
	}
	for rel, body := range files {
		path := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", rel, err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}

	got := inferWorkspaceChatTargets(root, routeRequest{
		Message: "Explain the current chat route and context gathering flow.",
	})
	if len(got) == 0 {
		t.Fatalf("expected inferred targets, got none")
	}
	if got[0] != "src/router/chat_flow.go" {
		t.Fatalf("expected generic search to prioritize chat_flow.go, got %#v", got)
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
