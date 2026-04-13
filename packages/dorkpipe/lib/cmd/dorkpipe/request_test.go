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

func TestInferWorkspaceChatTargets_PrefersSourceOverDocsForArchitectureQuery(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	files := map[string]string{
		"docs/pipeon-ide-experience.md": "This document describes the IDE experience and routing at a high level.\n",
		"src/runtime/ask_mode.go":       "package runtime\n\nfunc handleChatRoute() {}\nfunc buildWorkspaceChatContext() {}\n",
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
		Message: "Explain the ask mode routing and context flow.",
	})
	if len(got) == 0 {
		t.Fatalf("expected inferred targets, got none")
	}
	if got[0] != "src/runtime/ask_mode.go" {
		t.Fatalf("expected source file to outrank docs, got %#v", got)
	}
}

func TestInferWorkspaceChatTargets_SkipsCacheArtifacts(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	files := map[string]string{
		"bin/.dockpipe/internal/cache/tarball/abc/resolvers/pipeon/assets/docs/pipeon-ide-experience.md": "cached doc copy\n",
		"src/runtime/ask_mode.go": "package runtime\n\nfunc chooseRoute() {}\nfunc handleChatRoute() {}\n",
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
		Message: "Explain the current ask route.",
	})
	for _, rel := range got {
		if filepath.ToSlash(rel) == "bin/.dockpipe/internal/cache/tarball/abc/resolvers/pipeon/assets/docs/pipeon-ide-experience.md" {
			t.Fatalf("expected cache artifact to be skipped, got %#v", got)
		}
	}
}

func TestInferWorkspaceChatTargets_PrefersImplementationOverTests(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	files := map[string]string{
		"packages/dorkpipe/lib/cmd/dorkpipe/request.go":      "package main\n\nfunc chooseRoute(req routeRequest) routedRequest { return routedRequest{} }\n",
		"packages/dorkpipe/lib/cmd/dorkpipe/request_test.go": "package main\n\nfunc TestChooseRoute(t *testing.T) {}\n",
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
		Message: "Explain how chooseRoute currently works internally.",
	})
	if len(got) == 0 {
		t.Fatalf("expected inferred targets, got none")
	}
	if got[0] != "packages/dorkpipe/lib/cmd/dorkpipe/request.go" {
		t.Fatalf("expected implementation file first, got %#v", got)
	}
	for _, rel := range got {
		if rel == "packages/dorkpipe/lib/cmd/dorkpipe/request_test.go" {
			t.Fatalf("expected test file to be pruned when implementation exists, got %#v", got)
		}
	}
}

func TestInferWorkspaceChatTargets_RequiresDenseArchitectureMatches(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	files := map[string]string{
		"packages/dorkpipe/lib/cmd/dorkpipe/request.go":           "package main\n\nfunc chooseRoute(req routeRequest) routedRequest {}\nfunc handleChatRoute(ctx context.Context) {}\nfunc buildWorkspaceChatContext(root string, req routeRequest) workspaceChatContext {}\n",
		"packages/dorkpipe/lib/cmd/dorkpipe/structured_trace.go":  "package main\n\nfunc writePreparedArtifactBundle() {}\nfunc deriveStructuredEdits() {}\n// validation status for artifacts\n",
		"src/apps/pipeon-launcher/src/BasicModeWidget.cpp":        "namespace pipeon { class BasicModeWidget {}; }\n",
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
		Message: "Explain how Ask mode routes and grounds an architecture question after the latest validation changes.",
	})
	if len(got) == 0 {
		t.Fatalf("expected inferred targets, got none")
	}
	if got[0] != "packages/dorkpipe/lib/cmd/dorkpipe/request.go" {
		t.Fatalf("expected request.go first, got %#v", got)
	}
	for _, rel := range got {
		if rel == "packages/dorkpipe/lib/cmd/dorkpipe/structured_trace.go" || rel == "src/apps/pipeon-launcher/src/BasicModeWidget.cpp" {
			t.Fatalf("expected loose architecture overlaps to be filtered, got %#v", got)
		}
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

func TestValidateChatAnswer_AcceptsEvidenceAnchoredAnswer(t *testing.T) {
	t.Parallel()
	req := routeRequest{
		Message: "Explain how the chat route works internally.",
		Mode:    "ask",
	}
	ctx := workspaceChatContext{
		Text: "Relevant file: src/request.go\n\n```text\nfunc chooseRoute(req routeRequest) routedRequest {\n}\nfunc handleChatRoute(ctx context.Context) {\n}\n```",
		Targets: []string{"src/request.go"},
		Snippets: map[string]string{
			"src/request.go": "func chooseRoute(req routeRequest) routedRequest {}\nfunc handleChatRoute(ctx context.Context) {}\n",
		},
		Evidence: buildChatEvidenceGraph(req, []string{"src/request.go"}, map[string]string{
			"src/request.go": "func chooseRoute(req routeRequest) routedRequest {}\nfunc handleChatRoute(ctx context.Context) {}\n",
		}),
	}
	answer := "## Confirmed\n- Ask mode routes conversational requests through `chooseRoute`. Evidence: `src/request.go` :: `chooseRoute`\n- Chat execution runs through `handleChatRoute`. Evidence: `src/request.go` :: `handleChatRoute`\n\n## Uncertain\n- Anything beyond the retrieved snippet."
	got := validateChatAnswer(answer, req, ctx)
	if !got.Passed {
		t.Fatalf("validateChatAnswer() = %#v", got)
	}
}

func TestValidateChatAnswer_RejectsUnsupportedClaims(t *testing.T) {
	t.Parallel()
	req := routeRequest{
		Message: "Explain how the chat route works internally.",
		Mode:    "ask",
	}
	ctx := workspaceChatContext{
		Text: "Relevant file: src/request.go\n\n```text\nfunc chooseRoute(req routeRequest) routedRequest {\n}\n```",
		Targets: []string{"src/request.go"},
		Snippets: map[string]string{
			"src/request.go": "func chooseRoute(req routeRequest) routedRequest {}\n",
		},
		Evidence: buildChatEvidenceGraph(req, []string{"src/request.go"}, map[string]string{
			"src/request.go": "func chooseRoute(req routeRequest) routedRequest {}\n",
		}),
	}
	answer := "Ask mode falls back through req.Route and then calls handleChatRoute(req)."
	got := validateChatAnswer(answer, req, ctx)
	if got.Passed {
		t.Fatalf("expected validation failure, got %#v", got)
	}
	if len(got.Issues) == 0 {
		t.Fatalf("expected validation issues, got %#v", got)
	}
}

func TestBuildChatEvidenceGraph_IncludesFileAndSymbolNodes(t *testing.T) {
	t.Parallel()
	req := routeRequest{
		Message: "Explain the ask route.",
		Mode:    "ask",
	}
	graph := buildChatEvidenceGraph(req, []string{"src/request.go"}, map[string]string{
		"src/request.go": "func chooseRoute(req routeRequest) routedRequest {}\nfunc handleChatRoute(ctx context.Context) {}\n",
	})
	if countEvidenceNodesByKind(graph, "file") != 1 {
		t.Fatalf("expected one file node, got %#v", graph)
	}
	if countEvidenceNodesByKind(graph, "symbol") < 2 {
		t.Fatalf("expected symbol nodes, got %#v", graph)
	}
	if got := countSupportedEvidenceCitations("Evidence: `src/request.go` :: `chooseRoute`", graph); got != 1 {
		t.Fatalf("countSupportedEvidenceCitations() = %d", got)
	}
}

func TestBuildChatEvidenceGraph_FiltersTestSymbolsForArchitectureQuery(t *testing.T) {
	t.Parallel()
	req := routeRequest{
		Message: "Explain the internal route flow.",
		Mode:    "ask",
	}
	graph := buildChatEvidenceGraph(req, []string{"src/request_test.go"}, map[string]string{
		"src/request_test.go": "func TestChooseRoute(t *testing.T) {}\nfunc helperFixture() {}\n",
	})
	for _, node := range graph.Nodes {
		if node.Kind == "symbol" {
			t.Fatalf("expected test-only symbols to be filtered, got %#v", graph)
		}
	}
}

func TestExtractLikelySnippetSymbols_IgnoresCommentsAndProse(t *testing.T) {
	t.Parallel()
	snippet := `// evidence graph includes symbol names
/* type names should not count */
func chooseRoute(req routeRequest) routedRequest { return routedRequest{} }
# function fakeShellShouldNotCount
interface routePlanner {}
`
	got := extractLikelySnippetSymbols(snippet)
	if len(got) != 2 || got[0] != "chooseRoute" || got[1] != "routePlanner" {
		t.Fatalf("extractLikelySnippetSymbols() = %#v", got)
	}
}
