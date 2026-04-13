package main

import (
	"os"
	"path/filepath"
	"strings"
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

func TestInferWorkspaceChatTargets_PrefersCoreImplementationOverClientSurface(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	files := map[string]string{
		"packages/dorkpipe/lib/cmd/dorkpipe/request.go":                              "package main\n\nfunc handleChatRoute(ctx context.Context) {}\nfunc buildWorkspaceChatContext(root string, req routeRequest) workspaceChatContext {}\n",
		"packages/dorkpipe/lib/cmd/dorkpipe/reasoning_runtime.go":                    "package main\n\ntype runtimePolicy struct{}\nfunc resolveRuntimePolicy() {}\n",
		"packages/pipeon/resolvers/pipeon/vscode-extension/src/extension.ts":         "export function resolveDorkpipeInvocation() {}\nconst localFirst = true;\n",
		"packages/pipeon/resolvers/pipeon/vscode-extension/src/webview/chat.ts":      "export function renderRunInspector() {}\n",
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
		Message: "Explain how Ask mode now handles a plain-English architecture question after the v2 reasoning runtime changes.",
	})
	if len(got) == 0 {
		t.Fatalf("expected inferred targets, got none")
	}
	for _, rel := range got {
		if strings.Contains(rel, "vscode-extension") {
			t.Fatalf("expected client surface file to be pruned when core implementation exists, got %#v", got)
		}
	}
	if got[0] != "packages/dorkpipe/lib/cmd/dorkpipe/request.go" && got[0] != "packages/dorkpipe/lib/cmd/dorkpipe/reasoning_runtime.go" {
		t.Fatalf("expected core implementation file first, got %#v", got)
	}
}

func TestInferWorkspaceChatTargets_RequiresDenseArchitectureMatches(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	files := map[string]string{
		"packages/dorkpipe/lib/cmd/dorkpipe/request.go":           "package main\n\nfunc chooseRoute(req routeRequest) routedRequest {}\nfunc handleChatRoute(ctx context.Context) {}\nfunc buildWorkspaceChatContext(root string, req routeRequest) workspaceChatContext {}\n",
		"packages/dorkpipe/lib/cmd/dorkpipe/structured_trace.go":  "package main\n\nfunc writePreparedArtifactBundle() {}\nfunc deriveStructuredEdits() {}\n// validation status for artifacts\n",
		"packages/pipeon/apps/pipeon-launcher/src/BasicModeWidget.cpp": "namespace pipeon { class BasicModeWidget {}; }\n",
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
		if rel == "packages/dorkpipe/lib/cmd/dorkpipe/structured_trace.go" || rel == "packages/pipeon/apps/pipeon-launcher/src/BasicModeWidget.cpp" {
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
	root := t.TempDir()
	writeTestFile(t, root, "src/request.go", "package main\n\nfunc chooseRoute(req routeRequest) routedRequest { return routedRequest{} }\nfunc handleChatRoute(ctx context.Context) {}\n")
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
		Evidence: buildChatEvidenceGraph(root, req, []string{"src/request.go"}, map[string]string{
			"src/request.go": "func chooseRoute(req routeRequest) routedRequest {}\nfunc handleChatRoute(ctx context.Context) {}\n",
		}, extractChatSearchTerms(req.Message)),
	}
	answer := "## Confirmed\n- Ask mode routes conversational requests through `chooseRoute`. Evidence: `src/request.go` :: `chooseRoute`\n- Chat execution runs through `handleChatRoute`. Evidence: `src/request.go` :: `handleChatRoute`\n\n## Uncertain\n- Anything beyond the retrieved snippet."
	got := validateChatAnswer(answer, req, ctx)
	if !got.Passed {
		t.Fatalf("validateChatAnswer() = %#v", got)
	}
}

func TestValidateChatAnswer_RejectsUnsupportedClaims(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeTestFile(t, root, "src/request.go", "package main\n\nfunc chooseRoute(req routeRequest) routedRequest { return routedRequest{} }\n")
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
		Evidence: buildChatEvidenceGraph(root, req, []string{"src/request.go"}, map[string]string{
			"src/request.go": "func chooseRoute(req routeRequest) routedRequest {}\n",
		}, extractChatSearchTerms(req.Message)),
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

func TestValidateChatAnswer_RequiresMultipleCitationsForArchitectureWhenEvidenceSupportsIt(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeTestFile(t, root, "src/request.go", "package main\n\nfunc chooseRoute(req routeRequest) routedRequest { return routedRequest{} }\nfunc handleChatRoute(ctx context.Context) {}\n")
	req := routeRequest{
		Message: "Explain how Ask mode routes and validates architecture questions internally.",
		Mode:    "ask",
	}
	ctx := workspaceChatContext{
		Text: "Relevant file: src/request.go\n\n```text\nfunc chooseRoute(req routeRequest) routedRequest {}\nfunc handleChatRoute(ctx context.Context) {}\n```",
		Targets: []string{"src/request.go"},
		Snippets: map[string]string{
			"src/request.go": "func chooseRoute(req routeRequest) routedRequest {}\nfunc handleChatRoute(ctx context.Context) {}\n",
		},
		Evidence: buildChatEvidenceGraph(root, req, []string{"src/request.go"}, map[string]string{
			"src/request.go": "func chooseRoute(req routeRequest) routedRequest {}\nfunc handleChatRoute(ctx context.Context) {}\n",
		}, extractChatSearchTerms(req.Message)),
	}
	answer := "## Confirmed\n- Ask mode routes architecture requests through `chooseRoute`. Evidence: `src/request.go` :: `chooseRoute`\n\n## Uncertain\n- More detail is unclear."
	got := validateChatAnswer(answer, req, ctx)
	if got.Passed {
		t.Fatalf("expected architecture answer to require more citation coverage, got %#v", got)
	}
	if len(got.Issues) == 0 || !strings.Contains(got.Issues[0], "need at least 2") {
		t.Fatalf("expected multi-citation validation issue, got %#v", got)
	}
}

func TestValidateChatAnswer_RejectsMetaPolicyArchitectureAnswer(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeTestFile(t, root, "src/request.go", "package main\n\nfunc handleChatRoute(ctx context.Context) {}\nfunc resolveRuntimePolicy() {}\n")
	req := routeRequest{
		Message: "Explain how Ask mode handles architecture questions internally.",
		Mode:    "ask",
	}
	ctx := workspaceChatContext{
		Text: "Relevant file: src/request.go\n\n```text\nfunc handleChatRoute(ctx context.Context) {}\nfunc resolveRuntimePolicy() {}\n```",
		Targets: []string{"src/request.go"},
		Snippets: map[string]string{
			"src/request.go": "func handleChatRoute(ctx context.Context) {}\nfunc resolveRuntimePolicy() {}\n",
		},
		Evidence: buildChatEvidenceGraph(root, req, []string{"src/request.go"}, map[string]string{
			"src/request.go": "func handleChatRoute(ctx context.Context) {}\nfunc resolveRuntimePolicy() {}\n",
		}, extractChatSearchTerms(req.Message)),
	}
	answer := "## Confirmed\n- Every substantive claim should be code-anchored with exact citations.\n- If confidence is low, abstain and list only confirmed evidence.\n\n## Uncertain\n- More detail is unclear."
	got := validateChatAnswer(answer, req, ctx)
	if got.Passed {
		t.Fatalf("expected meta-policy answer to fail, got %#v", got)
	}
	if len(got.Issues) == 0 || !strings.Contains(strings.Join(got.Issues, " "), "response policy") {
		t.Fatalf("expected meta-policy validation issue, got %#v", got)
	}
}

func TestValidateChatAnswer_RejectsWrongNearbySymbolCitation(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeTestFile(t, root, "src/request.go", "package main\n\nfunc beginReasoningRun() {}\nfunc handleChatRoute(ctx context.Context) {}\n")
	req := routeRequest{
		Message: "Explain how Ask mode handles architecture questions internally.",
		Mode:    "ask",
	}
	ctx := workspaceChatContext{
		Text: "Relevant file: src/request.go\n\n```text\nfunc beginReasoningRun() {}\nfunc handleChatRoute(ctx context.Context) {}\n```",
		Targets: []string{"src/request.go"},
		Snippets: map[string]string{
			"src/request.go": "func beginReasoningRun() {}\nfunc handleChatRoute(ctx context.Context) {}\n",
		},
		Evidence: buildChatEvidenceGraph(root, req, []string{"src/request.go"}, map[string]string{
			"src/request.go": "func beginReasoningRun() {}\nfunc handleChatRoute(ctx context.Context) {}\n",
		}, extractChatSearchTerms(req.Message)),
	}
	answer := "## Confirmed\n- Ask mode handles architecture questions through chat routing. Evidence: `src/request.go` :: `beginReasoningRun`\n- Ask mode runs through `handleChatRoute`. Evidence: `src/request.go` :: `handleChatRoute`\n\n## Uncertain\n- More detail is unclear."
	got := validateChatAnswer(answer, req, ctx)
	if got.Passed {
		t.Fatalf("expected wrong nearby citation to fail, got %#v", got)
	}
	if !strings.Contains(strings.Join(got.Issues, " "), "weak evidence bindings") {
		t.Fatalf("expected weak binding issue, got %#v", got)
	}
}

func TestValidateChatAnswer_RejectsRouteMismatchedCitation(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeTestFile(t, root, "src/request.go", "package main\n\nfunc handleEditRoute(ctx context.Context) {}\nfunc buildChatAnswerRepairPrompt() {}\n")
	req := routeRequest{
		Message: "Explain how Ask mode repairs architecture answers internally.",
		Mode:    "ask",
	}
	ctx := workspaceChatContext{
		Text: "Relevant file: src/request.go\n\n```text\nfunc handleEditRoute(ctx context.Context) {}\nfunc buildChatAnswerRepairPrompt() {}\n```",
		Targets: []string{"src/request.go"},
		Snippets: map[string]string{
			"src/request.go": "func handleEditRoute(ctx context.Context) {}\nfunc buildChatAnswerRepairPrompt() {}\n",
		},
		Evidence: buildChatEvidenceGraph(root, req, []string{"src/request.go"}, map[string]string{
			"src/request.go": "func handleEditRoute(ctx context.Context) {}\nfunc buildChatAnswerRepairPrompt() {}\n",
		}, extractChatSearchTerms(req.Message)),
	}
	answer := "## Confirmed\n- Ask mode repairs unsupported architecture answers with `buildChatAnswerRepairPrompt`. Evidence: `src/request.go` :: `handleEditRoute`\n- Repair prompting is built by `buildChatAnswerRepairPrompt`. Evidence: `src/request.go` :: `buildChatAnswerRepairPrompt`\n\n## Uncertain\n- More detail is unclear."
	got := validateChatAnswer(answer, req, ctx)
	if got.Passed {
		t.Fatalf("expected route-mismatched citation to fail, got %#v", got)
	}
	if !strings.Contains(strings.Join(got.Issues, " "), "weak evidence bindings") {
		t.Fatalf("expected weak binding issue, got %#v", got)
	}
}

func TestBuildChatEvidenceGraph_IncludesFileAndSymbolNodes(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeTestFile(t, root, "src/request.go", "package main\n\nfunc chooseRoute(req routeRequest) routedRequest { return routedRequest{} }\nfunc handleChatRoute(ctx context.Context) {}\n")
	req := routeRequest{
		Message: "Explain the ask route.",
		Mode:    "ask",
	}
	graph := buildChatEvidenceGraph(root, req, []string{"src/request.go"}, map[string]string{
		"src/request.go": "func chooseRoute(req routeRequest) routedRequest {}\nfunc handleChatRoute(ctx context.Context) {}\n",
	}, extractChatSearchTerms(req.Message))
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
	root := t.TempDir()
	writeTestFile(t, root, "src/request_test.go", "package main\n\nfunc TestChooseRoute(t *testing.T) {}\nfunc helperFixture() {}\n")
	req := routeRequest{
		Message: "Explain the internal route flow.",
		Mode:    "ask",
	}
	graph := buildChatEvidenceGraph(root, req, []string{"src/request_test.go"}, map[string]string{
		"src/request_test.go": "func TestChooseRoute(t *testing.T) {}\nfunc helperFixture() {}\n",
	}, extractChatSearchTerms(req.Message))
	for _, node := range graph.Nodes {
		if node.Kind == "symbol" {
			t.Fatalf("expected test-only symbols to be filtered, got %#v", graph)
		}
	}
}

func TestBuildChatEvidenceGraph_PrioritizesExecutionSymbolsForArchitectureQuery(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeTestFile(t, root, "src/request.go", "package main\n\nfunc chooseRoute(req routeRequest) routedRequest { return routedRequest{} }\nfunc handleChatRoute(ctx context.Context) {}\nfunc handleInspectRoute(ctx context.Context) {}\nfunc buildWorkspaceChatContext(root string, req routeRequest) workspaceChatContext { return workspaceChatContext{} }\nfunc validateChatAnswer(answer string, req routeRequest, chatContext workspaceChatContext) chatAnswerValidation { return chatAnswerValidation{} }\nfunc resolveRuntimePolicy() {}\nfunc citationSupportsClaim() bool { return false }\n")
	req := routeRequest{
		Message: "Explain how Ask mode handles architecture questions internally.",
		Mode:    "ask",
	}
	graph := buildChatEvidenceGraph(root, req, []string{"src/request.go"}, map[string]string{
		"src/request.go": "func chooseRoute(req routeRequest) routedRequest { return routedRequest{} }\nfunc handleChatRoute(ctx context.Context) {}\nfunc handleInspectRoute(ctx context.Context) {}\nfunc buildWorkspaceChatContext(root string, req routeRequest) workspaceChatContext { return workspaceChatContext{} }\nfunc validateChatAnswer(answer string, req routeRequest, chatContext workspaceChatContext) chatAnswerValidation { return chatAnswerValidation{} }\nfunc resolveRuntimePolicy() {}\nfunc citationSupportsClaim() bool { return false }\n",
	}, extractChatSearchTerms(req.Message))
	var symbols []chatEvidenceNode
	for _, node := range graph.Nodes {
		if node.Kind == "symbol" {
			symbols = append(symbols, node)
		}
	}
	if len(symbols) < 4 {
		t.Fatalf("expected multiple symbols, got %#v", graph)
	}
	got := map[string]bool{}
	for _, node := range symbols {
		got[node.Symbol] = true
	}
	for _, expected := range []string{"handleChatRoute", "chooseRoute", "buildWorkspaceChatContext", "validateChatAnswer", "resolveRuntimePolicy"} {
		if !got[expected] {
			t.Fatalf("expected architecture graph to retain %s, got %#v", expected, symbols)
		}
	}
	if got["handleInspectRoute"] || got["citationSupportsClaim"] {
		t.Fatalf("expected architecture graph to suppress sibling/helper symbols, got %#v", symbols)
	}
}

func TestBuildChatEvidenceGraph_KeepsFocusedFlowStagesInLargeFiles(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeTestFile(t, root, "src/request.go", `package main

func handleChatRoute(ctx context.Context) {}
func handleInspectRoute(ctx context.Context) {}
func handleEditRoute(ctx context.Context) {}
func chooseRoute(req routeRequest) routedRequest { return routedRequest{} }
func buildWorkspaceChatContext(root string, req routeRequest) workspaceChatContext { return workspaceChatContext{} }
func validateChatAnswer(answer string, req routeRequest, chatContext workspaceChatContext) chatAnswerValidation { return chatAnswerValidation{} }
func resolveRuntimePolicy() {}
func buildChatPrompt() {}
func buildChatAnswerRepairPrompt() {}
func buildChatEvidenceGraph() {}
func summarizeStrictEvidenceGraph() {}
func preferredChatEvidenceNodes() {}
func citationSupportsClaim() bool { return false }
func isRouteHandlerLikeSymbol(symbol string) bool { return false }
func matchesRouteFocus(symbol string, focusTerms []string) bool { return false }
func extractChatSearchTerms(message string) []string { return nil }
func extractLikelySnippetSymbolsNearTerms(snippet string, searchTerms []string) []string { return nil }
func parseAnswerCitationBindings(answer string) []answerCitationBinding { return nil }
`)
	req := routeRequest{
		Message: "Explain how Ask mode now handles a plain-English architecture question after the v2 reasoning runtime changes.",
		Mode:    "ask",
	}
	graph := buildChatEvidenceGraph(root, req, []string{"src/request.go"}, map[string]string{
		"src/request.go": "func handleChatRoute(ctx context.Context) {}\nfunc chooseRoute(req routeRequest) routedRequest { return routedRequest{} }\nfunc buildWorkspaceChatContext(root string, req routeRequest) workspaceChatContext { return workspaceChatContext{} }\nfunc validateChatAnswer(answer string, req routeRequest, chatContext workspaceChatContext) chatAnswerValidation { return chatAnswerValidation{} }\nfunc resolveRuntimePolicy() {}\n",
	}, extractChatSearchTerms(req.Message))
	got := map[string]bool{}
	for _, node := range graph.Nodes {
		if node.Kind == "symbol" {
			got[node.Symbol] = true
		}
	}
	for _, expected := range []string{"handleChatRoute", "chooseRoute", "buildWorkspaceChatContext", "validateChatAnswer"} {
		if !got[expected] {
			t.Fatalf("expected graph to retain focused Ask flow stage %s, got %#v", expected, graph.Nodes)
		}
	}
	if got["handleInspectRoute"] || got["handleEditRoute"] || got["citationSupportsClaim"] {
		t.Fatalf("expected graph to suppress distracting symbols, got %#v", graph.Nodes)
	}
}

func TestBuildEvidenceOnlyChatFallback_UsesStrictEvidenceCitations(t *testing.T) {
	t.Parallel()
	graph := chatEvidenceGraph{
		Nodes: []chatEvidenceNode{
			{ID: "request", Kind: "request", Summary: "Explain ask mode."},
			{ID: "file:src/request.go", Kind: "file", File: "src/request.go", Summary: "request handlers"},
			{ID: "symbol:src/request.go:handleChatRoute", Kind: "symbol", File: "src/request.go", Symbol: "handleChatRoute", Summary: "chat handler"},
		},
		Edges: []chatEvidenceEdge{
			{From: "request", To: "file:src/request.go", Kind: "grounds"},
			{From: "file:src/request.go", To: "symbol:src/request.go:handleChatRoute", Kind: "contains"},
		},
	}
	out := buildEvidenceOnlyChatFallback(routeRequest{Message: "Explain ask mode flow."}, workspaceChatContext{Evidence: graph}, chatAnswerValidation{Issues: []string{"missing evidence citations to retrieved file/symbol nodes"}})
	if !strings.Contains(out, "Evidence: `src/request.go` :: `handleChatRoute`") {
		t.Fatalf("fallback missing strict evidence citation: %q", out)
	}
	if strings.Contains(out, "evidence graph includes symbol") {
		t.Fatalf("fallback leaked internal graph phrasing: %q", out)
	}
}

func TestBuildEvidenceOnlyChatFallback_PrefersTopScoredFlowSymbols(t *testing.T) {
	t.Parallel()
	graph := chatEvidenceGraph{
		Nodes: []chatEvidenceNode{
			{ID: "request", Kind: "request", Summary: "Explain ask flow."},
			{ID: "file:src/request.go", Kind: "file", File: "src/request.go", Summary: "request handlers"},
			{ID: "symbol:src/request.go:handleChatRoute", Kind: "symbol", File: "src/request.go", Symbol: "handleChatRoute", Summary: "chat handler", Score: 20},
			{ID: "symbol:src/request.go:resolveRuntimePolicy", Kind: "symbol", File: "src/request.go", Symbol: "resolveRuntimePolicy", Summary: "runtime policy", Score: 16},
			{ID: "symbol:src/request.go:citationSupportsClaim", Kind: "symbol", File: "src/request.go", Symbol: "citationSupportsClaim", Summary: "binding helper", Score: -3},
		},
	}
	out := buildEvidenceOnlyChatFallback(routeRequest{Message: "Explain ask mode flow."}, workspaceChatContext{Evidence: graph}, chatAnswerValidation{})
	if strings.Contains(out, "citationSupportsClaim") {
		t.Fatalf("fallback should prefer top scored flow symbols, got %q", out)
	}
	if !strings.Contains(out, "handleChatRoute") || !strings.Contains(out, "resolveRuntimePolicy") {
		t.Fatalf("fallback missing preferred flow symbols: %q", out)
	}
}

func TestBuildEvidenceOnlyChatFallback_SuppressesSiblingRouteHandlersAndHelpers(t *testing.T) {
	t.Parallel()
	graph := chatEvidenceGraph{
		Nodes: []chatEvidenceNode{
			{ID: "request", Kind: "request", Summary: "Explain ask flow."},
			{ID: "file:src/request.go", Kind: "file", File: "src/request.go", Summary: "request handlers"},
			{ID: "symbol:src/request.go:handleChatRoute", Kind: "symbol", File: "src/request.go", Symbol: "handleChatRoute", Score: 20},
			{ID: "symbol:src/request.go:handleInspectRoute", Kind: "symbol", File: "src/request.go", Symbol: "handleInspectRoute", Score: 18},
			{ID: "symbol:src/request.go:handleEditRoute", Kind: "symbol", File: "src/request.go", Symbol: "handleEditRoute", Score: 17},
			{ID: "symbol:src/request.go:citationSupportsClaim", Kind: "symbol", File: "src/request.go", Symbol: "citationSupportsClaim", Score: 15},
			{ID: "symbol:src/request.go:resolveRuntimePolicy", Kind: "symbol", File: "src/request.go", Symbol: "resolveRuntimePolicy", Score: 14},
		},
	}
	out := buildEvidenceOnlyChatFallback(routeRequest{Message: "Explain ask mode flow."}, workspaceChatContext{Evidence: graph}, chatAnswerValidation{})
	if strings.Contains(out, "handleInspectRoute") || strings.Contains(out, "handleEditRoute") {
		t.Fatalf("fallback should keep only one route handler representative, got %q", out)
	}
	if strings.Contains(out, "citationSupportsClaim") {
		t.Fatalf("fallback should suppress helper symbol, got %q", out)
	}
	if !strings.Contains(out, "handleChatRoute") || !strings.Contains(out, "resolveRuntimePolicy") {
		t.Fatalf("fallback missing expected flow symbols, got %q", out)
	}
}

func TestBuildEvidenceOnlyChatFallback_PrefersQuestionFocusedRouteHandler(t *testing.T) {
	t.Parallel()
	graph := chatEvidenceGraph{
		Nodes: []chatEvidenceNode{
			{ID: "request", Kind: "request", Summary: "Explain ask/chat flow."},
			{ID: "file:src/request.go", Kind: "file", File: "src/request.go"},
			{ID: "symbol:src/request.go:handleChatRoute", Kind: "symbol", File: "src/request.go", Symbol: "handleChatRoute", Score: 18},
			{ID: "symbol:src/request.go:handleInspectRoute", Kind: "symbol", File: "src/request.go", Symbol: "handleInspectRoute", Score: 20},
			{ID: "symbol:src/request.go:isRouteHandlerLikeSymbol", Kind: "symbol", File: "src/request.go", Symbol: "isRouteHandlerLikeSymbol", Score: 19},
			{ID: "symbol:src/request.go:resolveRuntimePolicy", Kind: "symbol", File: "src/request.go", Symbol: "resolveRuntimePolicy", Score: 14},
		},
	}
	out := buildEvidenceOnlyChatFallback(routeRequest{Message: "Explain how Ask mode now handles a plain-English architecture question."}, workspaceChatContext{Evidence: graph}, chatAnswerValidation{})
	if !strings.Contains(out, "handleChatRoute") {
		t.Fatalf("fallback should include chat-focused route handler, got %q", out)
	}
	if strings.Contains(out, "handleInspectRoute") || strings.Contains(out, "isRouteHandlerLikeSymbol") {
		t.Fatalf("fallback should suppress non-focused route/helper symbol, got %q", out)
	}
}

func TestBuildEvidenceOnlyChatFallback_ProducesValidatorPassingAnswer(t *testing.T) {
	t.Parallel()
	req := routeRequest{
		Message: "Explain how Ask mode now handles a plain-English architecture question after the v2 reasoning runtime changes.",
		Mode:    "ask",
	}
	ctx := workspaceChatContext{
		Targets: []string{
			"packages/dorkpipe/lib/cmd/dorkpipe/request.go",
			"packages/dorkpipe/lib/cmd/dorkpipe/reasoning_runtime.go",
		},
		Evidence: chatEvidenceGraph{
			Nodes: []chatEvidenceNode{
				{ID: "request", Kind: "request", Summary: req.Message},
				{ID: "file:request", Kind: "file", File: "packages/dorkpipe/lib/cmd/dorkpipe/request.go"},
				{ID: "file:runtime", Kind: "file", File: "packages/dorkpipe/lib/cmd/dorkpipe/reasoning_runtime.go"},
				{ID: "symbol:request:chooseRoute", Kind: "symbol", File: "packages/dorkpipe/lib/cmd/dorkpipe/request.go", Symbol: "chooseRoute", Score: 19},
				{ID: "symbol:request:buildWorkspaceChatContext", Kind: "symbol", File: "packages/dorkpipe/lib/cmd/dorkpipe/request.go", Symbol: "buildWorkspaceChatContext", Score: 17},
				{ID: "symbol:request:handleChatRoute", Kind: "symbol", File: "packages/dorkpipe/lib/cmd/dorkpipe/request.go", Symbol: "handleChatRoute", Score: 18},
				{ID: "symbol:request:validateChatAnswer", Kind: "symbol", File: "packages/dorkpipe/lib/cmd/dorkpipe/request.go", Symbol: "validateChatAnswer", Score: 16},
				{ID: "symbol:runtime:resolveRuntimePolicy", Kind: "symbol", File: "packages/dorkpipe/lib/cmd/dorkpipe/reasoning_runtime.go", Symbol: "resolveRuntimePolicy", Score: 16},
			},
		},
	}
	answer := buildEvidenceOnlyChatFallback(req, ctx, chatAnswerValidation{
		Issues: []string{"insufficient evidence citations to retrieved file/symbol nodes: got 0, need at least 2"},
	})
	got := validateChatAnswer(answer, req, ctx)
	if !got.Passed {
		t.Fatalf("fallback should validate once emitted, got %#v with answer %q", got, answer)
	}
	for _, symbol := range []string{"chooseRoute", "buildWorkspaceChatContext", "handleChatRoute", "validateChatAnswer"} {
		if !strings.Contains(answer, "`"+symbol+"`") {
			t.Fatalf("fallback should synthesize architecture flow symbol %s, got %q", symbol, answer)
		}
	}
	if strings.Contains(answer, "Retained flow handler") {
		t.Fatalf("fallback should use flow-oriented wording, got %q", answer)
	}
}

func TestFinalizeEvidenceOnlyChatFallback_StripsStaleSuppressedIssuesAfterPassing(t *testing.T) {
	t.Parallel()
	req := routeRequest{
		Message: "Explain how Ask mode now handles a plain-English architecture question after the v2 reasoning runtime changes.",
		Mode:    "ask",
	}
	ctx := workspaceChatContext{
		Targets: []string{
			"packages/dorkpipe/lib/cmd/dorkpipe/request.go",
			"packages/dorkpipe/lib/cmd/dorkpipe/reasoning_runtime.go",
		},
		Evidence: chatEvidenceGraph{
			Nodes: []chatEvidenceNode{
				{ID: "request", Kind: "request", Summary: req.Message},
				{ID: "file:request", Kind: "file", File: "packages/dorkpipe/lib/cmd/dorkpipe/request.go"},
				{ID: "file:runtime", Kind: "file", File: "packages/dorkpipe/lib/cmd/dorkpipe/reasoning_runtime.go"},
				{ID: "symbol:request:handleChatRoute", Kind: "symbol", File: "packages/dorkpipe/lib/cmd/dorkpipe/request.go", Symbol: "handleChatRoute", Score: 18},
				{ID: "symbol:request:buildWorkspaceChatContext", Kind: "symbol", File: "packages/dorkpipe/lib/cmd/dorkpipe/request.go", Symbol: "buildWorkspaceChatContext", Score: 17},
				{ID: "symbol:request:validateChatAnswer", Kind: "symbol", File: "packages/dorkpipe/lib/cmd/dorkpipe/request.go", Symbol: "validateChatAnswer", Score: 16},
				{ID: "symbol:runtime:resolveRuntimePolicy", Kind: "symbol", File: "packages/dorkpipe/lib/cmd/dorkpipe/reasoning_runtime.go", Symbol: "resolveRuntimePolicy", Score: 15},
			},
		},
	}
	answer, validation := finalizeEvidenceOnlyChatFallback(req, ctx, chatAnswerValidation{
		Issues: []string{"insufficient evidence citations to retrieved file/symbol nodes: got 0, need at least 2"},
	})
	if !validation.Passed {
		t.Fatalf("expected finalized fallback to pass validation, got %#v with answer %q", validation, answer)
	}
	if strings.Contains(answer, "Suppressed unsupported claims:") {
		t.Fatalf("expected stale suppressed issues to be stripped from passing fallback, got %q", answer)
	}
}

func writeTestFile(t *testing.T, root, rel, body string) {
	t.Helper()
	target := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", rel, err)
	}
	if err := os.WriteFile(target, []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", rel, err)
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

func TestExtractLikelySnippetSymbolsNearTerms_FocusesNearbyDeclarations(t *testing.T) {
	t.Parallel()
	snippet := `func unrelatedOne() {}
// route flow discussion
func chooseRoute(req routeRequest) routedRequest { return routedRequest{} }
func handleChatRoute(ctx context.Context) {}
func unrelatedTwo() {}
`
	got := extractLikelySnippetSymbolsNearTerms(snippet, []string{"route", "flow"})
	if len(got) == 0 || got[0] != "chooseRoute" {
		t.Fatalf("extractLikelySnippetSymbolsNearTerms() = %#v", got)
	}
}
