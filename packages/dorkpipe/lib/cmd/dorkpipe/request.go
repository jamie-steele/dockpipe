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
	SelectionText string
}

func requestCmd(argv []string) {
	fs := flag.NewFlagSet("request", flag.ExitOnError)
	workdir := fs.String("workdir", "", "working directory (default cwd)")
	message := fs.String("message", "", "request text")
	activeFile := fs.String("active-file", "", "repo-relative active file hint")
	selectionText := fs.String("selection-text", "", "selection hint")
	executeRoute := fs.Bool("execute", false, "execute the routed request and stream events")
	model := fs.String("model", "", "Ollama model override")
	ollamaHost := fs.String("ollama-host", "", "Ollama host override")
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
		SelectionText: strings.TrimSpace(*selectionText),
	}
	route, action, reason := chooseRoute(req)
	emitEditEvent(reqID, "routed", fmt.Sprintf("Route: %s", route), 0.22, map[string]any{
		"route":  route,
		"action": action,
		"reason": reason,
	})

	if !*executeRoute {
		emitEditDone(reqID, "Request routed.", map[string]any{
			"route":   route,
			"action":  action,
			"reason":  reason,
			"workdir": absWd,
		})
		return
	}

	ctx := context.Background()
	host, chosenModel := resolveModelConfig(strings.TrimSpace(*ollamaHost), strings.TrimSpace(*model))
	switch route {
	case "inspect":
		handleInspectRoute(ctx, reqID, absWd, action)
	case "edit":
		handleEditRoute(ctx, reqID, absWd, req, host, chosenModel)
	default:
		handleChatRoute(ctx, reqID, absWd, req, host, chosenModel)
	}
}

func chooseRoute(req routeRequest) (route, action, reason string) {
	msg := strings.TrimSpace(strings.ToLower(req.Message))
	if msg == "" {
		return "chat", "", "empty fallback"
	}

	if strings.HasPrefix(msg, "/") {
		return "inspect", "slash", "explicit slash command"
	}

	if isInspectIntent(msg) {
		return "inspect", inspectAction(msg), "natural-language inspect request"
	}

	if isEditIntent(msg, req.ActiveFile != "", req.SelectionText != "") {
		return "edit", "", "edit-oriented request with code/workspace cues"
	}

	return "chat", "", "default conversational route"
}

func handleInspectRoute(ctx context.Context, reqID, root, action string) {
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
	default:
		emitEditEvent(reqID, "context_gathering", "Reading context bundle", 0.45, nil)
		rel, text := readEditContextBundle(root)
		if strings.TrimSpace(text) == "" {
			emitEditDone(reqID, "No DorkPipe context bundle found yet. Run a context refresh first.", map[string]any{
				"route":  "inspect",
				"action": "context",
			})
			return
		}
		emitEditDone(reqID, fmt.Sprintf("Context bundle: `%s`\n\n%s", rel, codeFence("markdown", text)), map[string]any{
			"route":  "inspect",
			"action": "context",
		})
	}
}

func handleEditRoute(ctx context.Context, reqID, root string, req routeRequest, host, model string) {
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
	cmd := exec.CommandContext(ctx, exe, args...)
	cmd.Dir = root
	cmd.Env = os.Environ()
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		// Child already emitted error event; only emit here if it crashed before that.
		emitEditError(reqID, "INTERNAL_ERROR", fmt.Sprintf("Edit route failed: %v", err), false)
	}
}

func handleChatRoute(ctx context.Context, reqID, root string, req routeRequest, host, model string) {
	emitEditEvent(reqID, "context_gathering", "Inspecting workspace context", 0.35, nil)
	contextPath, contextText := readEditContextBundle(root)
	prompt := buildChatPrompt(root, req, contextText)
	emitEditEvent(reqID, "routed", fmt.Sprintf("Streaming from %s", model), 0.55, map[string]any{
		"route": "chat",
		"model": model,
	})
	answer, err := runChatModelStream(ctx, host, model, prompt, func(piece string) {
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
		"validation_status": "not_applicable",
	}
	if contextPath != "" {
		status["context_path"] = contextPath
	}
	emitEditDone(reqID, nonEmpty(answer, "(No response text returned.)"), status)
}

func isInspectIntent(msg string) bool {
	return strings.Contains(msg, "context bundle") ||
		(strings.Contains(msg, "what context") || strings.Contains(msg, "show context")) ||
		(strings.Contains(msg, "status") && (strings.Contains(msg, "show") || strings.Contains(msg, "check") || msg == "status")) ||
		(strings.Contains(msg, "bundle context") || strings.Contains(msg, "refresh context"))
}

func inspectAction(msg string) string {
	switch {
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

func buildChatPrompt(root string, req routeRequest, contextText string) string {
	opening := []string{
		"You are DorkPipe, a local-first repo-aware coding assistant.",
		"Ground your answer in the provided workspace context when relevant.",
		"If you provide code, use fenced code blocks with a language tag when possible.",
		"Be concise, practical, and explicit about uncertainty.",
	}
	if req.ActiveFile != "" {
		opening = append(opening, fmt.Sprintf("Active file: %s", req.ActiveFile))
	}
	if req.SelectionText != "" {
		opening = append(opening, fmt.Sprintf("Selected text:\n%s", clampString(req.SelectionText, maxEditSelectionChars)))
	}
	if strings.TrimSpace(contextText) != "" {
		opening = append(opening, fmt.Sprintf("Repository context bundle:\n\n%s", contextText))
	}
	opening = append(opening, fmt.Sprintf("User request:\n%s", req.Message))
	return strings.Join(opening, "\n\n")
}

func runChatModelStream(ctx context.Context, host, model, prompt string, onToken func(string)) (string, error) {
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
