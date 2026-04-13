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
	"sort"
	"strings"

	"dorkpipe.orchestrator/reasoning"
)

type routeRequest struct {
	Message       string
	ActiveFile    string
	OpenFiles     []string
	SelectionText string
	Mode          string
}

type workspaceChatContext struct {
	Text     string
	Meta     map[string]any
	Targets  []string
	Snippets map[string]string
	Evidence chatEvidenceGraph
}

type chatAnswerValidation struct {
	Required bool
	Passed   bool
	Issues   []string
}

type chatEvidenceNode struct {
	ID      string
	Kind    string
	File    string
	Symbol  string
	Summary string
	Score   int
}

type chatEvidenceEdge struct {
	From string
	To   string
	Kind string
}

type chatEvidenceGraph struct {
	Nodes []chatEvidenceNode
	Edges []chatEvidenceEdge
}

type chatScoredFile struct {
	rel            string
	score          int
	pathMatches    int
	contentMatches int
	strongMatches  int
	phraseMatches  int
	basenameMatch  bool
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
	reasoningArtifactDir := ""
	if routed.Route == "chat" {
		if artifactDir, runErr := beginReasoningRun(absWd, reqID, "chat", ""); runErr == nil {
			reasoningArtifactDir = artifactDir
			defer endArtifactTrace()
		}
	}
	switch routed.Route {
	case "inspect":
		handleInspectRoute(ctx, reqID, absWd, req, routed.Action, routed.Arg)
	case "edit":
		if routed.Arg != "" {
			req.Message = routed.Arg
		}
		handleEditRoute(ctx, reqID, absWd, req, host, chosenModel, chosenNumCtx)
	default:
		handleChatRoute(ctx, reqID, absWd, req, host, chosenModel, chosenNumCtx, reasoningArtifactDir)
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
		case "context", "status", "bundle", "test", "ci", "workflow", "validate", "workflow-validate", "callstack", "heap", "symbol", "references", "callers":
			return routedRequest{Route: "inspect", Action: name, Arg: arg, Reason: "explicit slash command"}
		default:
			return routedRequest{Route: "inspect", Action: "slash", Arg: raw, Reason: "explicit slash command"}
		}
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
	case "symbol":
		emitEditEvent(reqID, "context_gathering", "Inspecting symbol definition", 0.45, map[string]any{
			"target":      arg,
			"active_file": req.ActiveFile,
		})
		out, err := runRepoScript(ctx, root, "packages/dorkpipe/resolvers/dorkpipe/assets/scripts/inspect-symbol.sh", root, arg, req.ActiveFile, req.SelectionText)
		if err != nil {
			emitEditError(reqID, "INTERNAL_ERROR", fmt.Sprintf("Symbol inspection failed: %s", out), true)
			return
		}
		emitEditDone(reqID, codeFence("text", nonEmpty(out, "No symbol details found.")), map[string]any{
			"route":             "inspect",
			"action":            "symbol",
			"target":            arg,
			"validation_status": "not_applicable",
			"active_file":       req.ActiveFile,
		})
	case "references":
		emitEditEvent(reqID, "context_gathering", "Inspecting symbol references", 0.45, map[string]any{
			"target":      arg,
			"active_file": req.ActiveFile,
		})
		out, err := runRepoScript(ctx, root, "packages/dorkpipe/resolvers/dorkpipe/assets/scripts/inspect-references.sh", root, arg, req.ActiveFile, req.SelectionText)
		if err != nil {
			emitEditError(reqID, "INTERNAL_ERROR", fmt.Sprintf("Reference inspection failed: %s", out), true)
			return
		}
		emitEditDone(reqID, codeFence("text", nonEmpty(out, "No references found.")), map[string]any{
			"route":             "inspect",
			"action":            "references",
			"target":            arg,
			"validation_status": "not_applicable",
			"active_file":       req.ActiveFile,
		})
	case "callers":
		emitEditEvent(reqID, "context_gathering", "Inspecting callable usages", 0.45, map[string]any{
			"target":      arg,
			"active_file": req.ActiveFile,
		})
		out, err := runRepoScript(ctx, root, "packages/dorkpipe/resolvers/dorkpipe/assets/scripts/inspect-callers.sh", root, arg, req.ActiveFile, req.SelectionText)
		if err != nil {
			emitEditError(reqID, "INTERNAL_ERROR", fmt.Sprintf("Caller inspection failed: %s", out), true)
			return
		}
		emitEditDone(reqID, codeFence("text", nonEmpty(out, "No callers found.")), map[string]any{
			"route":             "inspect",
			"action":            "callers",
			"target":            arg,
			"validation_status": "not_applicable",
			"active_file":       req.ActiveFile,
		})
	default:
		emitEditEvent(reqID, "context_gathering", "Collecting focused workspace context", 0.45, nil)
		chatContext := buildWorkspaceChatContext(root, req)
		if strings.TrimSpace(chatContext.Text) == "" {
			emitEditDone(reqID, "No focused workspace context was collected for this request.", map[string]any{
				"route":  "inspect",
				"action": "context",
			})
			return
		}
		chatContext.Meta["route"] = "inspect"
		chatContext.Meta["action"] = "context"
		emitEditDone(reqID, fmt.Sprintf("Focused workspace context:\n\n%s", codeFence("markdown", chatContext.Text)), chatContext.Meta)
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

func handleChatRoute(ctx context.Context, reqID, root string, req routeRequest, host, model string, numCtx int, reasoningArtifactDir string) {
	emitEditEvent(reqID, "context_gathering", "Inspecting workspace context", 0.35, nil)
	chatContext := buildWorkspaceChatContext(root, req)
	policy := resolveRuntimePolicy("chat", req.Message, req.ActiveFile, req.SelectionText, len(chatContext.Targets), len(req.OpenFiles))
	if len(chatContext.Evidence.Nodes) > 0 {
		emitEditEvent(reqID, "decomposing", "Built evidence graph from retrieved context", 0.4, map[string]any{
			"evidence_nodes": len(chatContext.Evidence.Nodes),
			"evidence_edges": len(chatContext.Evidence.Edges),
			"evidence_files": countEvidenceNodesByKind(chatContext.Evidence, "file"),
			"evidence_symbols": countEvidenceNodesByKind(chatContext.Evidence, "symbol"),
		})
	}
	var mcpText string
	var mcpLoop *boundedMCPContextResult
	strictValidation := shouldStrictlyValidateChatAnswer(req)
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
	prompt := buildChatPrompt(root, req, chatContext, mcpText, mcpLoop)
	emitEditEvent(reqID, "routed", fmt.Sprintf("Streaming from %s", model), 0.55, map[string]any{
		"route":            "chat",
		"model":            model,
		"num_ctx":          numCtx,
		"branching_active": policy.BranchingActive && strictValidation,
	})
	attempts := []reasoning.AttemptRecord{}
	repairMemory := []string{}
	decision := reasoning.DecisionRecord{}
	answer := ""
	validationStatus := "not_applicable"
	validation := chatAnswerValidation{}
	runAttempt := func(attemptID, label, promptText string) (chatAttemptResult, error) {
		var buffered strings.Builder
		answerText, err := runChatModelStream(ctx, host, model, numCtx, promptText, func(piece string) {
			if strictValidation || policy.BranchingActive {
				buffered.WriteString(piece)
				return
			}
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
			return chatAttemptResult{
				Attempt: reasoning.AttemptRecord{
					ID:             attemptID,
					Label:          label,
					Kind:           "candidate",
					Status:         "model_failed",
					FailureSummary: err.Error(),
				},
			}, err
		}
		if strictValidation || policy.BranchingActive {
			answerText = buffered.String()
		}
		validationStatus := "not_applicable"
		attemptValidation := chatAnswerValidation{}
		if strictValidation {
			emitEditEvent(reqID, "validating", "Validating code-anchored answer", 0.72, map[string]any{
				"context_files": len(chatContext.Targets),
				"attempt_id":    attemptID,
			})
			attemptValidation = validateChatAnswer(answerText, req, chatContext)
			if attemptValidation.Passed {
				validationStatus = "passed"
			} else {
				emitEditEvent(reqID, "validating", "Repairing unsupported answer", 0.76, map[string]any{
					"issues":     attemptValidation.Issues,
					"attempt_id": attemptID,
				})
				repairPrompt := buildChatAnswerRepairPrompt(req, answerText, chatContext, mcpText, mcpLoop, attemptValidation)
				if policy.RepairMemory && len(repairMemory) > 0 {
					repairPrompt += "\n\nPrior repair memory:\n- " + strings.Join(repairMemory, "\n- ")
				}
				repaired, repairErr := runChatModelStream(ctx, host, model, numCtx, repairPrompt, nil)
				if repairErr == nil {
					repairedValidation := validateChatAnswer(repaired, req, chatContext)
					if repairedValidation.Passed {
						answerText = repaired
						attemptValidation = repairedValidation
						validationStatus = "repaired"
					} else {
						answerText = buildEvidenceOnlyChatFallback(chatContext, repairedValidation)
						attemptValidation = repairedValidation
						validationStatus = "fallback_evidence_only"
					}
				} else {
					answerText = buildEvidenceOnlyChatFallback(chatContext, attemptValidation)
					validationStatus = "fallback_evidence_only"
				}
			}
		}
		failureSummary := summarizeChatAttemptFailure(attemptValidation)
		attempt := reasoning.AttemptRecord{
			ID:               attemptID,
			Label:            label,
			Kind:             "candidate",
			Status:           validationStatus,
			Score:            scoreChatAttempt(answerText, attemptValidation, chatContext, req),
			Summary:          clampString(answerText, 400),
			ValidationStatus: validationStatus,
			FailureSummary:   failureSummary,
		}
		if failureSummary != "" && policy.RepairMemory {
			repairMemory = append(repairMemory, failureSummary)
		}
		return chatAttemptResult{
			Attempt:        attempt,
			Answer:         answerText,
			Validation:     attemptValidation,
			FailureSummary: failureSummary,
		}, nil
	}
	if policy.BranchingActive && strictValidation {
		branchAttempts := buildChatBranchPrompts(policy)
		emitEditEvent(reqID, "branching", "Evaluating competing reasoning candidates", 0.58, map[string]any{
			"best_of_n":     policy.BestOfN,
			"max_branches":  policy.MaxBranches,
			"ambiguity":     policy.AmbiguityReason,
			"branch_count":  len(branchAttempts),
			"repair_memory": policy.RepairMemory,
		})
		var best chatAttemptResult
		best.Attempt.Score = -1
		decision.BranchesConsidered = len(branchAttempts)
		for idx, branch := range branchAttempts {
			promptText := prompt
			if hint, _ := branch.Metadata["branch_hint"].(string); strings.TrimSpace(hint) != "" {
				promptText += "\n\nBranch strategy: " + hint
			}
			if policy.RepairMemory && len(repairMemory) > 0 {
				promptText += "\n\nPrior attempt failures:\n- " + strings.Join(repairMemory, "\n- ")
			}
			result, attemptErr := runAttempt(branch.ID, branch.Label, promptText)
			if attemptErr != nil {
				result.Attempt.Score = 0
				result.Attempt.Pruned = true
				result.Attempt.PrunedReason = "model_failed"
			}
			attempts = append(attempts, result.Attempt)
			if result.Attempt.Score > best.Attempt.Score {
				best = result
			}
			if idx < len(branchAttempts)-1 {
				emitEditEvent(reqID, "branching", "Recorded branch candidate", 0.6, map[string]any{
					"attempt_id": branch.ID,
					"score":      result.Attempt.Score,
					"status":     result.Attempt.ValidationStatus,
				})
			}
		}
		for idx := range attempts {
			if attempts[idx].ID == best.Attempt.ID {
				attempts[idx].Selected = true
				attempts[idx].Status = nonEmpty(attempts[idx].Status, "selected")
			} else {
				attempts[idx].Pruned = true
				attempts[idx].PrunedReason = "lower_scoring_branch"
			}
		}
		decision.SelectedAttemptID = best.Attempt.ID
		decision.BranchesPruned = maxInt(len(attempts)-1, 0)
		answer = best.Answer
		validation = best.Validation
		validationStatus = best.Attempt.ValidationStatus
		if best.Attempt.Score < policy.AbstainThreshold {
			decision.Abstained = true
			decision.Escalated = true
			decision.EscalationReason = "low_confidence_after_branching"
			answer = buildEvidenceOnlyChatFallback(chatContext, best.Validation)
			validationStatus = "abstained_low_confidence"
		}
	} else {
		result, err := runAttempt("attempt-1", "default", prompt)
		if err != nil {
			emitEditError(reqID, "MODEL_UNAVAILABLE", fmt.Sprintf("Chat model failed: %v", err), true)
			return
		}
		result.Attempt.Selected = true
		attempts = append(attempts, result.Attempt)
		decision.SelectedAttemptID = result.Attempt.ID
		decision.BranchesConsidered = 1
		answer = result.Answer
		validation = result.Validation
		validationStatus = result.Attempt.ValidationStatus
	}
	status := map[string]any{
		"route":             "chat",
		"model":             model,
		"num_ctx":           numCtx,
		"mode":              normalizeRequestMode(req.Mode),
		"validation_status": validationStatus,
		"active_file":       req.ActiveFile,
		"best_of_n":         policy.BestOfN,
		"max_branches":      policy.MaxBranches,
		"abstain_threshold": policy.AbstainThreshold,
		"repair_memory":     policy.RepairMemory,
		"high_ambiguity":    policy.HighAmbiguity,
		"ambiguity_reason":  policy.AmbiguityReason,
	}
	for key, value := range chatContext.Meta {
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
	if len(chatContext.Evidence.Nodes) > 0 {
		status["evidence_nodes"] = len(chatContext.Evidence.Nodes)
		status["evidence_edges"] = len(chatContext.Evidence.Edges)
		status["evidence_files"] = countEvidenceNodesByKind(chatContext.Evidence, "file")
		status["evidence_symbols"] = countEvidenceNodesByKind(chatContext.Evidence, "symbol")
	}
	if strings.TrimSpace(reasoningArtifactDir) != "" {
		writeReasoningRunArtifact(reasoningArtifactDir, buildChatRunArtifact(reqID, "chat", nonEmpty(answer, "(No response text returned.)"), req, chatContext, validationStatus, validation, policy, attempts, decision, repairMemory))
		status["artifact_dir"] = relativeTo(root, reasoningArtifactDir)
		status["trace_path"] = relativeTo(root, filepath.Join(reasoningArtifactDir, "trace.jsonl"))
		status["reasoning_path"] = relativeTo(root, filepath.Join(reasoningArtifactDir, "reasoning.json"))
	}
	emitEditDone(reqID, nonEmpty(answer, "(No response text returned.)"), status)
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

func buildChatPrompt(root string, req routeRequest, chatContext workspaceChatContext, mcpText string, mcpLoop *boundedMCPContextResult) string {
	opening := []string{
		"You are DorkPipe, a local-first repo-aware coding assistant.",
		"Ground your answer in focused workspace context when relevant.",
		"Use the evidence DAG, active-file snippets, explicit file references, and bounded MCP retrieval as primary grounding.",
		"Only factor scan findings, user guidance, or other artifact-backed signals into the answer when the user explicitly asks for them or the request is clearly about those topics.",
		"When explaining internals, architecture, or runtime behavior, anchor each substantive claim to concrete file and function names from the provided context.",
		"Do not infer behavior that is not shown in the current code context; if something is uncertain, say so explicitly.",
		"If the available context is insufficient, say what is missing and limit the answer to what the retrieved files actually support.",
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
	if shouldStrictlyValidateChatAnswer(req) {
		opening = append(opening,
			"For every substantive claim, cite exact evidence using this format: Evidence: `<repo/path>` :: `<symbol>`.",
			"If the retrieved evidence is insufficient, put the point under `## Uncertain` instead of guessing.",
		)
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
	if evidenceText := formatChatEvidenceGraph(chatContext.Evidence); strings.TrimSpace(evidenceText) != "" {
		opening = append(opening, fmt.Sprintf("Evidence DAG:\n\n%s", evidenceText))
	}
	if strings.TrimSpace(chatContext.Text) != "" {
		opening = append(opening, fmt.Sprintf("Focused workspace context:\n\n%s", chatContext.Text))
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

func buildWorkspaceChatContext(root string, req routeRequest) workspaceChatContext {
	sections := []string{}
	meta := map[string]any{}
	snippets := map[string]string{}
	searchTerms := extractChatSearchTerms(req.Message)
	targets := inferWorkspaceChatTargets(root, req)
	if len(targets) > 3 {
		targets = targets[:3]
	}
	architectureQuery := isArchitectureChatQuery(req.Message)
	evidenceTerms := searchTerms
	if architectureQuery {
		evidenceTerms = extractStrongArchitectureTerms(searchTerms)
	}
	var keptTargets []string
	for _, rel := range targets {
		snippet, err := readWorkspaceSnippet(root, rel, searchTerms)
		if err != nil || strings.TrimSpace(snippet) == "" {
			continue
		}
		if architectureQuery && !shouldKeepArchitectureSnippet(rel, snippet, evidenceTerms) {
			continue
		}
		snippets[rel] = snippet
		keptTargets = append(keptTargets, rel)
		sections = append(sections, fmt.Sprintf("Relevant file: %s\n\n```text\n%s\n```", rel, snippet))
	}
	targets = uniqueNonEmpty(keptTargets)
	if len(targets) > 0 {
		meta["context_files"] = len(targets)
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
	evidence := buildChatEvidenceGraph(root, req, targets, snippets, evidenceTerms)
	if len(evidence.Nodes) > 0 {
		meta["evidence_nodes"] = len(evidence.Nodes)
		meta["evidence_edges"] = len(evidence.Edges)
	}
	return workspaceChatContext{
		Text:     strings.Join(sections, "\n\n"),
		Meta:     meta,
		Targets:  targets,
		Snippets: snippets,
		Evidence: evidence,
	}
}

func inferWorkspaceChatTargets(root string, req routeRequest) []string {
	targets := []string{}
	if strings.TrimSpace(req.ActiveFile) != "" {
		targets = append(targets, strings.TrimSpace(req.ActiveFile))
	}
	targets = append(targets, explicitRepoFileMentions(root, req.Message)...)
	targets = append(targets, req.OpenFiles...)
	if len(targets) < 3 {
		targets = append(targets, inferMentionedBasenameTargets(root, req.Message)...)
	}
	if len(targets) < 3 {
		targets = append(targets, searchWorkspaceFilesForChatQuery(root, req.Message, 6)...)
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
	if isArchitectureChatQuery(req.Message) {
		existing = prioritizeImplementationTargets(existing)
		existing = pruneNonImplementationTargets(existing)
	}
	return uniqueNonEmpty(existing)
}

func prioritizeImplementationTargets(items []string) []string {
	scored := uniqueNonEmpty(items)
	sort.SliceStable(scored, func(i, j int) bool {
		si := implementationTargetScore(scored[i])
		sj := implementationTargetScore(scored[j])
		if si == sj {
			return false
		}
		return si > sj
	})
	return scored
}

func implementationTargetScore(rel string) int {
	lower := strings.ToLower(filepath.ToSlash(rel))
	score := 0
	if isTestLikePath(lower) {
		score -= 12
	}
	if isDocLikePath(lower) {
		score -= 6
	}
	if isScriptLikePath(lower) {
		score -= 5
	}
	for _, token := range []string{"/src/", "/lib/", "/cmd/", "/internal/", "/pkg/", "/app/", "/apps/"} {
		if strings.Contains(lower, token) {
			score += 8
		}
	}
	switch strings.ToLower(filepath.Ext(lower)) {
	case ".go", ".ts", ".tsx", ".js", ".jsx", ".py", ".rs", ".java", ".c", ".cc", ".cpp", ".h", ".hpp", ".cs", ".rb", ".php", ".swift", ".kt", ".kts":
		score += 6
	case ".sh":
		score += 1
	case ".md", ".txt":
		score -= 2
	}
	return score
}

func pruneNonImplementationTargets(items []string) []string {
	all := uniqueNonEmpty(items)
	hasImplementation := false
	for _, rel := range all {
		if isImplementationLikePath(rel) {
			hasImplementation = true
			break
		}
	}
	if !hasImplementation {
		return all
	}
	var out []string
	for _, rel := range all {
		if isImplementationLikePath(rel) {
			out = append(out, rel)
		}
	}
	if len(out) == 0 {
		return all
	}
	return pruneClientSurfaceTargets(out)
}

func inferMentionedBasenameTargets(root, message string) []string {
	basenames := extractMentionedBasenames(message)
	if len(basenames) == 0 {
		return nil
	}
	seenBase := map[string]struct{}{}
	for _, base := range basenames {
		seenBase[strings.ToLower(base)] = struct{}{}
	}
	var out []string
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if shouldSkipChatSearchDir(d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		base := strings.ToLower(filepath.Base(path))
		if _, ok := seenBase[base]; !ok {
			return nil
		}
		rel := filepath.ToSlash(relativeTo(root, path))
		if shouldSkipChatSearchPath(rel) {
			return nil
		}
		out = append(out, rel)
		if len(out) >= 6 {
			return filepath.SkipAll
		}
		return nil
	})
	return uniqueNonEmpty(out)
}

func extractMentionedBasenames(message string) []string {
	var out []string
	for _, token := range strings.Fields(message) {
		token = strings.TrimSpace(strings.Trim(token, ".,:;!?()[]{}<>\"'`"))
		token = filepath.ToSlash(strings.TrimPrefix(token, "./"))
		if token == "" || strings.Contains(token, "/") || !strings.Contains(token, ".") {
			continue
		}
		base := filepath.Base(token)
		if strings.HasPrefix(base, ".") || strings.Count(base, ".") == 0 {
			continue
		}
		out = append(out, base)
	}
	return uniqueNonEmpty(out)
}

func searchWorkspaceFilesForChatQuery(root, message string, maxResults int) []string {
	if maxResults <= 0 {
		return nil
	}
	terms := extractChatSearchTerms(message)
	phrases := extractChatSearchPhrases(message)
	basenames := extractMentionedBasenames(message)
	architectureQuery := isArchitectureChatQuery(message)
	if len(terms) == 0 && len(phrases) == 0 && len(basenames) == 0 {
		return nil
	}
	var scored []chatScoredFile
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if shouldSkipChatSearchDir(d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		rel := filepath.ToSlash(relativeTo(root, path))
		if rel == "." || rel == "" || shouldSkipChatSearchPath(rel) || !shouldConsiderChatSearchFile(rel) {
			return nil
		}
		info, infoErr := d.Info()
		if infoErr == nil && info.Size() > 256*1024 {
			return nil
		}
		lowerRel := strings.ToLower(rel)
		score := 0
		basenameMatch := false
		for _, base := range basenames {
			lowerBase := strings.ToLower(base)
			if filepath.Base(lowerRel) == lowerBase {
				score += 8
				basenameMatch = true
			}
		}
		matchedPathTerms := 0
		strongMatches := 0
		for _, term := range terms {
			if strings.Contains(lowerRel, term) {
				matchedPathTerms++
				if !isWeakArchitectureTerm(term) {
					strongMatches++
				}
				score += chatSearchTermMatchWeight(term, architectureQuery)
			}
		}
		score += scoreChatSearchPath(lowerRel, architectureQuery)

		b, readErr := os.ReadFile(path)
		if readErr == nil {
			text := strings.ToLower(clampString(string(b), 12000))
			matchedContentTerms := 0
			matchedPhrases := 0
			for _, phrase := range phrases {
				if strings.Contains(text, phrase) {
					score += 4
					matchedPhrases++
				}
			}
			for _, term := range terms {
				if strings.Contains(text, term) {
					matchedContentTerms++
					if !isWeakArchitectureTerm(term) {
						strongMatches++
					}
					score += chatSearchTermMatchWeight(term, architectureQuery)
				}
			}
			scored = append(scored, chatScoredFile{
				rel:            rel,
				score:          score,
				pathMatches:    matchedPathTerms,
				contentMatches: matchedContentTerms,
				strongMatches:  strongMatches,
				phraseMatches:  matchedPhrases,
				basenameMatch:  basenameMatch,
			})
			return nil
		}
		if score > 0 {
			scored = append(scored, chatScoredFile{rel: rel, score: score, pathMatches: matchedPathTerms, strongMatches: strongMatches, basenameMatch: basenameMatch})
		}
		return nil
	})
	if architectureQuery {
		scored = filterArchitectureScoredFiles(scored)
	}
	sort.SliceStable(scored, func(i, j int) bool {
		if scored[i].score == scored[j].score {
			if len(scored[i].rel) == len(scored[j].rel) {
				return scored[i].rel < scored[j].rel
			}
			return len(scored[i].rel) < len(scored[j].rel)
		}
		return scored[i].score > scored[j].score
	})
	var out []string
	for _, item := range scored {
		out = append(out, item.rel)
		if len(out) >= maxResults {
			break
		}
	}
	return uniqueNonEmpty(out)
}

func extractChatSearchTerms(message string) []string {
	raw := extractSearchTerms(message)
	var out []string
	for _, term := range raw {
		for _, candidate := range normalizeChatSearchTermVariants(term) {
			if isLowSignalChatTerm(candidate) {
				continue
			}
			out = append(out, candidate)
		}
	}
	return uniqueNonEmpty(out)
}

func extractChatSearchPhrases(message string) []string {
	raw := strings.Fields(strings.ToLower(message))
	var words []string
	for _, token := range raw {
		token = strings.Trim(token, ".,:;!?()[]{}<>\"'`")
		if len(token) < 3 {
			continue
		}
		words = append(words, token)
	}
	var phrases []string
	for i := 0; i < len(words)-1; i++ {
		phrase := strings.TrimSpace(words[i] + " " + words[i+1])
		if len(phrase) < 8 {
			continue
		}
		phrases = append(phrases, phrase)
	}
	if len(phrases) > 6 {
		phrases = phrases[:6]
	}
	return uniqueNonEmpty(phrases)
}

func isDenseArchitectureMatch(basenameMatch bool, pathMatches, contentMatches, strongMatches, phraseMatches int) bool {
	if basenameMatch {
		return true
	}
	if phraseMatches > 0 {
		return true
	}
	return strongMatches >= 1 && pathMatches+contentMatches >= 2
}

func isLowSignalChatTerm(term string) bool {
	switch strings.ToLower(strings.TrimSpace(term)) {
	case "current", "currently", "latest", "changes", "change", "question", "plain", "english", "architecture", "internally", "internal", "runtime", "behavior", "works", "working", "ground", "grounds", "grounding", "repo", "repository", "code", "anchored", "claim", "claims", "exact", "cite", "cited", "confidence", "guess", "symbol", "symbols", "path", "paths":
		return true
	default:
		return false
	}
}

func chatSearchTermMatchWeight(term string, architectureQuery bool) int {
	if architectureQuery && isWeakArchitectureTerm(term) {
		return 1
	}
	return 2
}

func isWeakArchitectureTerm(term string) bool {
	switch strings.ToLower(strings.TrimSpace(term)) {
	case "mode", "validation", "validations":
		return true
	default:
		return false
	}
}

func isGenericArchitectureTerm(term string) bool {
	switch strings.ToLower(strings.TrimSpace(term)) {
	case "explain", "question", "plain", "english", "latest", "changes", "change", "works", "working", "current", "currently":
		return true
	default:
		return false
	}
}

func normalizeChatSearchTermVariants(term string) []string {
	term = strings.ToLower(strings.TrimSpace(term))
	if term == "" {
		return nil
	}
	out := []string{term}
	if strings.HasSuffix(term, "ies") && len(term) > 4 {
		out = append(out, strings.TrimSuffix(term, "ies")+"y")
	}
	if strings.HasSuffix(term, "es") && len(term) > 4 {
		out = append(out, strings.TrimSuffix(term, "es"))
	}
	if strings.HasSuffix(term, "s") && len(term) > 4 {
		out = append(out, strings.TrimSuffix(term, "s"))
	}
	if strings.HasSuffix(term, "ing") && len(term) > 6 {
		out = append(out, strings.TrimSuffix(term, "ing"))
	}
	if strings.HasSuffix(term, "ed") && len(term) > 5 {
		out = append(out, strings.TrimSuffix(term, "ed"))
	}
	return uniqueNonEmpty(out)
}

func extractStrongArchitectureTerms(terms []string) []string {
	var out []string
	for _, term := range uniqueNonEmpty(terms) {
		if isWeakArchitectureTerm(term) || isGenericArchitectureTerm(term) {
			continue
		}
		out = append(out, term)
	}
	if len(out) == 0 {
		return uniqueNonEmpty(terms)
	}
	return uniqueNonEmpty(out)
}

func filterArchitectureScoredFiles(items []chatScoredFile) []chatScoredFile {
	if len(items) == 0 {
		return nil
	}
	var dense []chatScoredFile
	for _, item := range items {
		if isDenseArchitectureMatch(item.basenameMatch, item.pathMatches, item.contentMatches, item.strongMatches, item.phraseMatches) {
			dense = append(dense, item)
		}
	}
	if len(dense) > 0 {
		return dense
	}
	var implementation []chatScoredFile
	for _, item := range items {
		if isImplementationLikePath(item.rel) {
			implementation = append(implementation, item)
		}
	}
	if len(implementation) > 0 {
		return implementation
	}
	return items
}

func shouldSkipChatSearchDir(name string) bool {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case ".git", ".hg", ".svn", "node_modules", ".dockpipe", ".dorkpipe", "dist", "build", "coverage", ".next", ".turbo", "vendor", "target":
		return true
	default:
		return false
	}
}

func shouldSkipChatSearchPath(rel string) bool {
	lower := strings.ToLower(filepath.ToSlash(rel))
	if lower == "." || lower == "" {
		return true
	}
	segments := strings.Split(lower, "/")
	for _, segment := range segments {
		if shouldSkipChatSearchDir(segment) {
			return true
		}
	}
	for _, token := range []string{
		"/.dockpipe/internal/",
		"/.dorkpipe/",
		"/cache/",
		"/tmp/",
		"/temp/",
		"/artifacts/",
		"/generated/",
	} {
		if strings.Contains(lower, token) {
			return true
		}
	}
	return false
}

func shouldConsiderChatSearchFile(rel string) bool {
	base := strings.ToLower(filepath.Base(rel))
	switch base {
	case "readme", "readme.md", "makefile", "dockerfile", "compose.yml", "compose.yaml":
		return true
	}
	switch strings.ToLower(filepath.Ext(rel)) {
	case ".go", ".ts", ".tsx", ".js", ".jsx", ".mjs", ".cjs", ".json", ".md", ".txt", ".yaml", ".yml", ".toml", ".sh", ".bash", ".zsh", ".py", ".rs", ".java", ".c", ".cc", ".cpp", ".h", ".hpp", ".cs", ".rb", ".php", ".swift", ".kt", ".kts", ".sql":
		return true
	default:
		return false
	}
}

func isArchitectureChatQuery(message string) bool {
	lower := strings.ToLower(strings.TrimSpace(message))
	for _, token := range []string{
		"explain",
		"summarize",
		"walk me through",
		"how does",
		"how do",
		"what is",
		"what are",
		"internally",
		"internal",
		"architecture",
		"flow",
		"pipeline",
		"runtime behavior",
		"current behavior",
		"currently work",
		"how it works",
		"how this works",
		"what changed",
		"what role",
		"route",
	} {
		if strings.Contains(lower, token) {
			return true
		}
	}
	return false
}

func scoreChatSearchPath(lowerRel string, architectureQuery bool) int {
	score := 0
	ext := strings.ToLower(filepath.Ext(lowerRel))
	if architectureQuery {
		switch ext {
		case ".go", ".ts", ".tsx", ".js", ".jsx", ".py", ".rs", ".java", ".c", ".cc", ".cpp", ".h", ".hpp", ".cs", ".rb", ".php", ".swift", ".kt", ".kts", ".sh":
			score += 6
		case ".json", ".yaml", ".yml", ".toml", ".sql":
			score += 2
		case ".md", ".txt":
			score -= 2
		}
		for _, token := range []string{"/src/", "/lib/", "/cmd/", "/internal/", "/pkg/", "/app/", "/apps/"} {
			if strings.Contains(lowerRel, token) {
				score += 4
			}
		}
		for _, token := range []string{"readme", "/docs/", "/doc/"} {
			if strings.Contains(lowerRel, token) {
				score -= 3
			}
		}
		if isClientSurfaceLikePath(lowerRel) {
			score -= 6
		}
		if isTestLikePath(lowerRel) {
			score -= 12
		}
		if isScriptLikePath(lowerRel) {
			score -= 5
		}
	}
	if strings.HasSuffix(lowerRel, "readme.md") {
		score -= 1
	}
	return score
}

func shouldStrictlyValidateChatAnswer(req routeRequest) bool {
	if normalizeRequestMode(req.Mode) != "ask" {
		return false
	}
	lower := strings.ToLower(strings.TrimSpace(req.Message))
	for _, token := range []string{
		"explain",
		"summarize",
		"walk me through",
		"how does",
		"how do",
		"what is",
		"what are",
		"internally",
		"internal",
		"architecture",
		"flow",
		"pipeline",
		"runtime behavior",
		"current behavior",
		"currently work",
		"how it works",
		"how this works",
		"what changed",
		"what role",
	} {
		if strings.Contains(lower, token) {
			return true
		}
	}
	return false
}

func validateChatAnswer(answer string, req routeRequest, chatContext workspaceChatContext) chatAnswerValidation {
	result := chatAnswerValidation{
		Required: shouldStrictlyValidateChatAnswer(req),
		Passed:   true,
	}
	if !result.Required {
		return result
	}
	citationCount := countSupportedEvidenceCitations(answer, chatContext.Evidence)
	requiredCitations := requiredEvidenceCitationCount(req, chatContext)
	if citationCount < requiredCitations {
		result.Passed = false
		if requiredCitations > 1 {
			result.Issues = append(result.Issues, fmt.Sprintf("insufficient evidence citations to retrieved file/symbol nodes: got %d, need at least %d", citationCount, requiredCitations))
		} else {
			result.Issues = append(result.Issues, "missing evidence citations to retrieved file/symbol nodes")
		}
	}
	if unsupported := findUnsupportedAnswerReferences(answer, req, chatContext); len(unsupported) > 0 {
		result.Passed = false
		result.Issues = append(result.Issues, "unsupported references: "+strings.Join(unsupported, ", "))
	}
	if weakBindings := findWeakEvidenceBindings(answer, req, chatContext); len(weakBindings) > 0 {
		result.Passed = false
		result.Issues = append(result.Issues, "weak evidence bindings: "+strings.Join(weakBindings, "; "))
	}
	if isArchitectureChatQuery(req.Message) && looksLikeMetaPolicyAnswer(answer, req, chatContext) {
		result.Passed = false
		result.Issues = append(result.Issues, "answer restates response policy more than code behavior")
	}
	return result
}

func requiredEvidenceCitationCount(req routeRequest, chatContext workspaceChatContext) int {
	if !isArchitectureChatQuery(req.Message) {
		return 1
	}
	symbolCount := 0
	for _, node := range chatContext.Evidence.Nodes {
		if node.Kind == "symbol" {
			symbolCount++
		}
	}
	if symbolCount >= 2 {
		return 2
	}
	return 1
}

var evidenceCitationPattern = regexp.MustCompile("Evidence:\\s*`([^`]+)`\\s*::\\s*`([^`]+)`")

func countSupportedEvidenceCitations(answer string, evidence chatEvidenceGraph) int {
	fileSymbols := map[string]map[string]struct{}{}
	for _, node := range evidence.Nodes {
		if node.Kind != "symbol" || node.File == "" || node.Symbol == "" {
			continue
		}
		file := strings.ToLower(node.File)
		if _, ok := fileSymbols[file]; !ok {
			fileSymbols[file] = map[string]struct{}{}
		}
		fileSymbols[file][strings.ToLower(node.Symbol)] = struct{}{}
	}
	count := 0
	for _, match := range evidenceCitationPattern.FindAllStringSubmatch(answer, -1) {
		if len(match) < 3 {
			continue
		}
		file := strings.ToLower(strings.TrimSpace(match[1]))
		symbol := strings.ToLower(strings.TrimSpace(match[2]))
		if symbols, ok := fileSymbols[file]; ok {
			if _, ok := symbols[symbol]; ok {
				count++
			}
		}
	}
	return count
}

type answerCitationBinding struct {
	claim  string
	file   string
	symbol string
}

func findWeakEvidenceBindings(answer string, req routeRequest, chatContext workspaceChatContext) []string {
	bindings := parseAnswerCitationBindings(answer)
	if len(bindings) == 0 {
		return nil
	}
	knownSymbols := knownEvidenceSymbols(chatContext.Evidence)
	var issues []string
	for _, binding := range bindings {
		if ok, reason := citationSupportsClaim(binding, req, chatContext, knownSymbols); !ok {
			issues = append(issues, fmt.Sprintf("%s :: %s (%s)", binding.file, binding.symbol, reason))
		}
	}
	return uniqueNonEmpty(issues)
}

func parseAnswerCitationBindings(answer string) []answerCitationBinding {
	var out []answerCitationBinding
	for _, line := range strings.Split(answer, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.Contains(trimmed, "Evidence:") {
			continue
		}
		claim := strings.TrimSpace(trimmed)
		if idx := strings.Index(claim, "Evidence:"); idx >= 0 {
			claim = strings.TrimSpace(claim[:idx])
		}
		claim = strings.TrimSpace(strings.TrimLeft(claim, "-*0123456789. "))
		for _, match := range evidenceCitationPattern.FindAllStringSubmatch(trimmed, -1) {
			if len(match) < 3 {
				continue
			}
			out = append(out, answerCitationBinding{
				claim:  claim,
				file:   strings.TrimSpace(match[1]),
				symbol: strings.TrimSpace(match[2]),
			})
		}
	}
	return out
}

func citationSupportsClaim(binding answerCitationBinding, req routeRequest, chatContext workspaceChatContext, knownSymbols []string) (bool, string) {
	claim := strings.ToLower(strings.TrimSpace(binding.claim))
	if claim == "" {
		return false, "empty_claim"
	}
	symbol := strings.TrimSpace(binding.symbol)
	lowerSymbol := strings.ToLower(symbol)
	if lowerSymbol == "" {
		return false, "empty_symbol"
	}
	if strings.Contains(claim, lowerSymbol) {
		return true, ""
	}
	explicitSymbols := explicitMentionedEvidenceSymbols(claim, knownSymbols)
	if len(explicitSymbols) > 0 {
		for _, item := range explicitSymbols {
			if item == lowerSymbol {
				return true, ""
			}
		}
		return false, "claim_mentions_different_symbol"
	}
	codeIdentifiers := extractCodeLikeIdentifiers(binding.claim)
	for _, identifier := range codeIdentifiers {
		if strings.EqualFold(identifier, symbol) {
			return true, ""
		}
	}
	symbolTokens := significantIdentifierTokens(symbol)
	claimTokens := significantClaimTokens(binding.claim)
	overlap := intersectTokenCount(claimTokens, symbolTokens)
	fileTokens := significantPathTokens(binding.file)
	fileOverlap := intersectTokenCount(claimTokens, fileTokens)
	if overlap >= 1 && overlap+fileOverlap >= 2 {
		return true, ""
	}
	if !isArchitectureChatQuery(req.Message) && overlap >= 1 {
		return true, ""
	}
	return false, "claim_not_supported_by_cited_symbol"
}

func knownEvidenceSymbols(graph chatEvidenceGraph) []string {
	var out []string
	for _, node := range graph.Nodes {
		if node.Kind == "symbol" && node.Symbol != "" {
			out = append(out, strings.ToLower(node.Symbol))
		}
	}
	return uniqueNonEmpty(out)
}

func explicitMentionedEvidenceSymbols(claim string, knownSymbols []string) []string {
	lower := strings.ToLower(claim)
	var out []string
	for _, symbol := range knownSymbols {
		if symbol != "" && strings.Contains(lower, symbol) {
			out = append(out, symbol)
		}
	}
	return uniqueNonEmpty(out)
}

func extractCodeLikeIdentifiers(text string) []string {
	pattern := regexp.MustCompile(`\b[A-Za-z_][A-Za-z0-9_]{4,}\b`)
	var out []string
	for _, match := range pattern.FindAllString(text, -1) {
		if looksLikeNaturalWord(match) {
			continue
		}
		out = append(out, match)
	}
	return uniqueNonEmpty(out)
}

func looksLikeNaturalWord(token string) bool {
	lower := strings.ToLower(strings.TrimSpace(token))
	if lower == "" {
		return true
	}
	if lower == token && !strings.ContainsAny(token, "_") {
		switch lower {
		case "after", "plain", "english", "architecture", "question", "changes", "current", "behavior", "confidence", "confirmed", "uncertain", "evidence", "citations", "substantive", "claim", "claims", "anchored", "exact", "specific", "handles", "handling":
			return true
		}
	}
	return false
}

func significantIdentifierTokens(value string) []string {
	replacer := strings.NewReplacer("-", " ", "_", " ", ".", " ", "/", " ")
	normalized := replacer.Replace(camelCaseToSpaces(value))
	var out []string
	for _, token := range strings.Fields(strings.ToLower(normalized)) {
		if isWeakBindingToken(token) {
			continue
		}
		out = append(out, token)
	}
	return uniqueNonEmpty(out)
}

func significantClaimTokens(value string) []string {
	pattern := regexp.MustCompile(`[A-Za-z][A-Za-z0-9_/-]*`)
	var out []string
	for _, token := range pattern.FindAllString(strings.ToLower(value), -1) {
		for _, part := range significantIdentifierTokens(token) {
			if isWeakBindingToken(part) {
				continue
			}
			out = append(out, part)
		}
	}
	return uniqueNonEmpty(out)
}

func significantPathTokens(path string) []string {
	base := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	return significantIdentifierTokens(base + " " + path)
}

func isWeakBindingToken(token string) bool {
	switch strings.ToLower(strings.TrimSpace(token)) {
	case "", "the", "and", "for", "with", "from", "into", "after", "before", "mode", "question", "questions", "plain", "english", "architecture", "current", "changes", "change", "behavior", "handling", "handles", "internal", "internally", "exact", "evidence", "claim", "claims", "confirmed", "uncertain", "confidence", "file", "files", "code", "anchored":
		return true
	default:
		return false
	}
}

func intersectTokenCount(left, right []string) int {
	seen := map[string]struct{}{}
	for _, item := range left {
		seen[item] = struct{}{}
	}
	count := 0
	for _, item := range right {
		if _, ok := seen[item]; ok {
			count++
		}
	}
	return count
}

func camelCaseToSpaces(value string) string {
	var out []rune
	for i, r := range value {
		if i > 0 && r >= 'A' && r <= 'Z' {
			prev := rune(value[i-1])
			if (prev >= 'a' && prev <= 'z') || (prev >= '0' && prev <= '9') {
				out = append(out, ' ')
			}
		}
		out = append(out, r)
	}
	return string(out)
}

func findUnsupportedAnswerReferences(answer string, req routeRequest, chatContext workspaceChatContext) []string {
	support := strings.ToLower(strings.Join([]string{
		req.Message,
		chatContext.Text,
		strings.Join(chatContext.Targets, "\n"),
	}, "\n"))
	common := map[string]struct{}{
		"e.g": {},
		"i.e": {},
	}
	refPattern := regexp.MustCompile(`\b[A-Za-z_][A-Za-z0-9_]*(?:\.[A-Za-z_][A-Za-z0-9_]*)+\b|\b[A-Za-z_][A-Za-z0-9_]*\(\)`)
	pathPattern := regexp.MustCompile(`(?:[A-Za-z0-9._-]+/)+[A-Za-z0-9._-]+\.[A-Za-z0-9._-]+`)
	var unsupported []string
	for _, match := range refPattern.FindAllString(answer, -1) {
		normalized := strings.ToLower(strings.TrimSuffix(match, "()"))
		if _, skip := common[normalized]; skip {
			continue
		}
		if !strings.Contains(support, normalized) {
			unsupported = append(unsupported, match)
		}
	}
	allowedPaths := map[string]struct{}{}
	for _, rel := range uniqueNonEmpty(append(chatContext.Targets, explicitRepoFileMentions("", req.Message)...)) {
		allowedPaths[strings.ToLower(rel)] = struct{}{}
	}
	for _, match := range pathPattern.FindAllString(answer, -1) {
		lower := strings.ToLower(filepath.ToSlash(match))
		if !isAllowedAnswerPath(lower, allowedPaths) && !strings.Contains(support, lower) {
			unsupported = append(unsupported, match)
		}
	}
	return uniqueNonEmpty(unsupported)
}

func isAllowedAnswerPath(candidate string, allowedPaths map[string]struct{}) bool {
	candidate = strings.Trim(strings.ToLower(filepath.ToSlash(candidate)), " _*`\"'")
	if candidate == "" {
		return false
	}
	if _, ok := allowedPaths[candidate]; ok {
		return true
	}
	for allowed := range allowedPaths {
		if strings.HasSuffix(candidate, allowed) || strings.HasSuffix(allowed, candidate) {
			return true
		}
	}
	return false
}

func buildChatAnswerRepairPrompt(req routeRequest, answer string, chatContext workspaceChatContext, mcpText string, mcpLoop *boundedMCPContextResult, validation chatAnswerValidation) string {
	sections := []string{
		"Rewrite the answer so every substantive claim is supported only by the evidence DAG and retrieved files below.",
		"Required output format:",
		"## Confirmed",
		"- <claim>. Evidence: `<repo/path>` :: `<symbol-or-area>`",
		"## Uncertain",
		"- <anything not proven by the retrieved files>",
		"Do not mention fields, functions, routes, or files unless they appear in the retrieved context.",
		"Do not restate response instructions, citation policy, or abstain policy as if they were repository behavior.",
	}
	if len(validation.Issues) > 0 {
		sections = append(sections, "Validation issues:\n- "+strings.Join(validation.Issues, "\n- "))
	}
	if len(chatContext.Targets) > 0 {
		sections = append(sections, "Retrieved files:\n- "+strings.Join(chatContext.Targets, "\n- "))
	}
	if graphText := formatChatEvidenceGraph(chatContext.Evidence); strings.TrimSpace(graphText) != "" {
		sections = append(sections, "Evidence DAG:\n\n"+graphText)
	}
	if strings.TrimSpace(chatContext.Text) != "" {
		sections = append(sections, "Retrieved workspace context:\n\n"+clampString(chatContext.Text, 5000))
	}
	if strings.TrimSpace(mcpText) != "" {
		sections = append(sections, "MCP discovery context:\n\n"+clampString(mcpText, 1200))
	}
	if mcpLoop != nil && strings.TrimSpace(mcpLoop.Summary) != "" {
		sections = append(sections, "MCP bounded context loop:\n\n"+clampString(mcpLoop.Summary, 1800))
	}
	sections = append(sections,
		fmt.Sprintf("User request:\n%s", req.Message),
		fmt.Sprintf("Original answer to repair:\n%s", clampString(answer, 4000)),
	)
	return strings.Join(sections, "\n\n")
}

func buildEvidenceOnlyChatFallback(chatContext workspaceChatContext, validation chatAnswerValidation) string {
	lines := []string{
		"I couldn't verify a fully code-anchored answer from the retrieved context, so I'm limiting this to confirmed evidence.",
		"",
		"## Confirmed",
	}
	confirmed := summarizeStrictEvidenceGraph(chatContext.Evidence)
	if len(confirmed) == 0 {
		lines = append(lines, "- No repo files were retrieved with enough confidence to support stronger claims.")
	} else {
		for _, item := range confirmed {
			lines = append(lines, "- "+item)
		}
	}
	lines = append(lines, "", "## Uncertain", "- I can't confirm additional behavior beyond the retrieved snippets.")
	if len(validation.Issues) > 0 {
		lines = append(lines, "", "Suppressed unsupported claims:", "- "+strings.Join(validation.Issues, "\n- "))
	}
	return strings.Join(lines, "\n")
}

func summarizeStrictEvidenceGraph(graph chatEvidenceGraph) []string {
	var lines []string
	for _, node := range preferredChatEvidenceNodes(graph) {
		if node.Kind != "symbol" || node.File == "" || node.Symbol == "" {
			continue
		}
		lines = append(lines, summarizePreferredEvidenceNode(node))
	}
	if len(lines) > 0 {
		return uniqueNonEmpty(lines)
	}
	for _, node := range graph.Nodes {
		if node.Kind != "file" || node.File == "" {
			continue
		}
		lines = append(lines, fmt.Sprintf("Retrieved `%s` as relevant code context.", node.File))
	}
	return uniqueNonEmpty(lines)
}

func preferredChatEvidenceNodes(graph chatEvidenceGraph) []chatEvidenceNode {
	var symbols []chatEvidenceNode
	for _, node := range graph.Nodes {
		if node.Kind == "symbol" && node.File != "" && node.Symbol != "" {
			symbols = append(symbols, node)
		}
	}
	if len(symbols) == 0 {
		return nil
	}
	sort.SliceStable(symbols, func(i, j int) bool {
		if symbols[i].Score == symbols[j].Score {
			if symbols[i].File == symbols[j].File {
				return symbols[i].Symbol < symbols[j].Symbol
			}
			return symbols[i].File < symbols[j].File
		}
		return symbols[i].Score > symbols[j].Score
	})
	hasPositive := false
	for _, node := range symbols {
		if node.Score > 0 {
			hasPositive = true
			break
		}
	}
	if hasPositive {
		var positive []chatEvidenceNode
		for _, node := range symbols {
			if node.Score > 0 {
				positive = append(positive, node)
			}
		}
		symbols = positive
	}
	if len(symbols) > 4 {
		symbols = symbols[:4]
	}
	return symbols
}

func summarizePreferredEvidenceNode(node chatEvidenceNode) string {
	role := describeEvidenceRole(node.Symbol)
	return fmt.Sprintf("%s `%s` in `%s`. Evidence: `%s` :: `%s`", role, node.Symbol, node.File, node.File, node.Symbol)
}

func describeEvidenceRole(symbol string) string {
	lower := strings.ToLower(strings.TrimSpace(symbol))
	switch {
	case strings.Contains(lower, "handle") || strings.Contains(lower, "route"):
		return "Retained flow handler"
	case strings.Contains(lower, "resolve") || strings.Contains(lower, "select") || strings.Contains(lower, "policy"):
		return "Retained decision or policy stage"
	case strings.Contains(lower, "build") || strings.Contains(lower, "collect") || strings.Contains(lower, "retrieve") || strings.Contains(lower, "context"):
		return "Retained context or planning stage"
	case strings.Contains(lower, "validate") || strings.Contains(lower, "repair"):
		return "Retained validation or repair stage"
	case strings.Contains(lower, "begin") || strings.Contains(lower, "write") || strings.Contains(lower, "persist") || strings.Contains(lower, "emit"):
		return "Retained artifact or trace stage"
	default:
		return "Retained execution symbol"
	}
}

func extractLikelySnippetSymbols(snippet string) []string {
	if strings.TrimSpace(snippet) == "" {
		return nil
	}
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`^\s*func\s+([A-Za-z_][A-Za-z0-9_]*)`),
		regexp.MustCompile(`^\s*function\s+([A-Za-z_][A-Za-z0-9_]*)`),
		regexp.MustCompile(`^\s*class\s+([A-Za-z_][A-Za-z0-9_]*)`),
		regexp.MustCompile(`^\s*interface\s+([A-Za-z_][A-Za-z0-9_]*)`),
		regexp.MustCompile(`^\s*type\s+([A-Za-z_][A-Za-z0-9_]*)`),
	}
	var out []string
	for _, line := range strings.Split(snippet, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "*") || strings.HasPrefix(trimmed, "/*") {
			continue
		}
		for _, pattern := range patterns {
			match := pattern.FindStringSubmatch(line)
			if len(match) > 1 {
				out = append(out, match[1])
			}
		}
	}
	out = uniqueNonEmpty(out)
	if len(out) > 3 {
		return out[:3]
	}
	return out
}

func extractLikelySnippetSymbolsForFile(rel, snippet string, architectureQuery bool) []string {
	symbols := extractLikelySnippetSymbols(snippet)
	if len(symbols) == 0 {
		return nil
	}
	if !architectureQuery {
		return symbols
	}
	if isTestLikePath(rel) {
		var filtered []string
		for _, symbol := range symbols {
			if looksLikeTestSymbol(symbol) {
				continue
			}
			filtered = append(filtered, symbol)
		}
		return filtered
	}
	return symbols
}

func extractLikelySnippetSymbolsForFileWithTerms(rel, snippet string, searchTerms []string, architectureQuery bool) []string {
	symbols := extractLikelySnippetSymbolsNearTerms(snippet, searchTerms)
	if len(symbols) == 0 && !architectureQuery {
		symbols = extractLikelySnippetSymbols(snippet)
	}
	if len(symbols) == 0 {
		return nil
	}
	if !architectureQuery {
		return symbols
	}
	if isTestLikePath(rel) {
		var filtered []string
		for _, symbol := range symbols {
			if looksLikeTestSymbol(symbol) {
				continue
			}
			filtered = append(filtered, symbol)
		}
		return filtered
	}
	return symbols
}

func buildChatEvidenceGraph(root string, req routeRequest, targets []string, snippets map[string]string, searchTerms []string) chatEvidenceGraph {
	architectureQuery := isArchitectureChatQuery(req.Message)
	nodes := []chatEvidenceNode{{
		ID:      "request",
		Kind:    "request",
		Summary: strings.TrimSpace(req.Message),
	}}
	edges := []chatEvidenceEdge{}
	seenNodes := map[string]struct{}{"request": {}}
	extractor := reasoning.EvidenceExtractor{}
	for _, rel := range uniqueNonEmpty(targets) {
		snippet := strings.TrimSpace(snippets[rel])
		if rel == "" || snippet == "" {
			continue
		}
		evidenceRecord, evidenceErr := extractor.Extract(root, rel, searchTerms)
		if evidenceErr != nil {
			evidenceRecord = reasoning.EvidenceRecord{
				Nodes: []reasoning.EvidenceNode{{
					ID:      "file:" + rel,
					Kind:    "file",
					File:    rel,
					Summary: summarizeSnippetEvidence(snippet),
				}},
			}
		}
		fileID := "file:" + rel
		for _, node := range evidenceRecord.Nodes {
			if architectureQuery && isTestLikePath(rel) && node.Kind == "symbol" && looksLikeTestSymbol(node.Symbol) {
				continue
			}
			if _, ok := seenNodes[node.ID]; ok {
				continue
			}
			nodes = append(nodes, chatEvidenceNode{
				ID:      node.ID,
				Kind:    node.Kind,
				File:    node.File,
				Symbol:  node.Symbol,
				Summary: node.Summary,
				Score:   scoreArchitectureEvidenceNode(node, rel, evidenceRecord.Edges, architectureQuery, searchTerms),
			})
			seenNodes[node.ID] = struct{}{}
		}
		edges = append(edges, chatEvidenceEdge{From: "request", To: fileID, Kind: "grounds"})
		for _, edge := range evidenceRecord.Edges {
			edges = append(edges, chatEvidenceEdge{From: edge.From, To: edge.To, Kind: edge.Kind})
		}
	}
	uniqueNodes := uniqueChatEvidenceNodes(nodes)
	if architectureQuery {
		uniqueNodes = prioritizeArchitectureEvidenceNodes(uniqueNodes)
	}
	return chatEvidenceGraph{
		Nodes: uniqueNodes,
		Edges: uniqueChatEvidenceEdges(edges),
	}
}

func shouldKeepArchitectureSnippet(rel, snippet string, searchTerms []string) bool {
	if strings.TrimSpace(snippet) == "" {
		return false
	}
	if filepath.Base(rel) == strings.ToLower(filepath.Base(rel)) && strings.Contains(filepath.Base(rel), "request.go") {
		return true
	}
	if len(extractLikelySnippetSymbolsNearTerms(snippet, searchTerms)) > 0 {
		return true
	}
	lower := strings.ToLower(snippet)
	pathMatches := 0
	contentMatches := 0
	strongMatches := 0
	for _, term := range uniqueNonEmpty(searchTerms) {
		if strings.Contains(strings.ToLower(rel), term) {
			pathMatches++
			if !isWeakArchitectureTerm(term) {
				strongMatches++
			}
		}
		if strings.Contains(lower, term) {
			contentMatches++
			if !isWeakArchitectureTerm(term) {
				strongMatches++
			}
		}
	}
	return isDenseArchitectureMatch(false, pathMatches, contentMatches, strongMatches, 0)
}

func extractLikelySnippetSymbolsNearTerms(snippet string, searchTerms []string) []string {
	if strings.TrimSpace(snippet) == "" {
		return nil
	}
	lines := strings.Split(snippet, "\n")
	var matched []int
	for i, line := range lines {
		lower := strings.ToLower(line)
		for _, term := range uniqueNonEmpty(searchTerms) {
			if term != "" && strings.Contains(lower, term) {
				matched = append(matched, i)
				break
			}
		}
	}
	if len(matched) == 0 {
		return nil
	}
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`^\s*func\s+([A-Za-z_][A-Za-z0-9_]*)`),
		regexp.MustCompile(`^\s*function\s+([A-Za-z_][A-Za-z0-9_]*)`),
		regexp.MustCompile(`^\s*class\s+([A-Za-z_][A-Za-z0-9_]*)`),
		regexp.MustCompile(`^\s*interface\s+([A-Za-z_][A-Za-z0-9_]*)`),
		regexp.MustCompile(`^\s*type\s+([A-Za-z_][A-Za-z0-9_]*)`),
	}
	var out []string
	for i, line := range lines {
		if !isWithinMatchedDeclarationWindow(i, matched) {
			continue
		}
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "*") || strings.HasPrefix(trimmed, "/*") {
			continue
		}
		for _, pattern := range patterns {
			match := pattern.FindStringSubmatch(line)
			if len(match) > 1 {
				out = append(out, match[1])
			}
		}
	}
	out = uniqueNonEmpty(out)
	if len(out) > 3 {
		return out[:3]
	}
	return out
}

func isWithinMatchedDeclarationWindow(idx int, matched []int) bool {
	for _, m := range matched {
		if idx >= m && idx-m <= 4 {
			return true
		}
	}
	return false
}

func prioritizeArchitectureEvidenceNodes(nodes []chatEvidenceNode) []chatEvidenceNode {
	if len(nodes) == 0 {
		return nil
	}
	var requestNodes []chatEvidenceNode
	var fileNodes []chatEvidenceNode
	var symbolNodes []chatEvidenceNode
	for _, node := range nodes {
		switch node.Kind {
		case "request":
			requestNodes = append(requestNodes, node)
		case "file":
			fileNodes = append(fileNodes, node)
		case "symbol":
			symbolNodes = append(symbolNodes, node)
		}
	}
	sort.SliceStable(symbolNodes, func(i, j int) bool {
		if symbolNodes[i].Score == symbolNodes[j].Score {
			if symbolNodes[i].File == symbolNodes[j].File {
				return symbolNodes[i].Symbol < symbolNodes[j].Symbol
			}
			return symbolNodes[i].File < symbolNodes[j].File
		}
		return symbolNodes[i].Score > symbolNodes[j].Score
	})
	if len(symbolNodes) > 6 {
		symbolNodes = symbolNodes[:6]
	}
	return append(append(requestNodes, fileNodes...), symbolNodes...)
}

func scoreArchitectureEvidenceNode(node reasoning.EvidenceNode, rel string, edges []reasoning.EvidenceEdge, architectureQuery bool, searchTerms []string) int {
	score := 0
	if node.Kind == "file" {
		score += implementationTargetScore(rel)
	}
	if node.Kind != "symbol" {
		return score
	}
	score += scoreSymbolForArchitecture(node.Symbol)
	for _, edge := range edges {
		if edge.From == node.ID || edge.To == node.ID {
			score += 2
		}
	}
	if architectureQuery {
		for _, term := range searchTerms {
			lowerTerm := strings.ToLower(strings.TrimSpace(term))
			if lowerTerm != "" && strings.Contains(strings.ToLower(node.Symbol), lowerTerm) {
				score += 4
			}
		}
	}
	return score
}

func scoreSymbolForArchitecture(symbol string) int {
	lower := strings.ToLower(strings.TrimSpace(symbol))
	score := 0
	for _, token := range []string{"handle", "route", "run", "execute", "apply", "validate", "inspect", "resolve", "plan", "retrieve", "emit", "select", "decide", "branch", "repair", "collect", "buildcontext", "gather"} {
		if strings.Contains(lower, token) {
			score += 8
		}
	}
	for _, token := range []string{"format", "summary", "citation", "output", "clone", "normalize", "fallback", "sanitize"} {
		if strings.Contains(lower, token) {
			score -= 6
		}
	}
	if strings.HasPrefix(lower, "build") && score <= 0 {
		score -= 2
	}
	return score
}

func looksLikeMetaPolicyAnswer(answer string, req routeRequest, chatContext workspaceChatContext) bool {
	lower := strings.ToLower(answer)
	metaTerms := []string{
		"every substantive claim",
		"code-anchored",
		"exact citations",
		"do not guess",
		"if confidence is low",
		"abstain",
		"confirmed evidence",
		"citing evidence",
		"list only confirmed evidence",
	}
	metaHits := 0
	for _, term := range metaTerms {
		if strings.Contains(lower, term) {
			metaHits++
		}
	}
	if metaHits == 0 {
		return false
	}
	behaviorTerms := []string{
		"route", "retrieve", "gather", "validate", "repair", "select", "branch", "persist", "write", "emit", "score", "filter", "rank", "inspect",
	}
	behaviorHits := 0
	for _, term := range behaviorTerms {
		if strings.Contains(lower, term) {
			behaviorHits++
		}
	}
	symbolMentions := 0
	for _, node := range chatContext.Evidence.Nodes {
		if node.Kind == "symbol" && node.Symbol != "" && strings.Contains(lower, strings.ToLower(node.Symbol)) {
			symbolMentions++
		}
	}
	return metaHits >= 2 && symbolMentions == 0 && behaviorHits < 3
}

func summarizeSnippetEvidence(snippet string) string {
	text := strings.TrimSpace(snippet)
	if text == "" {
		return ""
	}
	lines := strings.Split(text, "\n")
	if len(lines) > 3 {
		lines = lines[:3]
	}
	return clampString(strings.Join(lines, " "), 180)
}

func isTestLikePath(rel string) bool {
	lower := strings.ToLower(filepath.ToSlash(rel))
	return strings.HasSuffix(lower, "_test.go") ||
		strings.Contains(lower, ".test.") ||
		strings.Contains(lower, ".spec.") ||
		strings.Contains(lower, "/test/") ||
		strings.Contains(lower, "/tests/") ||
		strings.Contains(lower, "/testdata/")
}

func isImplementationLikePath(rel string) bool {
	lower := strings.ToLower(filepath.ToSlash(rel))
	if isTestLikePath(lower) || isDocLikePath(lower) || isScriptLikePath(lower) || isClientSurfaceLikePath(lower) {
		return false
	}
	switch strings.ToLower(filepath.Ext(lower)) {
	case ".go", ".ts", ".tsx", ".js", ".jsx", ".py", ".rs", ".java", ".c", ".cc", ".cpp", ".h", ".hpp", ".cs", ".rb", ".php", ".swift", ".kt", ".kts":
		return true
	default:
		return false
	}
}

func isClientSurfaceLikePath(rel string) bool {
	lower := strings.ToLower(filepath.ToSlash(rel))
	return strings.Contains(lower, "/webview/") ||
		strings.Contains(lower, "/vscode-extension/") ||
		strings.Contains(lower, "/frontend/") ||
		strings.Contains(lower, "/ui/") ||
		strings.Contains(lower, "/client/")
}

func pruneClientSurfaceTargets(items []string) []string {
	all := uniqueNonEmpty(items)
	hasCoreImplementation := false
	for _, rel := range all {
		if !isClientSurfaceLikePath(rel) {
			hasCoreImplementation = true
			break
		}
	}
	if !hasCoreImplementation {
		return all
	}
	var out []string
	for _, rel := range all {
		if !isClientSurfaceLikePath(rel) {
			out = append(out, rel)
		}
	}
	if len(out) == 0 {
		return all
	}
	return out
}

func isDocLikePath(rel string) bool {
	lower := strings.ToLower(filepath.ToSlash(rel))
	return strings.Contains(lower, "/docs/") ||
		strings.Contains(lower, "/doc/") ||
		strings.HasSuffix(lower, ".md") ||
		strings.HasSuffix(lower, ".txt") ||
		strings.HasSuffix(lower, "readme")
}

func isScriptLikePath(rel string) bool {
	lower := strings.ToLower(filepath.ToSlash(rel))
	return strings.Contains(lower, "/scripts/") ||
		strings.HasSuffix(lower, ".sh") ||
		strings.HasSuffix(lower, ".bash") ||
		strings.HasSuffix(lower, ".zsh")
}

func looksLikeTestSymbol(symbol string) bool {
	lower := strings.ToLower(strings.TrimSpace(symbol))
	return strings.HasPrefix(lower, "test") ||
		strings.Contains(lower, "fixture") ||
		strings.Contains(lower, "mock") ||
		strings.Contains(lower, "fake")
}

func uniqueChatEvidenceNodes(nodes []chatEvidenceNode) []chatEvidenceNode {
	seen := map[string]struct{}{}
	var out []chatEvidenceNode
	for _, node := range nodes {
		if node.ID == "" {
			continue
		}
		if _, ok := seen[node.ID]; ok {
			continue
		}
		seen[node.ID] = struct{}{}
		out = append(out, node)
	}
	return out
}

func uniqueChatEvidenceEdges(edges []chatEvidenceEdge) []chatEvidenceEdge {
	seen := map[string]struct{}{}
	var out []chatEvidenceEdge
	for _, edge := range edges {
		key := edge.From + "|" + edge.Kind + "|" + edge.To
		if edge.From == "" || edge.To == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, edge)
	}
	return out
}

func countEvidenceNodesByKind(graph chatEvidenceGraph, kind string) int {
	count := 0
	for _, node := range graph.Nodes {
		if node.Kind == kind {
			count++
		}
	}
	return count
}

func formatChatEvidenceGraph(graph chatEvidenceGraph) string {
	if len(graph.Nodes) == 0 {
		return ""
	}
	var lines []string
	for _, edge := range graph.Edges {
		from := findChatEvidenceNode(graph, edge.From)
		to := findChatEvidenceNode(graph, edge.To)
		if from == nil || to == nil {
			continue
		}
		switch {
		case from.Kind == "request" && to.Kind == "file":
			lines = append(lines, fmt.Sprintf("- request -> file `%s` (%s)", to.File, emptyFallback(to.Summary, "retrieved context")))
		case from.Kind == "file" && to.Kind == "symbol":
			lines = append(lines, fmt.Sprintf("- file `%s` -> symbol `%s`", from.File, to.Symbol))
		}
	}
	return strings.Join(uniqueNonEmpty(lines), "\n")
}

func summarizeChatEvidenceGraph(graph chatEvidenceGraph) []string {
	var lines []string
	for _, node := range graph.Nodes {
		switch node.Kind {
		case "file":
			lines = append(lines, fmt.Sprintf("`%s`: %s", node.File, emptyFallback(node.Summary, "retrieved as relevant workspace context.")))
		case "symbol":
			lines = append(lines, fmt.Sprintf("`%s`: evidence graph includes symbol `%s`.", node.File, node.Symbol))
		}
	}
	return uniqueNonEmpty(lines)
}

func findChatEvidenceNode(graph chatEvidenceGraph, id string) *chatEvidenceNode {
	for i := range graph.Nodes {
		if graph.Nodes[i].ID == id {
			return &graph.Nodes[i]
		}
	}
	return nil
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
