package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	defaultEditModel      = "llama3.2"
	maxEditContextChars   = 6000
	maxEditSelectionChars = 2400
	maxEditSnippetPerFile = 1200
	maxEditCandidateCount = 4
	editContractVersion   = "v1"
	maxArtifactRepairPass = 2
	maxEditPromptChars    = 12000
	maxEditPlanChars      = 1600
	maxEditRetrievalChars = 2200
)

type editEvent struct {
	ContractVersion string         `json:"contract_version"`
	RequestID       string         `json:"request_id"`
	Type            string         `json:"type,omitempty"`
	DisplayText     string         `json:"display_text,omitempty"`
	Progress        float64        `json:"progress,omitempty"`
	Status          string         `json:"status,omitempty"`
	UserMessage     string         `json:"user_message,omitempty"`
	Metadata        map[string]any `json:"metadata,omitempty"`
	Error           *editError     `json:"error,omitempty"`
}

type editError struct {
	ErrorCode   string `json:"error_code"`
	UserMessage string `json:"user_message"`
	Retryable   bool   `json:"retryable"`
	Severity    string `json:"severity"`
}

type editModelArtifact struct {
	Summary     string   `json:"summary"`
	TargetFiles []string `json:"target_files"`
	Patch       string   `json:"patch"`
	Validations []string `json:"validations,omitempty"`
}

type editRequestRecord struct {
	ContractVersion string   `json:"contract_version"`
	RequestID       string   `json:"request_id"`
	WorkspaceRoot   string   `json:"workspace_root"`
	UserMessage     string   `json:"user_message"`
	ActiveFile      string   `json:"active_file,omitempty"`
	SelectionText   string   `json:"selection_text,omitempty"`
	Apply           bool     `json:"apply"`
	CandidateFiles  []string `json:"candidate_files,omitempty"`
	ContextPath     string   `json:"context_path,omitempty"`
}

type editPlan struct {
	Summary        string   `json:"summary"`
	TargetFiles    []string `json:"target_files,omitempty"`
	SearchTerms    []string `json:"search_terms,omitempty"`
	AllowNewFiles  bool     `json:"allow_new_files,omitempty"`
	Complexity     string   `json:"complexity,omitempty"`
	RetrievalStyle string   `json:"retrieval_style,omitempty"`
}

func editCmd(argv []string) {
	fs := flag.NewFlagSet("edit", flag.ExitOnError)
	workdir := fs.String("workdir", "", "working directory (default cwd)")
	message := fs.String("message", "", "edit request text")
	activeFile := fs.String("active-file", "", "repo-relative active file hint")
	selectionText := fs.String("selection-text", "", "active selection hint")
	apply := fs.Bool("apply", false, "apply the verified patch to the working tree")
	model := fs.String("model", "", "Ollama model override")
	ollamaHost := fs.String("ollama-host", "", "Ollama host override")
	numCtx := fs.Int("num-ctx", 0, "Ollama context window override")
	_ = fs.Parse(argv)
	if strings.TrimSpace(*message) == "" {
		fmt.Fprintln(os.Stderr, "edit: --message is required")
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

	host := strings.TrimSpace(*ollamaHost)
	if host == "" {
		host = strings.TrimSpace(os.Getenv("OLLAMA_HOST"))
	}
	if host == "" {
		host = "http://127.0.0.1:11434"
	}
	chosenModel := strings.TrimSpace(*model)
	if chosenModel == "" {
		chosenModel = strings.TrimSpace(os.Getenv("PIPEON_OLLAMA_MODEL"))
	}
	if chosenModel == "" {
		chosenModel = strings.TrimSpace(os.Getenv("DOCKPIPE_OLLAMA_MODEL"))
	}
	if chosenModel == "" {
		chosenModel = defaultEditModel
	}
	chosenNumCtx := resolveNumCtx(*numCtx)

	reqID := fmt.Sprintf("req_%d", time.Now().UnixNano())
	artifactsDir := filepath.Join(absWd, ".dorkpipe", "edit", reqID)
	if err := os.MkdirAll(artifactsDir, 0o755); err != nil {
		emitEditError(reqID, "INTERNAL_ERROR", fmt.Sprintf("Could not create artifact directory: %v", err), false)
		os.Exit(1)
	}

	ctx := context.Background()
	emitEditEvent(reqID, "received", "Received edit request", 0.05, nil)

	artifact, patchPath, artifactsDir, err := prepareEditArtifact(ctx, reqID, absWd, strings.TrimSpace(*message), strings.TrimSpace(*activeFile), strings.TrimSpace(*selectionText), host, chosenModel, chosenNumCtx, artifactsDir)
	if err != nil {
		os.Exit(1)
	}

	if !*apply {
		emitReadyToApply(reqID, absWd, artifactsDir, patchPath, artifact)
		return
	}

	if err := applyPreparedArtifact(ctx, reqID, absWd, artifactsDir, patchPath, artifact); err != nil {
		os.Exit(1)
	}
}

func applyEditCmd(argv []string) {
	fs := flag.NewFlagSet("apply-edit", flag.ExitOnError)
	workdir := fs.String("workdir", "", "working directory (default cwd)")
	artifactDir := fs.String("artifact-dir", "", "artifact directory produced by dorkpipe edit/request")
	_ = fs.Parse(argv)
	if strings.TrimSpace(*artifactDir) == "" {
		fmt.Fprintln(os.Stderr, "apply-edit: --artifact-dir is required")
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
	absArtifactDir := *artifactDir
	if !filepath.IsAbs(absArtifactDir) {
		absArtifactDir = filepath.Join(absWd, absArtifactDir)
	}
	reqID := fmt.Sprintf("req_%d", time.Now().UnixNano())
	ctx := context.Background()
	emitEditEvent(reqID, "received", "Received apply request", 0.05, nil)
	artifact, patchPath, err := loadPreparedArtifact(absArtifactDir)
	if err != nil {
		emitEditError(reqID, "INVALID_REQUEST", fmt.Sprintf("Could not load prepared artifact: %v", err), false)
		os.Exit(1)
	}
	if err := applyPreparedArtifact(ctx, reqID, absWd, absArtifactDir, patchPath, artifact); err != nil {
		os.Exit(1)
	}
}

func prepareEditArtifact(ctx context.Context, reqID, root, message, activeFile, selectionText, host, chosenModel string, chosenNumCtx int, artifactsDir string) (*editModelArtifact, string, string, error) {
	if artifact, patchPath, ok, err := tryDeterministicEditPrimitive(reqID, root, message, activeFile, artifactsDir); ok {
		return artifact, patchPath, artifactsDir, err
	}

	candidates, err := collectEditCandidates(ctx, root, activeFile, message, artifactsDir)
	if err != nil {
		emitEditError(reqID, "CONTEXT_GATHER_FAILED", fmt.Sprintf("Could not collect candidate files: %v", err), true)
		return nil, "", "", err
	}
	emitEditEvent(reqID, "context_gathering", "Collected candidate files", 0.18, map[string]any{
		"candidate_count": len(candidates),
	})

	contextPath, contextText := readEditContextBundle(root)
	baseRequest := editRequestRecord{
		ContractVersion: editContractVersion,
		RequestID:       reqID,
		WorkspaceRoot:   root,
		UserMessage:     message,
		ActiveFile:      activeFile,
		SelectionText:   clampString(selectionText, maxEditSelectionChars),
		Apply:           false,
		CandidateFiles:  candidates,
		ContextPath:     contextPath,
	}
	plan := buildDefaultEditPlan(root, baseRequest)
	if shouldUseComplexEditFlow(message, activeFile, selectionText) {
		emitEditEvent(reqID, "planning", "Planning edit strategy", 0.24, map[string]any{"candidate_count": len(candidates)})
		if len(plan.TargetFiles) >= 3 {
			emitEditEvent(reqID, "planning", "Using heuristic edit plan", 0.28, map[string]any{
				"planned_target_count": len(plan.TargetFiles),
			})
		} else {
			emitEditEvent(reqID, "planning", "Routing to planner model", 0.27, map[string]any{"model": chosenModel, "num_ctx": chosenNumCtx})
			if planned, planErr := buildEditPlan(ctx, host, chosenModel, chosenNumCtx, baseRequest, contextText, candidates); planErr == nil && planned != nil {
				plan = planned
				emitEditEvent(reqID, "planning", "Planner selected edit targets", 0.3, map[string]any{
					"planned_target_count": len(plan.TargetFiles),
				})
			} else {
				emitEditEvent(reqID, "planning", "Planner unavailable; using heuristic plan", 0.3, map[string]any{
					"planned_target_count": len(plan.TargetFiles),
				})
			}
		}
	}
	candidates = mergePlannedCandidates(candidates, plan.TargetFiles)
	emitEditEvent(reqID, "retrieving", "Ranking likely target files", 0.34, map[string]any{
		"candidate_count": len(candidates),
	})
	rankedCandidates := rankCandidates(root, candidates, message)
	emitEditEvent(reqID, "retrieving", "Building retrieval bundle", 0.4, map[string]any{
		"ranked_candidate_count": len(rankedCandidates),
	})
	snippets := readCandidateSnippets(root, rankedCandidates)
	retrievalBundle := collectRetrievalBundle(root, message, plan, rankedCandidates)
	requestRecord := editRequestRecord{
		ContractVersion: editContractVersion,
		RequestID:       reqID,
		WorkspaceRoot:   root,
		UserMessage:     message,
		ActiveFile:      activeFile,
		SelectionText:   clampString(selectionText, maxEditSelectionChars),
		Apply:           false,
		CandidateFiles:  rankedCandidates,
		ContextPath:     contextPath,
	}
	writeJSON(filepath.Join(artifactsDir, "request.json"), requestRecord)
	writeJSON(filepath.Join(artifactsDir, "plan.json"), plan)

	prompt := buildEditPrompt(requestRecord, plan, contextText, snippets, retrievalBundle)
	if err := os.WriteFile(filepath.Join(artifactsDir, "prompt.md"), []byte(prompt), 0o644); err != nil {
		emitEditError(reqID, "INTERNAL_ERROR", fmt.Sprintf("Could not write prompt artifact: %v", err), false)
		return nil, "", "", err
	}
	emitEditEvent(reqID, "routed", "Generating patch artifact", 0.48, map[string]any{
		"model": chosenModel,
		"num_ctx": chosenNumCtx,
	})

	modelText, err := runEditModel(ctx, host, chosenModel, chosenNumCtx, prompt)
	if err != nil {
		emitEditError(reqID, "MODEL_UNAVAILABLE", fmt.Sprintf("Ollama request failed: %v", err), true)
		return nil, "", "", err
	}
	if err := os.WriteFile(filepath.Join(artifactsDir, "model-response.txt"), []byte(modelText), 0o644); err != nil {
		emitEditError(reqID, "INTERNAL_ERROR", fmt.Sprintf("Could not write model response artifact: %v", err), false)
		return nil, "", "", err
	}

	artifact, err := parseEditArtifact(modelText)
	if err != nil {
		artifact, modelText, err = repairInvalidArtifactLoop(ctx, reqID, host, chosenModel, chosenNumCtx, prompt, modelText, fmt.Sprintf("Model output was not valid JSON: %v", err), artifactsDir)
		if err != nil {
			emitEditError(reqID, "MODEL_OUTPUT_INVALID", fmt.Sprintf("Model output was not a valid edit artifact: %v", err), true)
			return nil, "", "", err
		}
	}
	if err := validateEditArtifact(artifact); err != nil {
		artifact, modelText, err = repairInvalidArtifactLoop(ctx, reqID, host, chosenModel, chosenNumCtx, prompt, modelText, fmt.Sprintf("Edit artifact validation failed: %v", err), artifactsDir)
		if err != nil {
			emitEditError(reqID, "MODEL_OUTPUT_INVALID", fmt.Sprintf("Edit artifact validation failed: %v", err), true)
			return nil, "", "", err
		}
	}
	artifact.Patch = normalizeGeneratedPatch(artifact.Patch)
	writeJSON(filepath.Join(artifactsDir, "artifact.json"), artifact)

	patchPath := filepath.Join(artifactsDir, "patch.diff")
	if err := os.WriteFile(patchPath, []byte(artifact.Patch), 0o644); err != nil {
		emitEditError(reqID, "INTERNAL_ERROR", fmt.Sprintf("Could not write patch artifact: %v", err), false)
		return nil, "", "", err
	}

	emitEditEvent(reqID, "validating", "Checking patch applicability", 0.62, nil)
	verifyOutput, err := runRepoScript(ctx, root, "packages/dorkpipe/resolvers/dorkpipe/assets/scripts/verify-patch-applies.sh", patchPath, root)
	if err != nil {
		_ = os.WriteFile(filepath.Join(artifactsDir, "verify-patch.log"), []byte(verifyOutput), 0o644)
		artifact, modelText, err = repairInvalidArtifactLoop(ctx, reqID, host, chosenModel, chosenNumCtx, prompt, modelText, fmt.Sprintf("The generated patch did not apply cleanly.\n\nVerifier output:\n%s", emptyFallback(verifyOutput, "(no verifier output)")), artifactsDir)
		if err != nil {
			emitEditError(reqID, "VALIDATION_FAILED", "The generated patch did not apply cleanly.", true)
			return nil, "", "", err
		}
		writeJSON(filepath.Join(artifactsDir, "artifact.json"), artifact)
		patchPath = filepath.Join(artifactsDir, "patch.diff")
		if err := os.WriteFile(patchPath, []byte(artifact.Patch), 0o644); err != nil {
			emitEditError(reqID, "INTERNAL_ERROR", fmt.Sprintf("Could not write repaired patch artifact: %v", err), false)
			return nil, "", "", err
		}
		emitEditEvent(reqID, "validating", "Re-checking repaired patch", 0.7, nil)
		verifyOutput, err = runRepoScript(ctx, root, "packages/dorkpipe/resolvers/dorkpipe/assets/scripts/verify-patch-applies.sh", patchPath, root)
		if err != nil {
			_ = os.WriteFile(filepath.Join(artifactsDir, "verify-patch.log"), []byte(verifyOutput), 0o644)
			emitEditError(reqID, "VALIDATION_FAILED", "The generated patch did not apply cleanly.", true)
			return nil, "", "", err
		}
	}
	_ = os.WriteFile(filepath.Join(artifactsDir, "verify-patch.log"), []byte(verifyOutput), 0o644)
	return artifact, patchPath, artifactsDir, nil
}

func emitReadyToApply(reqID, root, artifactsDir, patchPath string, artifact *editModelArtifact) {
	emitEditEvent(reqID, "ready_to_apply", "Prepared a validated patch artifact", 0.8, map[string]any{
		"artifact_dir": relativeTo(root, artifactsDir),
		"patch_path":   relativeTo(root, patchPath),
		"target_files": artifact.TargetFiles,
	})
	emitEditDone(reqID, fmt.Sprintf("%s\n\nPrepared patch artifact at `%s`.", strings.TrimSpace(artifact.Summary), relativeTo(root, patchPath)), map[string]any{
		"route":             "edit",
		"files_touched":     len(artifact.TargetFiles),
		"validation_status": "patch_applies",
		"artifact_dir":      relativeTo(root, artifactsDir),
		"patch_path":        relativeTo(root, patchPath),
		"ready_to_apply":    true,
	})
}

func loadPreparedArtifact(artifactDir string) (*editModelArtifact, string, error) {
	artifactPath := filepath.Join(artifactDir, "artifact.json")
	patchPath := filepath.Join(artifactDir, "patch.diff")
	b, err := os.ReadFile(artifactPath)
	if err != nil {
		return nil, "", err
	}
	var artifact editModelArtifact
	if err := json.Unmarshal(b, &artifact); err != nil {
		return nil, "", err
	}
	if err := validateEditArtifact(&artifact); err != nil {
		return nil, "", err
	}
	return &artifact, patchPath, nil
}

func applyPreparedArtifact(ctx context.Context, reqID, root, artifactsDir, patchPath string, artifact *editModelArtifact) error {
	emitEditEvent(reqID, "applying", "Applying verified patch", 0.88, nil)
	applyOutput, err := runRepoScript(ctx, root, "packages/dorkpipe/resolvers/dorkpipe/assets/scripts/apply-unified-patch.sh", patchPath, root)
	if err != nil {
		_ = os.WriteFile(filepath.Join(artifactsDir, "apply.log"), []byte(applyOutput), 0o644)
		emitEditError(reqID, "APPLY_FAILED", "The patch was validated but could not be applied.", false)
		return err
	}
	_ = os.WriteFile(filepath.Join(artifactsDir, "apply.log"), []byte(applyOutput), 0o644)

	validationOutput, validationStatus := runPostApplyValidation(ctx, root, artifact.TargetFiles)
	_ = os.WriteFile(filepath.Join(artifactsDir, "post-apply-validation.log"), []byte(validationOutput), 0o644)

	emitEditDone(reqID, buildAppliedSummary(artifact, root, artifactsDir, validationStatus), map[string]any{
		"route":             "edit",
		"files_touched":     len(artifact.TargetFiles),
		"validation_status": validationStatus,
		"artifact_dir":      relativeTo(root, artifactsDir),
	})
	return nil
}

func emitEditEvent(requestID, typ, text string, progress float64, metadata map[string]any) {
	ev := editEvent{
		ContractVersion: editContractVersion,
		RequestID:       requestID,
		Type:            typ,
		DisplayText:     text,
		Progress:        progress,
		Metadata:        metadata,
	}
	writeEvent(ev)
}

func emitEditDone(requestID, userMessage string, metadata map[string]any) {
	ev := editEvent{
		ContractVersion: editContractVersion,
		RequestID:       requestID,
		Type:            "done",
		Status:          "ok",
		UserMessage:     userMessage,
		Metadata:        metadata,
	}
	writeEvent(ev)
}

func emitEditError(requestID, code, message string, retryable bool) {
	ev := editEvent{
		ContractVersion: editContractVersion,
		RequestID:       requestID,
		Type:            "error",
		Status:          "error",
		Error: &editError{
			ErrorCode:   code,
			UserMessage: message,
			Retryable:   retryable,
			Severity:    "warning",
		},
	}
	writeEvent(ev)
}

func writeEvent(ev editEvent) {
	b, _ := json.Marshal(ev)
	fmt.Println(string(b))
}

func collectEditCandidates(ctx context.Context, root, activeFile, message, artifactsDir string) ([]string, error) {
	candidateScript := filepath.Join(root, "packages/dorkpipe/resolvers/dorkpipe/assets/scripts/edit-collect-candidates.sh")
	args := []string{candidateScript, root}
	if strings.TrimSpace(activeFile) != "" {
		args = append(args, strings.TrimSpace(activeFile))
	} else {
		args = append(args, "")
	}
	args = append(args, message)
	cmd := exec.CommandContext(ctx, "bash", args...)
	cmd.Dir = root
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("%v: %s", err, strings.TrimSpace(stderr.String()))
	}
	lines := uniqueNonEmpty(strings.Split(stdout.String(), "\n"))
	if len(lines) == 0 && strings.TrimSpace(activeFile) != "" {
		lines = append(lines, strings.TrimSpace(activeFile))
	}
	if len(lines) > maxEditCandidateCount {
		lines = lines[:maxEditCandidateCount]
	}
	if err := os.WriteFile(filepath.Join(artifactsDir, "candidates.txt"), []byte(strings.Join(lines, "\n")), 0o644); err != nil {
		return nil, err
	}
	return lines, nil
}

func readEditContextBundle(root string) (string, string) {
	candidates := []string{
		filepath.Join(root, "bin", ".dockpipe", "pipeon-context.md"),
		filepath.Join(root, ".dockpipe", "pipeon-context.md"),
	}
	for _, p := range candidates {
		b, err := os.ReadFile(p)
		if err == nil {
			return relativeTo(root, p), clampString(string(b), maxEditContextChars)
		}
	}
	return "", ""
}

func readCandidateSnippets(root string, candidates []string) string {
	var parts []string
	for _, rel := range candidates {
		abs := filepath.Join(root, rel)
		b, err := os.ReadFile(abs)
		if err != nil {
			continue
		}
		parts = append(parts, fmt.Sprintf("## %s\n\n```text\n%s\n```", rel, clampString(string(b), maxEditSnippetPerFile)))
	}
	return strings.Join(parts, "\n\n")
}

func shouldUseComplexEditFlow(message, activeFile, selectionText string) bool {
	lower := strings.ToLower(strings.TrimSpace(message))
	if lower == "" {
		return false
	}
	if strings.Contains(lower, ".staging") || strings.Contains(lower, "staging") {
		return true
	}
	if strings.Contains(lower, "package") && (strings.Contains(lower, "make") || strings.Contains(lower, "create") || strings.Contains(lower, "author") || strings.Contains(lower, "new")) {
		return true
	}
	if strings.Contains(lower, "resolver") || strings.Contains(lower, "workflow") || strings.Contains(lower, "scaffold") {
		return true
	}
	if selectionText == "" && activeFile == "" && (strings.Contains(lower, "add") || strings.Contains(lower, "build") || strings.Contains(lower, "implement")) {
		return true
	}
	return false
}

func buildDefaultEditPlan(root string, req editRequestRecord) *editPlan {
	heuristicTargets := heuristicTargetsForRequest(root, req.UserMessage)
	plan := &editPlan{
		Summary:        "Use nearby workspace context and keep the edit narrowly scoped.",
		TargetFiles:    uniqueNonEmpty(append(heuristicTargets, req.CandidateFiles...)),
		SearchTerms:    extractSearchTerms(req.UserMessage),
		Complexity:     "simple",
		RetrievalStyle: "local_search",
	}
	if shouldUseComplexEditFlow(req.UserMessage, req.ActiveFile, req.SelectionText) {
		plan.Complexity = "complex"
		plan.AllowNewFiles = true
		plan.RetrievalStyle = "ranked_search"
	}
	return plan
}

func heuristicTargetsForRequest(root, message string) []string {
	lower := strings.ToLower(strings.TrimSpace(message))
	var out []string
	if strings.Contains(lower, ".staging") || strings.Contains(lower, "staging") {
		if matches, _ := filepath.Glob(filepath.Join(root, ".staging", "packages", "*", "package.yml")); len(matches) > 0 {
			for _, match := range matches {
				out = append(out, relativeTo(root, match))
			}
		}
		out = append(out, ".staging/packages/README.md")
	}
	if strings.Contains(lower, "package") {
		if matches, _ := filepath.Glob(filepath.Join(root, "packages", "*", "package.yml")); len(matches) > 0 {
			for _, match := range matches {
				out = append(out, relativeTo(root, match))
			}
		}
	}
	if strings.Contains(lower, "resolver") {
		if matches, _ := filepath.Glob(filepath.Join(root, ".staging", "packages", "*", "resolvers", "*", "config.yml")); len(matches) > 0 {
			for _, match := range matches {
				out = append(out, relativeTo(root, match))
			}
		}
	}
	return uniqueNonEmpty(out)
}

func buildEditPlan(ctx context.Context, host, model string, numCtx int, req editRequestRecord, contextText string, candidates []string) (*editPlan, error) {
	prompt := buildEditPlanPrompt(req, contextText, candidates)
	text, err := runEditModel(ctx, host, model, numCtx, prompt)
	if err != nil {
		return nil, err
	}
	return parseEditPlan(text)
}

func buildEditPlanPrompt(req editRequestRecord, contextText string, candidates []string) string {
	return strings.TrimSpace(fmt.Sprintf(`
You are DorkPipe planning a complex repository edit before patch generation.

Return JSON only. No markdown fences. The JSON shape must be:
{
  "summary": "short planning summary",
  "target_files": ["repo/relative/path"],
  "search_terms": ["term one", "term two"],
  "allow_new_files": true,
  "complexity": "simple or complex",
  "retrieval_style": "local_search or ranked_search"
}

Rules:
- Prefer repo-relative paths.
- Include likely package manifests, configs, and nearby files needed to author the edit.
- Use "complex" when the request sounds like authoring/scaffolding/new-package work.
- Keep search_terms short and practical.

User request:
%s

Active file:
%s

Selection:
%s

Current candidate files:
%s

Context bundle:
%s
`, req.UserMessage,
		emptyFallback(req.ActiveFile, "(none)"),
		emptyFallback(req.SelectionText, "(none)"),
		emptyFallback(strings.Join(candidates, "\n"), "(none)"),
		emptyFallback(contextText, "(no context bundle available)")))
}

func parseEditPlan(text string) (*editPlan, error) {
	clean := strings.TrimSpace(text)
	clean = strings.TrimPrefix(clean, "```json")
	clean = strings.TrimPrefix(clean, "```")
	clean = strings.TrimSuffix(clean, "```")
	clean = strings.TrimSpace(clean)
	start := strings.Index(clean, "{")
	end := strings.LastIndex(clean, "}")
	if start >= 0 && end > start {
		clean = clean[start : end+1]
	}
	var plan editPlan
	if err := json.Unmarshal([]byte(clean), &plan); err != nil {
		return nil, err
	}
	if strings.TrimSpace(plan.Summary) == "" {
		plan.Summary = "Plan a safe scoped edit."
	}
	plan.TargetFiles = uniqueNonEmpty(plan.TargetFiles)
	plan.SearchTerms = uniqueNonEmpty(plan.SearchTerms)
	if strings.TrimSpace(plan.Complexity) == "" {
		plan.Complexity = "complex"
	}
	if strings.TrimSpace(plan.RetrievalStyle) == "" {
		plan.RetrievalStyle = "ranked_search"
	}
	return &plan, nil
}

func extractSearchTerms(message string) []string {
	raw := strings.Fields(strings.ToLower(message))
	var terms []string
	for _, token := range raw {
		token = strings.Trim(token, ".,:;!?()[]{}\"'`")
		if len(token) < 4 {
			continue
		}
		if token == "with" || token == "that" || token == "make" || token == "your" || token == "real" || token == "test" {
			continue
		}
		terms = append(terms, token)
	}
	return uniqueNonEmpty(terms)
}

func mergePlannedCandidates(candidates []string, planned []string) []string {
	return uniqueNonEmpty(append(planned, candidates...))
}

func rankCandidates(root string, candidates []string, message string) []string {
	if len(candidates) == 0 {
		return nil
	}
	scored := uniqueNonEmpty(candidates)
	sort.SliceStable(scored, func(i, j int) bool {
		return candidatePathScore(message, scored[i]) > candidatePathScore(message, scored[j])
	})
	rankScript := filepath.Join(root, "packages/dorkpipe/resolvers/dorkpipe/assets/scripts/rank-candidate-files.sh")
	cmd := exec.Command("bash", rankScript)
	cmd.Dir = root
	cmd.Stdin = strings.NewReader(strings.Join(scored, "\n"))
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		if len(scored) > maxEditCandidateCount {
			return scored[:maxEditCandidateCount]
		}
		return scored
	}
	ranked := uniqueNonEmpty(strings.Split(stdout.String(), "\n"))
	if len(ranked) == 0 {
		if len(scored) > maxEditCandidateCount {
			return scored[:maxEditCandidateCount]
		}
		return scored
	}
	if len(ranked) > maxEditCandidateCount {
		ranked = ranked[:maxEditCandidateCount]
	}
	return ranked
}

func candidatePathScore(message, rel string) int {
	lowerMsg := strings.ToLower(message)
	lowerPath := strings.ToLower(rel)
	score := 0
	if strings.Contains(lowerMsg, ".staging") || strings.Contains(lowerMsg, "staging") {
		if strings.Contains(lowerPath, ".staging/") {
			score += 100
		}
	}
	if strings.Contains(lowerMsg, "package") {
		if strings.HasSuffix(lowerPath, "package.yml") {
			score += 80
		}
		if strings.Contains(lowerPath, "/packages/") {
			score += 35
		}
	}
	if strings.Contains(lowerMsg, "resolver") && strings.HasSuffix(lowerPath, "config.yml") {
		score += 45
	}
	if strings.HasSuffix(lowerPath, "readme.md") {
		score -= 15
	}
	if strings.HasSuffix(lowerPath, ".go") {
		score -= 10
	}
	return score
}

func collectRetrievalBundle(root, message string, plan *editPlan, candidates []string) string {
	var sections []string
	if plan != nil {
		sections = append(sections, fmt.Sprintf("## Plan\n\n- Summary: %s\n- Complexity: %s\n- Retrieval style: %s\n- Allow new files: %t",
			plan.Summary, emptyFallback(plan.Complexity, "unknown"), emptyFallback(plan.RetrievalStyle, "unknown"), plan.AllowNewFiles))
		if len(plan.TargetFiles) > 0 {
			sections = append(sections, "## Planned targets\n\n- "+strings.Join(plan.TargetFiles, "\n- "))
		}
		if len(plan.SearchTerms) > 0 {
			sections = append(sections, "## Search terms\n\n- "+strings.Join(plan.SearchTerms, "\n- "))
		}
	}
	searchTerms := extractSearchTerms(message)
	if plan != nil && len(plan.SearchTerms) > 0 {
		searchTerms = uniqueNonEmpty(append(plan.SearchTerms, searchTerms...))
	}
	if len(searchTerms) > 0 {
		var matches []string
		files := append([]string{}, candidates...)
		if plan != nil {
			files = append(files, plan.TargetFiles...)
		}
		files = uniqueNonEmpty(files)
		for _, rel := range files {
			abs := filepath.Join(root, rel)
			b, err := os.ReadFile(abs)
			if err != nil {
				continue
			}
			lower := strings.ToLower(string(b))
			for _, term := range searchTerms {
				if strings.Contains(lower, strings.ToLower(term)) {
					matches = append(matches, fmt.Sprintf("- %s matches %q", rel, term))
					break
				}
			}
			if len(matches) >= 12 {
				break
			}
		}
		if len(matches) > 0 {
			sections = append(sections, "## Retrieval bundle\n\n"+strings.Join(matches, "\n"))
		}
	}
	return strings.Join(sections, "\n\n")
}

func formatEditPlan(plan *editPlan) string {
	if plan == nil {
		return ""
	}
	parts := []string{
		fmt.Sprintf("Summary: %s", emptyFallback(plan.Summary, "(none)")),
		fmt.Sprintf("Complexity: %s", emptyFallback(plan.Complexity, "unknown")),
		fmt.Sprintf("Retrieval style: %s", emptyFallback(plan.RetrievalStyle, "unknown")),
		fmt.Sprintf("Allow new files: %t", plan.AllowNewFiles),
	}
	if len(plan.TargetFiles) > 0 {
		parts = append(parts, "Target files:\n- "+strings.Join(plan.TargetFiles, "\n- "))
	}
	if len(plan.SearchTerms) > 0 {
		parts = append(parts, "Search terms:\n- "+strings.Join(plan.SearchTerms, "\n- "))
	}
	return strings.Join(parts, "\n")
}

func buildEditPrompt(req editRequestRecord, plan *editPlan, contextText, snippets, retrievalBundle string) string {
	applyLine := "Do not assume the patch will be applied automatically."
	if req.Apply {
		applyLine = "The user explicitly requested an applied edit; keep the patch minimal and safe."
	}
	planText := clampString(emptyFallback(formatEditPlan(plan), "(no explicit plan)"), maxEditPlanChars)
	retrievalText := clampString(emptyFallback(retrievalBundle, "(no retrieval bundle available)"), maxEditRetrievalChars)
	snippetText := clampString(emptyFallback(snippets, "(no candidate file snippets available)"), maxEditPromptChars/3)
	contextBudget := maxEditPromptChars - len(planText) - len(retrievalText) - len(snippetText) - 2200
	if contextBudget < 1200 {
		contextBudget = 1200
	}
	contextText = clampString(emptyFallback(contextText, "(no context bundle available)"), contextBudget)
	prompt := strings.TrimSpace(fmt.Sprintf(`
You are DorkPipe preparing a bounded edit artifact for a local repository.

Return JSON only. No markdown fences. The JSON shape must be:
{
  "summary": "short user-facing summary",
  "target_files": ["path/one", "path/two"],
  "patch": "unified diff patch",
  "validations": ["short validation note"]
}

Rules:
- Keep changes minimal and directly tied to the request.
- Patch must be a valid unified diff that can be applied with git apply.
- Use repo-relative paths in target_files.
- Do not invent files not supported by the provided context unless the request clearly needs a new file.
- Prefer editing the active file when it is relevant.
- If a plan is provided, follow it unless the retrieved code clearly contradicts it.
- %s

User request:
%s

Plan:
%s

Active file:
%s

Selection:
%s

Context bundle:
%s

Retrieval bundle:
%s

Candidate file snippets:
%s
`, applyLine,
		req.UserMessage,
		planText,
		emptyFallback(req.ActiveFile, "(none)"),
		emptyFallback(req.SelectionText, "(none)"),
		contextText,
		retrievalText,
		snippetText))
	if len(prompt) > maxEditPromptChars {
		prompt = clampString(prompt, maxEditPromptChars)
	}
	return prompt
}

func runEditModel(ctx context.Context, host, model string, numCtx int, prompt string) (string, error) {
	u, err := buildOllamaChatURL(host)
	if err != nil {
		return "", err
	}
	payload := map[string]any{
		"model":  model,
		"stream": false,
		"messages": []map[string]string{
			{
				"role":    "system",
				"content": "You produce structured edit artifacts for a local coding assistant.",
			},
			{
				"role":    "user",
				"content": prompt,
			},
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
	client := &http.Client{Timeout: 15 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	out, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(out))
	}
	var parsed struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
		Response string `json:"response"`
	}
	if err := json.Unmarshal(out, &parsed); err != nil {
		return "", err
	}
	if strings.TrimSpace(parsed.Message.Content) != "" {
		return parsed.Message.Content, nil
	}
	return parsed.Response, nil
}

func buildOllamaChatURL(rawBase string) (*url.URL, error) {
	s := strings.TrimSpace(rawBase)
	s = strings.TrimSuffix(s, "/")
	if s == "" {
		return nil, errors.New("empty ollama host")
	}
	if !strings.Contains(s, "://") {
		s = "http://" + s
	}
	u, err := url.Parse(s)
	if err != nil {
		return nil, err
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("unsupported ollama URL scheme %q", u.Scheme)
	}
	if strings.TrimSpace(u.Host) == "" {
		return nil, errors.New("ollama host is missing")
	}
	return &url.URL{Scheme: u.Scheme, Host: u.Host, Path: "/api/chat"}, nil
}

func parseEditArtifact(text string) (*editModelArtifact, error) {
	clean := strings.TrimSpace(text)
	clean = strings.TrimPrefix(clean, "```json")
	clean = strings.TrimPrefix(clean, "```")
	clean = strings.TrimSuffix(clean, "```")
	clean = strings.TrimSpace(clean)
	start := strings.Index(clean, "{")
	end := strings.LastIndex(clean, "}")
	if start >= 0 && end > start {
		clean = clean[start : end+1]
	}
	var artifact editModelArtifact
	if err := json.Unmarshal([]byte(clean), &artifact); err != nil {
		return nil, err
	}
	return &artifact, nil
}

func normalizeGeneratedPatch(patch string) string {
	p := strings.ReplaceAll(patch, "\r\n", "\n")
	p = strings.ReplaceAll(p, " 100644 --- a/", " 100644\n--- a/")
	p = strings.ReplaceAll(p, " 100755 --- a/", " 100755\n--- a/")
	lines := strings.Split(p, "\n")
	for i, line := range lines {
		switch {
		case strings.HasPrefix(line, "-diff --git a/"):
			lines[i] = strings.TrimPrefix(line, "-")
		case strings.HasPrefix(line, "-index "):
			lines[i] = strings.TrimPrefix(line, "-")
		case strings.HasPrefix(line, "---- a/"):
			lines[i] = strings.TrimPrefix(line, "-")
		case strings.HasPrefix(line, "-+++ b/"):
			lines[i] = strings.TrimPrefix(line, "-")
		}
	}
	return strings.TrimSpace(strings.Join(lines, "\n")) + "\n"
}

func repairInvalidArtifactLoop(ctx context.Context, reqID, host, model string, numCtx int, originalPrompt, previousOutput, reason, artifactsDir string) (*editModelArtifact, string, error) {
	var lastErr error
	currentOutput := previousOutput
	for attempt := 1; attempt <= maxArtifactRepairPass; attempt++ {
		artifact, repairedOutput, err := retryInvalidEditArtifact(ctx, reqID, host, model, numCtx, originalPrompt, currentOutput, reason, artifactsDir, attempt)
		if err == nil {
			return artifact, repairedOutput, nil
		}
		lastErr = err
		currentOutput = repairedOutput
	}
	return nil, currentOutput, lastErr
}

func retryInvalidEditArtifact(ctx context.Context, reqID, host, model string, numCtx int, originalPrompt, previousOutput, reason, artifactsDir string, attempt int) (*editModelArtifact, string, error) {
	emitEditEvent(reqID, "repairing", fmt.Sprintf("Repairing invalid patch artifact (pass %d/%d)", attempt, maxArtifactRepairPass), 0.52, map[string]any{
		"model":   model,
		"attempt": attempt,
		"num_ctx": numCtx,
	})
	retryPrompt := buildEditArtifactRepairPrompt(originalPrompt, previousOutput, reason)
	modelText, err := runEditModel(ctx, host, model, numCtx, retryPrompt)
	if err != nil {
		return nil, previousOutput, err
	}
	suffix := ""
	if attempt > 1 {
		suffix = fmt.Sprintf("-%d", attempt)
	}
	_ = os.WriteFile(filepath.Join(artifactsDir, "model-response-repair"+suffix+".txt"), []byte(modelText), 0o644)
	artifact, err := parseEditArtifact(modelText)
	if err != nil {
		return nil, modelText, err
	}
	if err := validateEditArtifact(artifact); err != nil {
		return nil, modelText, err
	}
	return artifact, modelText, nil
}

func buildEditArtifactRepairPrompt(originalPrompt, previousOutput, reason string) string {
	return strings.TrimSpace(fmt.Sprintf(`%s

Your previous response was invalid.

Problem:
%s

Previous response:
%s

Return JSON only. No markdown fences. Do not truncate the JSON.

The "patch" field must contain a valid unified diff. Metadata lines are not file deletions.
Every edited file must look like this:

diff --git a/path/to/file b/path/to/file
index 1234567..89abcde 100644
--- a/path/to/file
+++ b/path/to/file
@@ -1,2 +1,2 @@
-old line
+new line

Rules:
- "diff --git", "index", "--- a/...", and "+++ b/..." must each be on their own lines
- never prefix diff metadata lines with "-" or "+"
- only changed file content lines inside hunks may begin with "-" or "+"
- the patch must apply with git apply
`, originalPrompt, reason, emptyFallback(previousOutput, "(empty response)")))
}

func validateEditArtifact(artifact *editModelArtifact) error {
	if strings.TrimSpace(artifact.Summary) == "" {
		return errors.New("summary is empty")
	}
	if strings.TrimSpace(artifact.Patch) == "" {
		return errors.New("patch is empty")
	}
	if !strings.Contains(artifact.Patch, "diff --git ") {
		return errors.New("patch does not look like a unified diff")
	}
	if !strings.Contains(artifact.Patch, "\n--- a/") || !strings.Contains(artifact.Patch, "\n+++ b/") {
		return errors.New("patch is missing unified diff file headers")
	}
	if len(artifact.TargetFiles) == 0 {
		return errors.New("target_files is empty")
	}
	for _, file := range artifact.TargetFiles {
		if strings.TrimSpace(file) == "" {
			return errors.New("target_files contains an empty path")
		}
		if strings.HasPrefix(file, "/") || strings.Contains(file, "..") {
			return fmt.Errorf("unsafe target file %q", file)
		}
	}
	return nil
}

func runRepoScript(ctx context.Context, root, relScript string, args ...string) (string, error) {
	script := filepath.Join(root, relScript)
	cmdArgs := append([]string{script}, args...)
	cmd := exec.CommandContext(ctx, "bash", cmdArgs...)
	cmd.Dir = root
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

func runPostApplyValidation(ctx context.Context, root string, targetFiles []string) (string, string) {
	relFiles := uniqueNonEmpty(targetFiles)
	if len(relFiles) == 0 {
		return "No post-apply validation requested.", "not_run"
	}
	args := append([]string{
		filepath.Join(root, "packages/dorkpipe/resolvers/dorkpipe/assets/scripts/edit-validate-applied.sh"),
		root,
	}, relFiles...)
	cmd := exec.CommandContext(ctx, "bash", args...)
	cmd.Dir = root
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	text := strings.TrimSpace(strings.TrimSpace(stdout.String()) + "\n" + strings.TrimSpace(stderr.String()))
	if err != nil {
		if text == "" {
			text = err.Error()
		}
		return text, "failed"
	}
	if text == "" {
		text = "No targeted validation executed."
	}
	return text, "passed"
}

func buildAppliedSummary(artifact *editModelArtifact, root, artifactsDir, validationStatus string) string {
	files := strings.Join(artifact.TargetFiles, ", ")
	base := strings.TrimSpace(artifact.Summary)
	if base == "" {
		base = "Applied a verified edit patch."
	}
	return fmt.Sprintf("%s\n\nFiles: `%s`\nValidation: `%s`\nArtifacts: `%s`", base, files, validationStatus, relativeTo(root, artifactsDir))
}

func writeJSON(path string, value any) {
	b, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(path, b, 0o644)
}

func clampString(s string, limit int) string {
	if len(s) <= limit {
		return s
	}
	return s[:limit] + "\n[truncated]"
}

func uniqueNonEmpty(items []string) []string {
	seen := make(map[string]struct{}, len(items))
	var out []string
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		out = append(out, item)
	}
	return out
}

func relativeTo(root, target string) string {
	rel, err := filepath.Rel(root, target)
	if err != nil {
		return target
	}
	return rel
}

func emptyFallback(s, fallback string) string {
	if strings.TrimSpace(s) == "" {
		return fallback
	}
	return s
}

func tryDeterministicEditPrimitive(reqID, root, message, activeFile, artifactsDir string) (*editModelArtifact, string, bool, error) {
	targetFile, lineText, ok := deterministicReadmeAppend(root, message, activeFile)
	if !ok {
		return nil, "", false, nil
	}
	emitEditEvent(reqID, "context_gathering", "Using deterministic README edit primitive", 0.18, map[string]any{
		"target_file": targetFile,
	})
	beforeBytes, err := os.ReadFile(filepath.Join(root, targetFile))
	if err != nil {
		emitEditError(reqID, "CONTEXT_GATHER_FAILED", fmt.Sprintf("Could not read %s: %v", targetFile, err), false)
		return nil, "", true, err
	}
	before := string(beforeBytes)
	if strings.Contains(before, lineText) {
		artifact := &editModelArtifact{
			Summary:     fmt.Sprintf("`%s` already contains the requested proof text.", targetFile),
			TargetFiles: []string{targetFile},
			Patch:       buildNoopPatch(targetFile),
			Validations: []string{"No change was needed."},
		}
		patchPath := filepath.Join(artifactsDir, "patch.diff")
		if err := os.WriteFile(patchPath, []byte(artifact.Patch), 0o644); err != nil {
			return nil, "", true, err
		}
		writeJSON(filepath.Join(artifactsDir, "artifact.json"), artifact)
		_ = os.WriteFile(filepath.Join(artifactsDir, "verify-patch.log"), []byte("No patch generated; requested text already present."), 0o644)
		return artifact, patchPath, true, nil
	}

	artifact := &editModelArtifact{
		Summary:     fmt.Sprintf("Append the requested proof text to `%s`.", targetFile),
		TargetFiles: []string{targetFile},
		Patch:       buildAppendLinePatch(targetFile, before, lineText),
		Validations: []string{"Verify the README now includes the requested proof text."},
	}
	if err := validateEditArtifact(artifact); err != nil {
		emitEditError(reqID, "MODEL_OUTPUT_INVALID", fmt.Sprintf("Deterministic edit artifact validation failed: %v", err), false)
		return nil, "", true, err
	}
	writeJSON(filepath.Join(artifactsDir, "artifact.json"), artifact)
	patchPath := filepath.Join(artifactsDir, "patch.diff")
	if err := os.WriteFile(patchPath, []byte(artifact.Patch), 0o644); err != nil {
		return nil, "", true, err
	}
	_ = os.WriteFile(filepath.Join(artifactsDir, "verify-patch.log"), []byte("Deterministic README append primitive selected."), 0o644)
	return artifact, patchPath, true, nil
}

func deterministicReadmeAppend(root, message, activeFile string) (string, string, bool) {
	msg := strings.TrimSpace(message)
	lower := strings.ToLower(msg)
	targetFile := ""
	switch {
	case strings.EqualFold(activeFile, "README.md"):
		targetFile = "README.md"
	case strings.Contains(strings.ToLower(activeFile), "readme"):
		targetFile = activeFile
	case strings.Contains(lower, "main project readme"), strings.Contains(lower, "main repo readme"), strings.Contains(lower, "project readme"):
		targetFile = "README.md"
	case strings.Contains(lower, "readme"):
		targetFile = "README.md"
	}
	if targetFile == "" {
		return "", "", false
	}
	if _, err := os.Stat(filepath.Join(root, targetFile)); err != nil {
		return "", "", false
	}

	lineText := ""
	if idx := strings.LastIndex(lower, " like "); idx >= 0 {
		lineText = strings.TrimSpace(msg[idx+6:])
	}
	if lineText == "" {
		if idx := strings.LastIndex(lower, " saying "); idx >= 0 {
			lineText = strings.TrimSpace(msg[idx+8:])
		}
	}
	if lineText == "" {
		lineText = extractQuotedText(msg)
	}
	if lineText == "" {
		return "", "", false
	}
	lineText = strings.TrimSpace(strings.Trim(lineText, "`\"'"))
	if lineText == "" {
		return "", "", false
	}
	return targetFile, lineText, true
}

func extractQuotedText(message string) string {
	for _, quote := range []string{`"`, `'`, "`"} {
		start := strings.Index(message, quote)
		end := strings.LastIndex(message, quote)
		if start >= 0 && end > start {
			return message[start+1 : end]
		}
	}
	return ""
}

func buildAppendLinePatch(targetFile, before, lineText string) string {
	lines := strings.Split(before, "\n")
	oldCount := len(lines)
	if before == "" {
		oldCount = 0
		lines = nil
	}
	newLines := append(append([]string{}, lines...), "", lineText)
	newCount := len(newLines)
	var b strings.Builder
	fmt.Fprintf(&b, "diff --git a/%s b/%s\n", targetFile, targetFile)
	fmt.Fprintf(&b, "--- a/%s\n", targetFile)
	fmt.Fprintf(&b, "+++ b/%s\n", targetFile)
	fmt.Fprintf(&b, "@@ -%d,%d +%d,%d @@\n", oldCount+1, 0, oldCount+1, 2)
	b.WriteString("+\n")
	b.WriteString("+")
	b.WriteString(lineText)
	b.WriteString("\n")
	_ = newCount
	return b.String()
}

func buildNoopPatch(targetFile string) string {
	return fmt.Sprintf("diff --git a/%s b/%s\n", targetFile, targetFile) +
		fmt.Sprintf("--- a/%s\n", targetFile) +
		fmt.Sprintf("+++ b/%s\n", targetFile)
}
