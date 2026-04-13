package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

type routeRequest struct {
	Message       string
	ActiveFile    string
	OpenFiles     []string
	SelectionText string
	Mode          string
}

type stringListFlag []string

func (s *stringListFlag) String() string {
	return strings.Join(*s, ",")
}

func (s *stringListFlag) Set(value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	*s = append(*s, value)
	return nil
}

type routedRequest struct {
	Route  string
	Action string
	Arg    string
	Reason string
}

func requestCmd(argv []string) {
	fs := flag.NewFlagSet("request", flag.ExitOnError)
	workdir := fs.String("workdir", "", "working directory (default cwd)")
	message := fs.String("message", "", "request text")
	activeFile := fs.String("active-file", "", "repo-relative active file hint")
	var openFiles stringListFlag
	fs.Var(&openFiles, "open-file", "repo-relative open file hint (repeatable)")
	selectionText := fs.String("selection-text", "", "selection hint")
	mode := fs.String("mode", "ask", "request mode: ask, agent, or plan")
	executeRoute := fs.Bool("execute", false, "execute the routed request and stream events")
	model := fs.String("model", "", "Ollama model override")
	ollamaHost := fs.String("ollama-host", "", "Ollama host override")
	numCtx := fs.Int("num-ctx", 0, "Ollama context window override")
	_ = fs.Parse(argv)
	if strings.TrimSpace(*message) == "" {
		fmt.Fprintln(os.Stderr, "request: --message is required")
		os.Exit(2)
	}

	wd := *workdir
	if wd == "" {
		var err error
		wd, err = os.Getwd()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
	absWd, err := filepath.Abs(wd)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	reqID := fmt.Sprintf("req_%d", timeNowUnixNano())
	emitEditEvent(reqID, "received", "Received request", 0.05, nil)
	req := routeRequest{
		Message:       strings.TrimSpace(*message),
		ActiveFile:    strings.TrimSpace(*activeFile),
		OpenFiles:     uniqueNonEmpty(openFiles),
		SelectionText: strings.TrimSpace(*selectionText),
		Mode:          normalizeRequestMode(*mode),
	}
	routed := chooseRoute(req)
	emitEditEvent(reqID, "routed", fmt.Sprintf("Route: %s", routed.Route), 0.22, map[string]any{
		"route":  routed.Route,
		"action": routed.Action,
		"arg":    routed.Arg,
		"reason": routed.Reason,
		"mode":   req.Mode,
	})

	if !*executeRoute {
		emitEditDone(reqID, "Request routed.", map[string]any{
			"route":   routed.Route,
			"action":  routed.Action,
			"arg":     routed.Arg,
			"reason":  routed.Reason,
			"mode":    req.Mode,
			"workdir": absWd,
		})
		return
	}

	ctx := context.Background()
	host, chosenModel := resolveModelConfig(strings.TrimSpace(*ollamaHost), strings.TrimSpace(*model))
	chosenNumCtx := resolveNumCtx(*numCtx)
	switch routed.Route {
	case "inspect":
		handleInspectRoute(ctx, reqID, absWd, req, routed.Action, routed.Arg)
	case "edit":
		if routed.Arg != "" {
			req.Message = routed.Arg
		}
		handleEditRoute(ctx, reqID, absWd, req, host, chosenModel, chosenNumCtx)
	default:
		handleChatRoute(ctx, reqID, absWd, req, host, chosenModel, chosenNumCtx)
	}
}

func chooseRoute(req routeRequest) routedRequest {
	raw := strings.TrimSpace(req.Message)
	msg := strings.ToLower(raw)
	mode := normalizeRequestMode(req.Mode)
	if msg == "" {
		return routedRequest{Route: "chat", Reason: "empty fallback"}
	}

	if strings.HasPrefix(msg, "/") {
		name, arg := parseSlashCommand(raw)
		switch name {
		case "edit":
			return routedRequest{Route: "edit", Action: "edit", Arg: arg, Reason: "explicit slash edit"}
		case "context", "status", "bundle", "test", "ci", "workflow", "validate", "workflow-validate", "callstack", "heap":
			return routedRequest{Route: "inspect", Action: name, Arg: arg, Reason: "explicit slash command"}
		default:
			return routedRequest{Route: "inspect", Action: "slash", Arg: raw, Reason: "explicit slash command"}
		}
	}

	if isInspectIntent(msg) {
		return routedRequest{Route: "inspect", Action: inspectAction(msg), Reason: "natural-language inspect request"}
	}

	switch mode {
	case "plan":
		if isEditIntent(msg, req.ActiveFile != "", req.SelectionText != "") {
			return routedRequest{Route: "chat", Reason: "plan mode prefers planning over mutation"}
		}
		return routedRequest{Route: "chat", Reason: "plan mode routes to planning chat"}
	case "agent":
		if isChatIntent(msg) {
			return routedRequest{Route: "chat", Reason: "agent mode kept a clearly conversational request in chat"}
		}
		if isEditIntent(msg, req.ActiveFile != "", req.SelectionText != "") {
			return routedRequest{Route: "edit", Reason: "agent mode with edit-oriented cues"}
		}
		return routedRequest{Route: "edit", Reason: "agent mode defaults to edit"}
	}

	if isChatIntent(msg) {
		return routedRequest{Route: "chat", Reason: "ask mode kept a conversational/information request in chat"}
	}
	if isEditIntent(msg, req.ActiveFile != "", req.SelectionText != "") {
		return routedRequest{Route: "edit", Reason: "ask mode detected edit-oriented request with code/workspace cues"}
	}

	return routedRequest{Route: "chat", Reason: "ask mode defaulted to chat"}
}

func parseSlashCommand(raw string) (string, string) {
	trimmed := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(raw), "/"))
	if trimmed == "" {
		return "", ""
	}
	parts := strings.Fields(trimmed)
	if len(parts) == 0 {
		return "", ""
	}
	name := strings.ToLower(parts[0])
	arg := strings.TrimSpace(strings.TrimPrefix(trimmed, parts[0]))
	return name, strings.TrimSpace(arg)
}

func handleInspectRoute(ctx context.Context, reqID, root string, req routeRequest, action, arg string) {
	switch action {
	case "bundle":
		emitEditEvent(reqID, "context_gathering", "Refreshing context bundle", 0.45, nil)
		out, err := runRepoCommand(ctx, root, "./packages/pipeon/resolvers/pipeon/bin/pipeon", "bundle")
		if err != nil {
			emitEditError(reqID, "INTERNAL_ERROR", fmt.Sprintf("Context refresh failed: %s", out), true)
			return
		}
		emitEditDone(reqID, codeFence("text", nonEmpty(out, "Context bundle refreshed.")), map[string]any{
			"route":             "inspect",
			"action":            "bundle",
			"validation_status": "not_applicable",
		})
	case "status":
		emitEditEvent(reqID, "context_gathering", "Checking local status", 0.45, nil)
		out, err := runRepoCommand(ctx, root, "./packages/pipeon/resolvers/pipeon/bin/pipeon", "status")
		if err != nil {
			emitEditError(reqID, "INTERNAL_ERROR", fmt.Sprintf("Status failed: %s", out), true)
			return
		}
		emitEditDone(reqID, codeFence("text", nonEmpty(out, "No status output.")), map[string]any{
			"route":             "inspect",
			"action":            "status",
			"validation_status": "not_applicable",
		})
	case "test":
		emitEditEvent(reqID, "context_gathering", "Running test workflow", 0.45, nil)
		out, err := runRepoCommand(ctx, root, "./src/bin/dockpipe", "--workflow", "test", "--workdir", ".", "--")
		if err != nil {
			emitEditError(reqID, "INTERNAL_ERROR", fmt.Sprintf("Test workflow failed: %s", out), true)
			return
		}
		emitEditDone(reqID, codeFence("text", nonEmpty(out, "Test workflow finished.")), map[string]any{
			"route":             "inspect",
			"action":            "test",
			"validation_status": "passed",
		})
	case "ci":
		emitEditEvent(reqID, "context_gathering", "Running ci-emulate workflow", 0.45, nil)
		out, err := runRepoCommand(ctx, root, "./src/bin/dockpipe", "--workflow", "ci-emulate", "--workdir", ".", "--")
		if err != nil {
			emitEditError(reqID, "INTERNAL_ERROR", fmt.Sprintf("ci-emulate failed: %s", out), true)
			return
		}
		emitEditDone(reqID, codeFence("text", nonEmpty(out, "ci-emulate finished.")), map[string]any{
			"route":             "inspect",
			"action":            "ci",
			"validation_status": "passed",
		})
	case "workflow":
		if strings.TrimSpace(arg) == "" {
			emitEditError(reqID, "INVALID_REQUEST", "Usage: /workflow <name>", false)
			return
		}
		emitEditEvent(reqID, "context_gathering", fmt.Sprintf("Running workflow %s", arg), 0.45, map[string]any{"workflow": arg})
		out, err := runRepoCommand(ctx, root, "./src/bin/dockpipe", "--workflow", arg, "--workdir", ".", "--")
		if err != nil {
			emitEditError(reqID, "INTERNAL_ERROR", fmt.Sprintf("Workflow %s failed: %s", arg, out), true)
			return
		}
		emitEditDone(reqID, codeFence("text", nonEmpty(out, "Workflow finished.")), map[string]any{
			"route":             "inspect",
			"action":            "workflow",
			"workflow":          arg,
			"validation_status": "passed",
		})
	case "validate", "workflow-validate":
		target := strings.TrimSpace(arg)
		if target == "" {
			emitEditError(reqID, "INVALID_REQUEST", fmt.Sprintf("Usage: /%s <path-to-config.yml>", action), false)
			return
		}
		emitEditEvent(reqID, "context_gathering", fmt.Sprintf("Validating workflow %s", target), 0.45, map[string]any{"target": target})
		out, err := runRepoCommand(ctx, root, "./src/bin/dockpipe", "workflow", "validate", target)
		if err != nil {
			emitEditError(reqID, "INTERNAL_ERROR", fmt.Sprintf("Workflow validation failed: %s", out), true)
			return
		}
		emitEditDone(reqID, codeFence("text", nonEmpty(out, "Workflow validation finished.")), map[string]any{
			"route":             "inspect",
			"action":            action,
			"target":            target,
			"validation_status": "passed",
		})
	case "callstack":
		emitEditEvent(reqID, "context_gathering", "Inspecting callstack candidates", 0.45, map[string]any{
			"target":      arg,
			"active_file": req.ActiveFile,
		})
		out, err := runRepoScript(ctx, root, "packages/dorkpipe/resolvers/dorkpipe/assets/scripts/inspect-callstack.sh", root, arg, req.ActiveFile, req.SelectionText)
		if err != nil {
			emitEditError(reqID, "INTERNAL_ERROR", fmt.Sprintf("Callstack inspection failed: %s", out), true)
			return
		}
		emitEditDone(reqID, codeFence("text", nonEmpty(out, "No callstack candidates found.")), map[string]any{
			"route":             "inspect",
			"action":            "callstack",
			"target":            arg,
			"validation_status": "not_applicable",
			"active_file":       req.ActiveFile,
		})
	case "heap":
		emitEditEvent(reqID, "context_gathering", "Inspecting heap or memory profile", 0.45, map[string]any{
			"target": arg,
		})
		out, err := runRepoScript(ctx, root, "packages/dorkpipe/resolvers/dorkpipe/assets/scripts/inspect-heap.sh", root, arg)
		if err != nil {
			emitEditError(reqID, "INTERNAL_ERROR", fmt.Sprintf("Heap inspection failed: %s", out), true)
			return
		}
		emitEditDone(reqID, codeFence("text", nonEmpty(out, "No heap data available.")), map[string]any{
			"route":             "inspect",
			"action":            "heap",
			"target":            arg,
			"validation_status": "not_applicable",
		})
	default:
		emitEditEvent(reqID, "context_gathering", "Collecting focused workspace context", 0.45, nil)
		text, meta := buildWorkspaceChatContext(root, req)
		if strings.TrimSpace(text) == "" {
			emitEditDone(reqID, "No focused workspace context was collected for this request.", map[string]any{
				"route":  "inspect",
				"action": "context",
			})
			return
		}
		meta["route"] = "inspect"
		meta["action"] = "context"
		emitEditDone(reqID, fmt.Sprintf("Focused workspace context:\n\n%s", codeFence("markdown", text)), meta)
	}
}

func handleEditRoute(ctx context.Context, reqID, root string, req routeRequest, host, model string, numCtx int) {
	exe, err := os.Executable()
	if err != nil {
		emitEditError(reqID, "INTERNAL_ERROR", fmt.Sprintf("Could not resolve dorkpipe executable: %v", err), false)
		return
	}
	args := []string{
		"edit",
		"--workdir", root,
		"--message", req.Message,
	}
	if req.ActiveFile != "" {
		args = append(args, "--active-file", req.ActiveFile)
	}
	if req.SelectionText != "" {
		args = append(args, "--selection-text", req.SelectionText)
	}
	if host != "" {
		args = append(args, "--ollama-host", host)
	}
	if model != "" {
		args = append(args, "--model", model)
	}
	if numCtx > 0 {
		args = append(args, "--num-ctx", fmt.Sprintf("%d", numCtx))
	}
	if strings.TrimSpace(reqID) != "" {
		args = append(args, "--parent-request-id", reqID)
	}
	cmd := exec.CommandContext(ctx, exe, args...)
	cmd.Dir = root
	cmd.Env = os.Environ()
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		// The child edit flow emits structured error events for expected failures.
		// Avoid layering a generic exit-status wrapper on top of the real cause.
		return
	}
}

func handleChatRoute(ctx context.Context, reqID, root string, req routeRequest, host, model string, numCtx int) {
	emitEditEvent(reqID, "context_gathering", "Inspecting workspace context", 0.35, nil)
	contextText, contextMeta := buildWorkspaceChatContext(root, req)
	var mcpText string
	var mcpLoop *boundedMCPContextResult
	mcpDisc, mcpErr := discoverMCPContext(ctx)
	if mcpErr == nil && mcpDisc != nil {
		emitEditEvent(reqID, "context_gathering", "Consulting MCP bridge", 0.42, mcpMetadata(mcpDisc))
		mcpText = mcpSummaryText(mcpDisc)
		emitEditEvent(reqID, "retrieving", "Running bounded MCP context loop", 0.46, map[string]any{
			"step_cap": 5,
		})
		mcpLoop = runBoundedMCPContextLoop(ctx, req, mcpDisc)
		if mcpLoop != nil {
			if len(mcpLoop.Refined) > 0 {
				emitEditEvent(reqID, "retrieving", "Refining MCP context focus", 0.48, map[string]any{
					"refined_terms": mcpLoop.Refined,
				})
			}
			emitEditEvent(reqID, "retrieving", "Completed bounded MCP context loop", 0.5, map[string]any{
				"mcp_steps_used":  mcpLoop.StepsUsed,
				"mcp_search_hits": len(uniqueNonEmpty(mcpLoop.SearchHits)),
				"mcp_files_read":  len(uniqueNonEmpty(mcpLoop.ReadFiles)),
			})
		}
	}
	prompt := buildChatPrompt(root, req, contextText, mcpText, mcpLoop)
	emitEditEvent(reqID, "routed", fmt.Sprintf("Streaming from %s", model), 0.55, map[string]any{
		"route":   "chat",
		"model":   model,
		"num_ctx": numCtx,
	})
	answer, err := runChatModelStream(ctx, host, model, numCtx, prompt, func(piece string) {
		writeEvent(editEvent{
			ContractVersion: editContractVersion,
			RequestID:       reqID,
			Type:            "model_stream",
			Metadata: map[string]any{
				"text": piece,
			},
		})
	})
	if err != nil {
		emitEditError(reqID, "MODEL_UNAVAILABLE", fmt.Sprintf("Chat model failed: %v", err), true)
		return
	}
	status := map[string]any{
		"route":             "chat",
		"model":             model,
		"num_ctx":           numCtx,
		"mode":              normalizeRequestMode(req.Mode),
		"validation_status": "not_applicable",
		"active_file":       req.ActiveFile,
	}
	for key, value := range contextMeta {
		status[key] = value
	}
	for key, value := range mcpMetadata(mcpDisc) {
		status[key] = value
	}
	if mcpLoop != nil {
		status["mcp_steps_used"] = mcpLoop.StepsUsed
		status["mcp_search_hits"] = len(uniqueNonEmpty(mcpLoop.SearchHits))
		status["mcp_files_read"] = len(uniqueNonEmpty(mcpLoop.ReadFiles))
	}
	emitEditDone(reqID, nonEmpty(answer, "(No response text returned.)"), status)
}

func isInspectIntent(msg string) bool {
	return strings.Contains(msg, "context bundle") ||
		(strings.Contains(msg, "what context") || strings.Contains(msg, "show context")) ||
		(strings.Contains(msg, "status") && (strings.Contains(msg, "show") || strings.Contains(msg, "check") || msg == "status")) ||
		(strings.Contains(msg, "bundle context") || strings.Contains(msg, "refresh context")) ||
		strings.Contains(msg, "callstack") ||
		strings.Contains(msg, "stack trace") ||
		(strings.Contains(msg, "heap") && (strings.Contains(msg, "inspect") || strings.Contains(msg, "show") || strings.Contains(msg, "profile") || strings.Contains(msg, "memory")))
}

func inspectAction(msg string) string {
	switch {
	case strings.Contains(msg, "callstack") || strings.Contains(msg, "stack trace"):
		return "callstack"
	case strings.Contains(msg, "heap") || strings.Contains(msg, "memory profile"):
		return "heap"
	case strings.Contains(msg, "bundle context") || strings.Contains(msg, "refresh context"):
		return "bundle"
	case strings.Contains(msg, "status") && (strings.Contains(msg, "show") || strings.Contains(msg, "check") || msg == "status"):
		return "status"
	default:
		return "context"
	}
}

var (
	editActionPattern      = regexp.MustCompile(`\b(update|change|modify|edit|fix|rewrite|refactor|rename|add|remove|delete|clean up|implement|wire up|hook up|patch)\b`)
	editCodeTargetPattern  = regexp.MustCompile(`\b(file|code|function|component|extension|readme|workflow|config|script|test|panel|ui|chat|router|prompt|validation)\b`)
	editImperativePattern  = regexp.MustCompile(`^(update|change|modify|fix|rewrite|refactor|rename|add|remove|delete|implement|wire up|hook up|patch)\b`)
	editRequestVerbPattern = regexp.MustCompile(`\b(can you|please|could you|i want|we should|let'?s)\b`)
	chatIntentPattern      = regexp.MustCompile(`\b(how are you|hello|hi|hey|thanks|thank you|what do you think|explain|summarize|why|how does|what is|who are you|help me understand)\b`)
)

func isEditIntent(msg string, hasActiveFile, hasSelection bool) bool {
	if hasSelection && editActionPattern.MatchString(msg) {
		return true
	}
	if hasActiveFile && editActionPattern.MatchString(msg) {
		return true
	}
	if editImperativePattern.MatchString(msg) && editCodeTargetPattern.MatchString(msg) {
		return true
	}
	if editRequestVerbPattern.MatchString(msg) && editActionPattern.MatchString(msg) && editCodeTargetPattern.MatchString(msg) {
		return true
	}
	if strings.Contains(msg, "make it") && (hasActiveFile || hasSelection) {
		return true
	}
	return false
}

func isChatIntent(msg string) bool {
	if chatIntentPattern.MatchString(msg) {
		return true
	}
	return strings.HasSuffix(msg, "?") &&
		(strings.Contains(msg, "what ") || strings.Contains(msg, "how ") || strings.Contains(msg, "why ") || strings.Contains(msg, "who "))
}

func buildChatPrompt(root string, req routeRequest, contextText, mcpText string, mcpLoop *boundedMCPContextResult) string {
	opening := []string{
		"You are DorkPipe, a local-first repo-aware coding assistant.",
		"Ground your answer in focused workspace context when relevant.",
		"Use active-file snippets, explicit file references, and bounded MCP retrieval as primary grounding.",
		"Only factor scan findings, user guidance, or other artifact-backed signals into the answer when the user explicitly asks for them or the request is clearly about those topics.",
		"When explaining internals, architecture, or runtime behavior, anchor each substantive claim to concrete file and function names from the provided context.",
		"Do not infer behavior that is not shown in the current code context; if something is uncertain, say so explicitly.",
		"When MCP discovery data is provided, treat it as typed control-plane context and prefer it over guessing about workflows or tool availability.",
		"When a bounded MCP context loop is provided, use it as curated retrieval context rather than asking for broad extra context.",
		"If you provide code, use fenced code blocks with a language tag when possible.",
		"Be concise, practical, and explicit about uncertainty.",
	}
	switch normalizeRequestMode(req.Mode) {
	case "plan":
		opening = append(opening,
			"Mode: Plan.",
			"Do not make or imply code changes. Give a concrete implementation plan, likely files, and validation steps.")
	case "agent":
		opening = append(opening,
			"Mode: Agent.",
			"Assume the user wants action-oriented help and bias toward concrete next steps over broad explanation.")
	default:
		opening = append(opening, "Mode: Ask.")
	}
	if req.ActiveFile != "" {
		opening = append(opening, fmt.Sprintf("Active file: %s", req.ActiveFile))
	}
	if req.SelectionText != "" {
		opening = append(opening, fmt.Sprintf("Selected text:\n%s", clampString(req.SelectionText, maxEditSelectionChars)))
	}
	if len(req.OpenFiles) > 0 {
		opening = append(opening, "Open files:\n- "+strings.Join(req.OpenFiles, "\n- "))
	}
	if strings.TrimSpace(contextText) != "" {
		opening = append(opening, fmt.Sprintf("Focused workspace context:\n\n%s", contextText))
	}
	if strings.TrimSpace(mcpText) != "" {
		opening = append(opening, fmt.Sprintf("MCP discovery context:\n\n%s", mcpText))
	}
	if mcpLoop != nil && strings.TrimSpace(mcpLoop.Summary) != "" {
		opening = append(opening, fmt.Sprintf("MCP bounded context loop:\n\n%s", mcpLoop.Summary))
	}
	opening = append(opening, fmt.Sprintf("User request:\n%s", req.Message))
	return strings.Join(opening, "\n\n")
}

func buildWorkspaceChatContext(root string, req routeRequest) (string, map[string]any) {
	sections := []string{}
	meta := map[string]any{}
	searchTerms := extractSearchTerms(req.Message)
	targets := inferWorkspaceChatTargets(root, req)
	if len(targets) > 3 {
		targets = targets[:3]
	}
	if len(targets) > 0 {
		meta["context_files"] = len(targets)
	}
	for _, rel := range targets {
		snippet, err := readWorkspaceSnippet(root, rel, searchTerms)
		if err != nil || strings.TrimSpace(snippet) == "" {
			continue
		}
		sections = append(sections, fmt.Sprintf("Relevant file: %s\n\n```text\n%s\n```", rel, snippet))
	}
	if wantsScanSignals(req.Message) {
		if summary := summarizeScanSignals(root); strings.TrimSpace(summary) != "" {
			sections = append(sections, "Scan signals:\n\n"+summary)
			meta["scan_signals_used"] = true
		}
	}
	if wantsGuidanceSignals(req.Message) {
		if summary := summarizeGuidanceSignals(root); strings.TrimSpace(summary) != "" {
			sections = append(sections, "User guidance signals:\n\n"+summary)
			meta["guidance_signals_used"] = true
		}
	}
	return strings.Join(sections, "\n\n"), meta
}

func inferWorkspaceChatTargets(root string, req routeRequest) []string {
	targets := []string{}
	if strings.TrimSpace(req.ActiveFile) != "" {
		targets = append(targets, strings.TrimSpace(req.ActiveFile))
	}
	targets = append(targets, req.OpenFiles...)
	targets = append(targets, explicitRepoFileMentions(root, req.Message)...)
	if looksLikeInternalArchitectureQuestion(req.Message) {
		targets = append(targets,
			"packages/dorkpipe/lib/cmd/dorkpipe/request.go",
			"packages/pipeon/resolvers/pipeon/vscode-extension/src/extension.ts",
			"packages/pipeon/resolvers/pipeon/assets/scripts/prompts/system.md",
			"packages/pipeon/resolvers/pipeon/vscode-extension/src/webview/chat.ts",
		)
	}
	var existing []string
	for _, rel := range uniqueNonEmpty(targets) {
		if strings.TrimSpace(rel) == "" {
			continue
		}
		if _, err := os.Stat(filepath.Join(root, rel)); err == nil {
			existing = append(existing, rel)
		}
	}
	return uniqueNonEmpty(existing)
}

func looksLikeInternalArchitectureQuestion(message string) bool {
	lower := strings.ToLower(strings.TrimSpace(message))
	for _, token := range []string{
		"how does",
		"how do",
		"internal flow",
		"internally",
		"on the inside",
		"current flow",
		"ask mode",
		"edit mode",
		"extension flow",
		"request.go",
		"extension.ts",
		"architecture",
		"runtime behavior",
		"what changed",
		"what role",
	} {
		if strings.Contains(lower, token) {
			return true
		}
	}
	return false
}

func readWorkspaceSnippet(root, rel string, searchTerms []string) (string, error) {
	if strings.TrimSpace(rel) == "" {
		return "", nil
	}
	abs := filepath.Join(root, rel)
	b, err := os.ReadFile(abs)
	if err != nil {
		return "", err
	}
	text := focusSnippetText(string(b), searchTerms, 1800)
	if strings.TrimSpace(text) == "" {
		text = clampString(string(b), 1800)
	}
	return text, nil
}

func wantsScanSignals(message string) bool {
	lower := strings.ToLower(message)
	for _, token := range []string{"scan", "finding", "findings", "security", "compliance", "vuln", "vulnerability", "gosec", "govuln", "cve", "audit", "risk"} {
		if strings.Contains(lower, token) {
			return true
		}
	}
	return false
}

func wantsGuidanceSignals(message string) bool {
	lower := strings.ToLower(message)
	for _, token := range []string{"insight", "guidance", "preference", "convention", "policy", "rule", "guideline", "style"} {
		if strings.Contains(lower, token) {
			return true
		}
	}
	return false
}

func summarizeScanSignals(root string) string {
	for _, rel := range []string{
		filepath.Join("bin", ".dockpipe", "ci-analysis", "findings.json"),
		filepath.Join(".dockpipe", "ci-analysis", "findings.json"),
	} {
		abs := filepath.Join(root, rel)
		b, err := os.ReadFile(abs)
		if err != nil {
			continue
		}
		var parsed struct {
			Provenance struct {
				Commit string `json:"commit"`
				Source string `json:"source"`
			} `json:"provenance"`
			Findings []struct {
				Tool     string `json:"tool"`
				RuleID   string `json:"rule_id"`
				Title    string `json:"title"`
				File     string `json:"file"`
				Severity string `json:"severity"`
			} `json:"findings"`
		}
		if err := json.Unmarshal(b, &parsed); err != nil {
			continue
		}
		lines := []string{
			fmt.Sprintf("- file: %s", filepath.ToSlash(rel)),
			fmt.Sprintf("- findings: %d", len(parsed.Findings)),
		}
		if strings.TrimSpace(parsed.Provenance.Commit) != "" {
			lines = append(lines, fmt.Sprintf("- provenance commit: %s", parsed.Provenance.Commit))
		}
		if strings.TrimSpace(parsed.Provenance.Source) != "" {
			lines = append(lines, fmt.Sprintf("- source: %s", parsed.Provenance.Source))
		}
		for _, finding := range parsed.Findings {
			title := strings.TrimSpace(finding.Title)
			if title == "" {
				title = finding.RuleID
			}
			lines = append(lines, fmt.Sprintf("- [%s] %s %s %s", emptyFallback(finding.Severity, "?"), emptyFallback(finding.Tool, "?"), emptyFallback(finding.File, "?"), strings.TrimSpace(title)))
			if len(lines) >= 7 {
				break
			}
		}
		return strings.Join(lines, "\n")
	}
	return ""
}

func summarizeGuidanceSignals(root string) string {
	for _, rel := range []string{
		filepath.Join("bin", ".dockpipe", "analysis", "insights.json"),
		filepath.Join(".dockpipe", "analysis", "insights.json"),
	} {
		abs := filepath.Join(root, rel)
		b, err := os.ReadFile(abs)
		if err != nil {
			continue
		}
		var parsed struct {
			Insights []struct {
				Status         string `json:"status"`
				Category       string `json:"category"`
				NormalizedText string `json:"normalized_text"`
			} `json:"insights"`
		}
		if err := json.Unmarshal(b, &parsed); err != nil {
			continue
		}
		lines := []string{
			fmt.Sprintf("- file: %s", filepath.ToSlash(rel)),
			fmt.Sprintf("- insights: %d", len(parsed.Insights)),
		}
		for _, insight := range parsed.Insights {
			text := clampString(strings.TrimSpace(insight.NormalizedText), 120)
			lines = append(lines, fmt.Sprintf("- [%s] %s: %s", emptyFallback(insight.Status, "?"), emptyFallback(insight.Category, "general"), text))
			if len(lines) >= 7 {
				break
			}
		}
		return strings.Join(lines, "\n")
	}
	return ""
}

func normalizeRequestMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "agent":
		return "agent"
	case "plan":
		return "plan"
	default:
		return "ask"
	}
}

func runChatModelStream(ctx context.Context, host, model string, numCtx int, prompt string, onToken func(string)) (string, error) {
	u, err := buildOllamaChatURL(host)
	if err != nil {
		return "", err
	}
	payload := map[string]any{
		"model":  model,
		"stream": true,
		"messages": []map[string]string{
			{"role": "system", "content": "You are DorkPipe, a repo-aware coding assistant."},
			{"role": "user", "content": prompt},
		},
	}
	if numCtx > 0 {
		payload["options"] = map[string]any{"num_ctx": numCtx}
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), bytes.NewReader(b))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := (&http.Client{}).Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}
	decoder := json.NewDecoder(resp.Body)
	var full strings.Builder
	for {
		var obj map[string]any
		if err := decoder.Decode(&obj); err != nil {
			if err == io.EOF {
				break
			}
			return full.String(), err
		}
		piece := nestedString(obj, "message", "content")
		if piece == "" {
			piece = stringValue(obj["response"])
		}
		if piece != "" {
			full.WriteString(piece)
			if onToken != nil {
				onToken(piece)
			}
		}
	}
	return full.String(), nil
}

func nestedString(obj map[string]any, outer, inner string) string {
	v, ok := obj[outer]
	if !ok {
		return ""
	}
	m, ok := v.(map[string]any)
	if !ok {
		return ""
	}
	return stringValue(m[inner])
}

func stringValue(v any) string {
	s, _ := v.(string)
	return s
}

func resolveModelConfig(hostOverride, modelOverride string) (string, string) {
	host := strings.TrimSpace(hostOverride)
	if host == "" {
		host = strings.TrimSpace(os.Getenv("OLLAMA_HOST"))
	}
	if host == "" {
		host = "http://127.0.0.1:11434"
	}
	model := strings.TrimSpace(modelOverride)
	if model == "" {
		model = strings.TrimSpace(os.Getenv("PIPEON_OLLAMA_MODEL"))
	}
	if model == "" {
		model = strings.TrimSpace(os.Getenv("DOCKPIPE_OLLAMA_MODEL"))
	}
	if model == "" {
		model = defaultEditModel
	}
	return host, model
}

func resolveNumCtx(flagValue int) int {
	if flagValue > 0 {
		return flagValue
	}
	raw := strings.TrimSpace(os.Getenv("PIPEON_OLLAMA_NUM_CTX"))
	if raw == "" {
		raw = strings.TrimSpace(os.Getenv("DOCKPIPE_OLLAMA_NUM_CTX"))
	}
	if raw == "" {
		return 0
	}
	var num int
	fmt.Sscanf(raw, "%d", &num)
	if num > 0 {
		return num
	}
	return 0
}

func runRepoCommand(ctx context.Context, root, command string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, command, args...)
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "DOCKPIPE_WORKDIR="+root)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	text := strings.TrimSpace(stdout.String())
	if strings.TrimSpace(stderr.String()) != "" {
		if text != "" {
			text += "\n"
		}
		text += strings.TrimSpace(stderr.String())
	}
	return text, err
}

func codeFence(lang, text string) string {
	return fmt.Sprintf("```%s\n%s\n```", lang, text)
}

func nonEmpty(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
