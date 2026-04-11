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
	Summary      string            `json:"summary"`
	TargetFiles  []string          `json:"target_files"`
	Patch        string            `json:"patch"`
	Validations  []string          `json:"validations,omitempty"`
	HelperScript *editHelperScript `json:"helper_script,omitempty"`
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

type editHelperScript struct {
	Runtime string `json:"runtime"`
	Purpose string `json:"purpose"`
	Content string `json:"content"`
}

type editHelperResponse struct {
	Summary      string            `json:"summary"`
	HelperScript *editHelperScript `json:"helper_script"`
}

type packageScaffoldSpec struct {
	PackageRoot   string
	PackageName   string
	PackageDir    string
	ManifestPath  string
	ReadmePath    string
	ManifestBody  string
	ReadmeBody    string
	TargetFiles   []string
	Summary       string
	ValidationMsg string
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
	emitEditEvent(reqID, "decomposing", "Breaking the request into primitives", 0.08, nil)
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

	if shouldTryHelperSidecar(reqID, baseRequest, plan) {
		emitEditEvent(reqID, "scripting", "Generating bounded helper script", 0.44, map[string]any{
			"model":   chosenModel,
			"num_ctx": chosenNumCtx,
		})
		if artifact, patchPath, used, sidecarErr := tryHelperSidecarPatch(ctx, reqID, root, host, chosenModel, chosenNumCtx, requestRecord, plan, snippets, retrievalBundle, artifactsDir); used {
			if sidecarErr == nil {
				return artifact, patchPath, artifactsDir, nil
			}
			emitEditEvent(reqID, "scripting", "Helper script fell back to patch generation", 0.46, map[string]any{
				"reason": sidecarErr.Error(),
			})
		}
	}

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
	metadata := map[string]any{
		"artifact_dir": relativeTo(root, artifactsDir),
		"patch_path":   relativeTo(root, patchPath),
		"target_files": artifact.TargetFiles,
	}
	if artifact.HelperScript != nil {
		helperPath := filepath.Join(artifactsDir, "sidecar", "helper.sh")
		metadata["helper_script_used"] = true
		metadata["helper_script_runtime"] = artifact.HelperScript.Runtime
		metadata["helper_script_purpose"] = artifact.HelperScript.Purpose
		metadata["helper_script_path"] = relativeTo(root, helperPath)
	}
	emitEditEvent(reqID, "ready_to_apply", "Prepared a validated patch artifact", 0.8, metadata)
	emitEditDone(reqID, fmt.Sprintf("%s\n\nPrepared patch artifact at `%s`.", strings.TrimSpace(artifact.Summary), relativeTo(root, patchPath)), map[string]any{
		"route":             "edit",
		"files_touched":     len(artifact.TargetFiles),
		"validation_status": "patch_applies",
		"artifact_dir":      relativeTo(root, artifactsDir),
		"patch_path":        relativeTo(root, patchPath),
		"ready_to_apply":    true,
		"helper_script_used": artifact.HelperScript != nil,
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

func shouldTryHelperSidecar(reqID string, req editRequestRecord, plan *editPlan) bool {
	lower := strings.ToLower(req.UserMessage)
	if plan != nil && strings.EqualFold(plan.Complexity, "complex") && plan.AllowNewFiles {
		return true
	}
	for _, token := range []string{"package", "workflow", "resolver", "scaffold", "author", "create", "make"} {
		if strings.Contains(lower, token) {
			return true
		}
	}
	return false
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

func buildHelperSidecarPrompt(req editRequestRecord, plan *editPlan, snippets, retrievalBundle string) string {
	planText := clampString(emptyFallback(formatEditPlan(plan), "(no explicit plan)"), maxEditPlanChars)
	retrievalText := clampString(emptyFallback(retrievalBundle, "(no retrieval bundle available)"), maxEditRetrievalChars)
	snippetText := clampString(emptyFallback(snippets, "(no candidate file snippets available)"), maxEditPromptChars/4)
	return strings.TrimSpace(fmt.Sprintf(`
You are DorkPipe generating a bounded helper script for a local repository edit.

Return JSON only. No markdown fences. The JSON shape must be:
{
  "summary": "short user-facing summary",
  "helper_script": {
    "runtime": "bash",
    "purpose": "what the helper is doing",
    "content": "bash script that prints a unified diff to stdout"
  }
}

Rules:
- The script must only PRINT a unified diff patch to stdout.
- The script must not modify the repository directly.
- The script must not use network access, package managers, git apply, git checkout, git reset, rm, mv, sudo, or chmod.
- Prefer printf/cat heredocs and simple shell text generation over clever tricks.
- The patch must be valid for git apply once the script runs.
- Keep the script short and deterministic.

User request:
%s

Plan:
%s

Active file:
%s

Selection:
%s

Retrieval bundle:
%s

Candidate file snippets:
%s
`, req.UserMessage, planText, emptyFallback(req.ActiveFile, "(none)"), emptyFallback(req.SelectionText, "(none)"), retrievalText, snippetText))
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

func tryHelperSidecarPatch(ctx context.Context, reqID, root, host, model string, numCtx int, req editRequestRecord, plan *editPlan, snippets, retrievalBundle, artifactsDir string) (*editModelArtifact, string, bool, error) {
	prompt := buildHelperSidecarPrompt(req, plan, snippets, retrievalBundle)
	modelText, err := runEditModel(ctx, host, model, numCtx, prompt)
	if err != nil {
		return nil, "", true, err
	}
	_ = os.WriteFile(filepath.Join(artifactsDir, "helper-response.txt"), []byte(modelText), 0o644)
	resp, err := parseHelperSidecarResponse(modelText)
	if err != nil {
		return nil, "", true, err
	}
	emitEditEvent(reqID, "scripting", "Running bounded helper script", 0.5, map[string]any{
		"runtime": resp.HelperScript.Runtime,
		"purpose": resp.HelperScript.Purpose,
	})
	patchText, helperPath, err := runHelperSidecarScript(ctx, root, artifactsDir, req, resp.HelperScript)
	if err != nil {
		return nil, "", true, err
	}
	patchText = normalizeGeneratedPatch(patchText)
	targetFiles := targetFilesFromPatch(patchText)
	artifact := &editModelArtifact{
		Summary:      resp.Summary,
		TargetFiles:  targetFiles,
		Patch:        patchText,
		Validations:  []string{"Verify the helper-generated patch still matches the requested scope."},
		HelperScript: resp.HelperScript,
	}
	if err := validateEditArtifact(artifact); err != nil {
		return nil, "", true, err
	}
	writeJSON(filepath.Join(artifactsDir, "artifact.json"), artifact)
	patchPath := filepath.Join(artifactsDir, "patch.diff")
	if err := os.WriteFile(patchPath, []byte(artifact.Patch), 0o644); err != nil {
		return nil, "", true, err
	}
	verifyOutput, err := runRepoScript(ctx, root, "packages/dorkpipe/resolvers/dorkpipe/assets/scripts/verify-patch-applies.sh", patchPath, root)
	_ = os.WriteFile(filepath.Join(artifactsDir, "verify-patch.log"), []byte(verifyOutput), 0o644)
	if err != nil {
		return nil, "", true, err
	}
	emitEditEvent(reqID, "scripting", "Helper script produced a valid patch", 0.58, map[string]any{
		"helper_path": relativeTo(root, helperPath),
	})
	return artifact, patchPath, true, nil
}

func runHelperSidecarScript(ctx context.Context, root, artifactsDir string, req editRequestRecord, helper *editHelperScript) (string, string, error) {
	sidecarDir := filepath.Join(artifactsDir, "sidecar")
	if err := os.MkdirAll(sidecarDir, 0o755); err != nil {
		return "", "", err
	}
	helperPath := filepath.Join(sidecarDir, "helper.sh")
	content := strings.TrimSpace(helper.Content)
	if !strings.HasPrefix(content, "#!") {
		content = "#!/usr/bin/env bash\nset -euo pipefail\n\n" + content
	}
	if err := os.WriteFile(helperPath, []byte(content+"\n"), 0o755); err != nil {
		return "", "", err
	}
	runCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(runCtx, "bash", helperPath)
	cmd.Dir = root
	cmd.Env = append(os.Environ(),
		"DORKPIPE_ROOT="+root,
		"DORKPIPE_REQUEST="+req.UserMessage,
		"DORKPIPE_ACTIVE_FILE="+req.ActiveFile,
		"DORKPIPE_SELECTION="+req.SelectionText,
	)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return "", helperPath, errors.New(msg)
	}
	_ = os.WriteFile(filepath.Join(sidecarDir, "helper.stdout.txt"), stdout.Bytes(), 0o644)
	_ = os.WriteFile(filepath.Join(sidecarDir, "helper.stderr.txt"), stderr.Bytes(), 0o644)
	return stdout.String(), helperPath, nil
}

func targetFilesFromPatch(patch string) []string {
	var out []string
	for _, line := range strings.Split(strings.ReplaceAll(patch, "\r\n", "\n"), "\n") {
		if strings.HasPrefix(line, "+++ b/") {
			out = append(out, strings.TrimPrefix(line, "+++ b/"))
		}
	}
	return uniqueNonEmpty(out)
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

func parseHelperSidecarResponse(text string) (*editHelperResponse, error) {
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
	var resp editHelperResponse
	if err := json.Unmarshal([]byte(clean), &resp); err != nil {
		return nil, err
	}
	if err := validateHelperSidecarResponse(&resp); err != nil {
		return nil, err
	}
	return &resp, nil
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

func validateHelperSidecarResponse(resp *editHelperResponse) error {
	if resp == nil {
		return errors.New("helper response is empty")
	}
	if strings.TrimSpace(resp.Summary) == "" {
		return errors.New("helper summary is empty")
	}
	if resp.HelperScript == nil {
		return errors.New("helper_script is missing")
	}
	runtime := strings.ToLower(strings.TrimSpace(resp.HelperScript.Runtime))
	if runtime != "bash" && runtime != "sh" {
		return fmt.Errorf("unsupported helper runtime %q", resp.HelperScript.Runtime)
	}
	if strings.TrimSpace(resp.HelperScript.Content) == "" {
		return errors.New("helper script content is empty")
	}
	if len(resp.HelperScript.Content) > 12000 {
		return errors.New("helper script content is too large")
	}
	lower := strings.ToLower(resp.HelperScript.Content)
	for _, banned := range []string{
		"curl ", "wget ", "ssh ", "scp ", "rsync ", "nc ", "telnet ", "sudo ", "doas ",
		"rm ", " rm\n", "mv ", "chmod ", "chown ", "tee ", "git apply", "git checkout", "git reset", "git clean",
		"npm ", "pnpm ", "yarn ", "bun ", "pip ", "go build", "go test", "cargo ", "docker ", "podman ",
		">", ">>", "touch ", "mkdir ", "cp ", "install ",
	} {
		if strings.Contains(lower, banned) {
			return fmt.Errorf("helper script contains blocked token %q", strings.TrimSpace(banned))
		}
	}
	return nil
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
	if !(strings.Contains(artifact.Patch, "\n--- a/") || strings.Contains(artifact.Patch, "\n--- /dev/null")) || !strings.Contains(artifact.Patch, "\n+++ b/") {
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
	if artifact.HelperScript != nil {
		base += fmt.Sprintf("\n\nBounded helper script: `%s` (%s)", artifact.HelperScript.Runtime, emptyFallback(artifact.HelperScript.Purpose, "generated patch helper"))
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
	if artifact, patchPath, ok, err := tryDeterministicFileCreatePrimitive(reqID, root, message, artifactsDir); ok {
		return artifact, patchPath, true, err
	}
	if artifact, patchPath, ok, err := tryDeterministicAnchorInsertPrimitive(reqID, root, message, activeFile, artifactsDir); ok {
		return artifact, patchPath, true, err
	}
	if artifact, patchPath, ok, err := tryDeterministicCollectionScaffoldPrimitive(reqID, root, message, artifactsDir); ok {
		return artifact, patchPath, true, err
	}
	if artifact, patchPath, ok, err := tryDeterministicYAMLScalarUpdatePrimitive(reqID, root, message, activeFile, artifactsDir); ok {
		return artifact, patchPath, true, err
	}
	if artifact, patchPath, ok, err := tryDeterministicYAMLListAddPrimitive(reqID, root, message, activeFile, artifactsDir); ok {
		return artifact, patchPath, true, err
	}
	if artifact, patchPath, ok, err := tryDeterministicMarkdownSectionPrimitive(reqID, root, message, activeFile, artifactsDir); ok {
		return artifact, patchPath, true, err
	}
	targetFile, lineText, ok := deterministicReadmeAppend(root, message, activeFile)
	if !ok {
		return nil, "", false, nil
	}
	emitEditEvent(reqID, "context_gathering", "Using deterministic text append primitive", 0.18, map[string]any{
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
		Validations: []string{"Verify the target file now includes the requested text."},
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
	_ = os.WriteFile(filepath.Join(artifactsDir, "verify-patch.log"), []byte("Deterministic text append primitive selected."), 0o644)
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

func tryDeterministicFileCreatePrimitive(reqID, root, message, artifactsDir string) (*editModelArtifact, string, bool, error) {
	targetFile, content, ok := inferFileCreateSpec(message)
	if !ok {
		return nil, "", false, nil
	}
	if strings.HasPrefix(targetFile, "/") || strings.Contains(targetFile, "..") {
		return nil, "", false, nil
	}
	if _, err := os.Stat(filepath.Join(root, targetFile)); err == nil {
		return nil, "", false, nil
	}
	emitEditEvent(reqID, "context_gathering", "Using deterministic file creation primitive", 0.18, map[string]any{
		"target_file": targetFile,
	})
	artifact := &editModelArtifact{
		Summary:     fmt.Sprintf("Create `%s` using a local file-creation primitive.", targetFile),
		TargetFiles: []string{targetFile},
		Patch:       buildCreateFilePatch(targetFile, content),
		Validations: []string{"Verify the new file content matches the requested intent."},
	}
	if err := validateEditArtifact(artifact); err != nil {
		return nil, "", true, err
	}
	return writePreparedDeterministicArtifact(artifactsDir, artifact, "Deterministic file creation primitive selected.")
}

func tryDeterministicAnchorInsertPrimitive(reqID, root, message, activeFile, artifactsDir string) (*editModelArtifact, string, bool, error) {
	targetFile := resolveDeterministicTargetFile(root, activeFile, message, []string{".go", ".js", ".ts", ".tsx", ".jsx", ".py", ".md", ".yml", ".yaml", ".json", ".txt"})
	if targetFile == "" {
		return nil, "", false, nil
	}
	anchor, block, ok := inferAnchorInsertSpec(message)
	if !ok {
		return nil, "", false, nil
	}
	beforeBytes, err := os.ReadFile(filepath.Join(root, targetFile))
	if err != nil {
		return nil, "", false, err
	}
	before := string(beforeBytes)
	after, changed := insertBlockAfterAnchor(before, anchor, block)
	if !changed {
		return nil, "", false, nil
	}
	emitEditEvent(reqID, "context_gathering", "Using deterministic anchor insert primitive", 0.18, map[string]any{
		"target_file": targetFile,
		"anchor":      anchor,
	})
	artifact := &editModelArtifact{
		Summary:     fmt.Sprintf("Insert a requested block into `%s` after `%s` using a local anchor primitive.", targetFile, anchor),
		TargetFiles: []string{targetFile},
		Patch:       buildReplaceFilePatch(targetFile, before, after),
		Validations: []string{"Verify the inserted block landed at the intended anchor."},
	}
	if err := validateEditArtifact(artifact); err != nil {
		return nil, "", true, err
	}
	return writePreparedDeterministicArtifact(artifactsDir, artifact, "Deterministic anchor insert primitive selected.")
}

func tryDeterministicYAMLScalarUpdatePrimitive(reqID, root, message, activeFile, artifactsDir string) (*editModelArtifact, string, bool, error) {
	targetFile := resolveDeterministicTargetFile(root, activeFile, message, []string{".yml", ".yaml"})
	if targetFile == "" {
		return nil, "", false, nil
	}
	key, value, ok := inferYAMLScalarUpdate(message)
	if !ok {
		return nil, "", false, nil
	}
	beforeBytes, err := os.ReadFile(filepath.Join(root, targetFile))
	if err != nil {
		return nil, "", false, err
	}
	before := string(beforeBytes)
	after, changed := updateYAMLScalarField(before, key, value)
	if !changed {
		return nil, "", false, nil
	}
	emitEditEvent(reqID, "context_gathering", "Using deterministic YAML field update primitive", 0.18, map[string]any{
		"target_file": targetFile,
		"yaml_key":    key,
	})
	artifact := &editModelArtifact{
		Summary:     fmt.Sprintf("Update `%s` in `%s` using a local YAML primitive.", key, targetFile),
		TargetFiles: []string{targetFile},
		Patch:       buildReplaceFilePatch(targetFile, before, after),
		Validations: []string{fmt.Sprintf("Verify `%s` now reflects the requested value.", key)},
	}
	if err := validateEditArtifact(artifact); err != nil {
		return nil, "", true, err
	}
	return writePreparedDeterministicArtifact(artifactsDir, artifact, "Deterministic YAML field update primitive selected.")
}

func tryDeterministicYAMLListAddPrimitive(reqID, root, message, activeFile, artifactsDir string) (*editModelArtifact, string, bool, error) {
	targetFile := resolveDeterministicTargetFile(root, activeFile, message, []string{".yml", ".yaml"})
	if targetFile == "" {
		return nil, "", false, nil
	}
	key, value, ok := inferYAMLListAppend(message)
	if !ok {
		return nil, "", false, nil
	}
	beforeBytes, err := os.ReadFile(filepath.Join(root, targetFile))
	if err != nil {
		return nil, "", false, err
	}
	before := string(beforeBytes)
	after, changed := appendYAMLListItem(before, key, value)
	if !changed {
		return nil, "", false, nil
	}
	emitEditEvent(reqID, "context_gathering", "Using deterministic YAML list primitive", 0.18, map[string]any{
		"target_file": targetFile,
		"yaml_key":    key,
	})
	artifact := &editModelArtifact{
		Summary:     fmt.Sprintf("Add `%s` to `%s` in `%s` using a local YAML primitive.", value, key, targetFile),
		TargetFiles: []string{targetFile},
		Patch:       buildReplaceFilePatch(targetFile, before, after),
		Validations: []string{fmt.Sprintf("Verify `%s` now includes `%s`.", key, value)},
	}
	if err := validateEditArtifact(artifact); err != nil {
		return nil, "", true, err
	}
	return writePreparedDeterministicArtifact(artifactsDir, artifact, "Deterministic YAML list primitive selected.")
}

func tryDeterministicMarkdownSectionPrimitive(reqID, root, message, activeFile, artifactsDir string) (*editModelArtifact, string, bool, error) {
	targetFile := resolveDeterministicTargetFile(root, activeFile, message, []string{".md"})
	if targetFile == "" {
		return nil, "", false, nil
	}
	title, body, ok := inferMarkdownSection(message)
	if !ok {
		return nil, "", false, nil
	}
	beforeBytes, err := os.ReadFile(filepath.Join(root, targetFile))
	if err != nil {
		return nil, "", false, err
	}
	before := string(beforeBytes)
	after, changed := appendMarkdownSection(before, title, body)
	if !changed {
		return nil, "", false, nil
	}
	emitEditEvent(reqID, "context_gathering", "Using deterministic markdown section primitive", 0.18, map[string]any{
		"target_file": targetFile,
		"title":       title,
	})
	artifact := &editModelArtifact{
		Summary:     fmt.Sprintf("Add a `%s` section to `%s` using a local markdown primitive.", title, targetFile),
		TargetFiles: []string{targetFile},
		Patch:       buildReplaceFilePatch(targetFile, before, after),
		Validations: []string{"Verify the new markdown section reads naturally in context."},
	}
	if err := validateEditArtifact(artifact); err != nil {
		return nil, "", true, err
	}
	return writePreparedDeterministicArtifact(artifactsDir, artifact, "Deterministic markdown section primitive selected.")
}

func tryDeterministicCollectionScaffoldPrimitive(reqID, root, message, artifactsDir string) (*editModelArtifact, string, bool, error) {
	spec, ok := inferPackageScaffoldSpec(root, message)
	if !ok {
		return nil, "", false, nil
	}
	emitEditEvent(reqID, "context_gathering", "Using deterministic scaffold primitive", 0.18, map[string]any{
		"package_root": spec.PackageRoot,
		"package_name": spec.PackageName,
	})
	if _, err := os.Stat(filepath.Join(root, spec.ManifestPath)); err == nil {
		return nil, "", false, nil
	}
	var patchParts []string
	patchParts = append(patchParts, buildCreateFilePatch(spec.ManifestPath, spec.ManifestBody))
	patchParts = append(patchParts, buildCreateFilePatch(spec.ReadmePath, spec.ReadmeBody))
	artifact := &editModelArtifact{
		Summary:     spec.Summary,
		TargetFiles: spec.TargetFiles,
		Patch:       strings.Join(patchParts, "\n"),
		Validations: []string{spec.ValidationMsg},
	}
	if err := validateEditArtifact(artifact); err != nil {
		emitEditError(reqID, "MODEL_OUTPUT_INVALID", fmt.Sprintf("Deterministic scaffold validation failed: %v", err), false)
		return nil, "", true, err
	}
	writeJSON(filepath.Join(artifactsDir, "artifact.json"), artifact)
	patchPath := filepath.Join(artifactsDir, "patch.diff")
	if err := os.WriteFile(patchPath, []byte(artifact.Patch), 0o644); err != nil {
		return nil, "", true, err
	}
	_ = os.WriteFile(filepath.Join(artifactsDir, "verify-patch.log"), []byte("Deterministic scaffold primitive selected."), 0o644)
	return artifact, patchPath, true, nil
}

func writePreparedDeterministicArtifact(artifactsDir string, artifact *editModelArtifact, verifyText string) (*editModelArtifact, string, bool, error) {
	writeJSON(filepath.Join(artifactsDir, "artifact.json"), artifact)
	patchPath := filepath.Join(artifactsDir, "patch.diff")
	if err := os.WriteFile(patchPath, []byte(artifact.Patch), 0o644); err != nil {
		return nil, "", true, err
	}
	_ = os.WriteFile(filepath.Join(artifactsDir, "verify-patch.log"), []byte(verifyText), 0o644)
	return artifact, patchPath, true, nil
}

func inferPackageScaffoldSpec(root, message string) (*packageScaffoldSpec, bool) {
	lower := strings.ToLower(strings.TrimSpace(message))
	if !looksLikePackageCreation(lower) {
		return nil, false
	}
	packageRoots := discoverPackageRoots(root)
	if len(packageRoots) == 0 {
		return nil, false
	}
	packageRoot := selectPackageRootForMessage(lower, packageRoots)
	if packageRoot == "" {
		return nil, false
	}
	packageName := inferRequestedPackageName(lower)
	if packageName == "" {
		packageName = pickAvailablePackageName(root, packageRoot)
	}
	if packageName == "" {
		return nil, false
	}
	packageDir := filepath.ToSlash(filepath.Join(packageRoot, packageName))
	manifestPath := filepath.ToSlash(filepath.Join(packageDir, "package.yml"))
	readmePath := filepath.ToSlash(filepath.Join(packageDir, "README.md"))
	version := strings.TrimSpace(readRepoVersion(root))
	if version == "" {
		version = "0.1.0"
	}
	titleName := humanizePackageName(packageName)
	manifestBody := strings.TrimSpace(fmt.Sprintf(`schema: 1
kind: package
name: %s
version: %s
title: %s
description: |
  A generated package scaffold created through a primitive-first edit lane.
  This package is intended as a lightweight authoring starting point that can grow into real
  workflows, resolvers, or assets.
author: Generated
license: Apache-2.0
repository: https://github.com/dockpipe/dockpipe
tags: [experimental, authoring, generated]
`, packageName, version, titleName)) + "\n"
	readmeBody := strings.TrimSpace(fmt.Sprintf(`# %s

This package was scaffolded as a fast local authoring starting point.

Ideas to extend it:
- add a workflow under a nested package/resolver tree
- add assets or scripts beside the package manifest
- tighten the title, description, and tags once the package direction is clear
`, titleName)) + "\n"
	return &packageScaffoldSpec{
		PackageRoot:   packageRoot,
		PackageName:   packageName,
		PackageDir:    packageDir,
		ManifestPath:  manifestPath,
		ReadmePath:    readmePath,
		ManifestBody:  manifestBody,
		ReadmeBody:    readmeBody,
		TargetFiles:   []string{manifestPath, readmePath},
		Summary:       fmt.Sprintf("Scaffold a new `%s` collection item under `%s` using a local scaffold primitive.", packageName, packageRoot),
		ValidationMsg: "Verify the new package manifest and README reflect the intended package direction.",
	}, true
}

func looksLikePackageCreation(lower string) bool {
	if !strings.Contains(lower, "package") {
		return false
	}
	for _, verb := range []string{"make", "create", "new", "scaffold", "author", "build"} {
		if strings.Contains(lower, verb) {
			return true
		}
	}
	return strings.Contains(lower, "of your choosing")
}

func discoverPackageRoots(root string) []string {
	skipDirs := map[string]struct{}{
		".git":      {},
		"node_modules": {},
		"target":    {},
		"bin":       {},
		".dockpipe": {},
		".dorkpipe": {},
	}
	var roots []string
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if _, skip := skipDirs[d.Name()]; skip && path != root {
				return filepath.SkipDir
			}
			return nil
		}
		if d.Name() != "package.yml" {
			return nil
		}
		rel := filepath.ToSlash(relativeTo(root, filepath.Dir(filepath.Dir(path))))
		if rel == "." || rel == "" {
			return nil
		}
		if rel == "packages" || strings.HasPrefix(rel, "packages/") || rel == ".staging/packages" || strings.HasPrefix(rel, ".staging/packages/") {
			roots = append(roots, rel)
		}
		return nil
	})
	return uniqueNonEmpty(roots)
}

func selectPackageRootForMessage(lower string, roots []string) string {
	sort.SliceStable(roots, func(i, j int) bool {
		return len(roots[i]) > len(roots[j])
	})
	for _, root := range roots {
		if strings.Contains(lower, strings.ToLower(root)) {
			return root
		}
		base := pathBase(root)
		if base != "" && strings.Contains(lower, base) {
			return root
		}
	}
	for _, root := range roots {
		if strings.Contains(root, "/") {
			return root
		}
	}
	return roots[0]
}

func inferRequestedPackageName(lower string) string {
	for _, prefix := range []string{"package called ", "package named ", "package name ", "called ", "named "} {
		if idx := strings.Index(lower, prefix); idx >= 0 {
			tail := strings.TrimSpace(lower[idx+len(prefix):])
			name := normalizePackageNameToken(firstToken(tail))
			if name != "" && name != "package" {
				return name
			}
		}
	}
	return ""
}

func pickAvailablePackageName(root, packageRoot string) string {
	for _, candidate := range []string{"playground", "spark", "arcade", "lab", "pixel"} {
		manifestPath := filepath.Join(root, packageRoot, candidate, "package.yml")
		if _, err := os.Stat(manifestPath); errors.Is(err, os.ErrNotExist) {
			return candidate
		}
	}
	return fmt.Sprintf("package-%d", time.Now().Unix()%10000)
}

func normalizePackageNameToken(token string) string {
	token = strings.TrimSpace(token)
	token = strings.Trim(token, ".,:;!?()[]{}\"'`")
	token = strings.ToLower(token)
	token = strings.ReplaceAll(token, " ", "-")
	var b strings.Builder
	for _, ch := range token {
		if (ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') || ch == '-' {
			b.WriteRune(ch)
		}
	}
	return strings.Trim(b.String(), "-")
}

func firstToken(text string) string {
	fields := strings.Fields(text)
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}

func readRepoVersion(root string) string {
	b, err := os.ReadFile(filepath.Join(root, "VERSION"))
	if err != nil {
		return ""
	}
	return string(bytes.TrimSpace(b))
}

func humanizePackageName(name string) string {
	parts := strings.Fields(strings.ReplaceAll(name, "-", " "))
	for i, part := range parts {
		if part == "" {
			continue
		}
		parts[i] = strings.ToUpper(part[:1]) + part[1:]
	}
	return strings.Join(parts, " ")
}

func buildCreateFilePatch(targetFile, content string) string {
	lines := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	var b strings.Builder
	fmt.Fprintf(&b, "diff --git a/%s b/%s\n", targetFile, targetFile)
	b.WriteString("new file mode 100644\n")
	b.WriteString("index 0000000..1111111\n")
	fmt.Fprintf(&b, "--- /dev/null\n")
	fmt.Fprintf(&b, "+++ b/%s\n", targetFile)
	fmt.Fprintf(&b, "@@ -0,0 +1,%d @@\n", len(lines))
	for _, line := range lines {
		b.WriteString("+")
		b.WriteString(line)
		b.WriteString("\n")
	}
	return b.String()
}

func buildReplaceFilePatch(targetFile, before, after string) string {
	beforeLines := strings.Split(strings.ReplaceAll(before, "\r\n", "\n"), "\n")
	afterLines := strings.Split(strings.ReplaceAll(after, "\r\n", "\n"), "\n")
	if len(beforeLines) > 0 && beforeLines[len(beforeLines)-1] == "" {
		beforeLines = beforeLines[:len(beforeLines)-1]
	}
	if len(afterLines) > 0 && afterLines[len(afterLines)-1] == "" {
		afterLines = afterLines[:len(afterLines)-1]
	}
	var b strings.Builder
	fmt.Fprintf(&b, "diff --git a/%s b/%s\n", targetFile, targetFile)
	fmt.Fprintf(&b, "--- a/%s\n", targetFile)
	fmt.Fprintf(&b, "+++ b/%s\n", targetFile)
	fmt.Fprintf(&b, "@@ -1,%d +1,%d @@\n", len(beforeLines), len(afterLines))
	for _, line := range beforeLines {
		b.WriteString("-")
		b.WriteString(line)
		b.WriteString("\n")
	}
	for _, line := range afterLines {
		b.WriteString("+")
		b.WriteString(line)
		b.WriteString("\n")
	}
	return b.String()
}

func pathBase(value string) string {
	value = strings.TrimSuffix(filepath.ToSlash(value), "/")
	if idx := strings.LastIndex(value, "/"); idx >= 0 {
		return value[idx+1:]
	}
	return value
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

func resolveDeterministicTargetFile(root, activeFile, message string, exts []string) string {
	candidates := []string{}
	if strings.TrimSpace(activeFile) != "" {
		candidates = append(candidates, strings.TrimSpace(activeFile))
	}
	for _, token := range strings.Fields(message) {
		token = strings.Trim(token, ".,:;!?()[]{}\"'`")
		if token == "" {
			continue
		}
		for _, ext := range exts {
			if strings.HasSuffix(strings.ToLower(token), ext) {
				candidates = append(candidates, token)
				break
			}
		}
	}
	if containsExt(message, ".md") && !strings.EqualFold(activeFile, "") {
		candidates = append(candidates, "README.md")
	}
	for _, candidate := range uniqueNonEmpty(candidates) {
		lower := strings.ToLower(candidate)
		matched := false
		for _, ext := range exts {
			if strings.HasSuffix(lower, ext) {
				matched = true
				break
			}
		}
		if !matched {
			continue
		}
		if strings.HasPrefix(candidate, "/") || strings.Contains(candidate, "..") {
			continue
		}
		if _, err := os.Stat(filepath.Join(root, candidate)); err == nil {
			return filepath.ToSlash(candidate)
		}
	}
	if len(exts) == 1 && exts[0] == ".md" && strings.Contains(strings.ToLower(message), "readme") {
		if _, err := os.Stat(filepath.Join(root, "README.md")); err == nil {
			return "README.md"
		}
	}
	return ""
}

func inferYAMLScalarUpdate(message string) (string, string, bool) {
	lower := strings.ToLower(message)
	for _, key := range []string{"title", "description", "version", "name", "license", "author", "repository"} {
		for _, marker := range []string{
			"set " + key + " to ",
			"change " + key + " to ",
			"update " + key + " to ",
			"make " + key + " ",
		} {
			if idx := strings.Index(lower, marker); idx >= 0 {
				raw := strings.TrimSpace(message[idx+len(marker):])
				value := cleanPrimitiveValue(raw)
				if value != "" {
					return key, value, true
				}
			}
		}
	}
	return "", "", false
}

func updateYAMLScalarField(before, key, value string) (string, bool) {
	lines := strings.Split(strings.ReplaceAll(before, "\r\n", "\n"), "\n")
	replaced := false
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), key+":") {
			indent := line[:len(line)-len(strings.TrimLeft(line, " "))]
			lines[i] = fmt.Sprintf("%s%s: %s", indent, key, quoteYAMLValue(value))
			replaced = true
			break
		}
	}
	if !replaced {
		return before, false
	}
	after := strings.Join(lines, "\n")
	if after == before {
		return before, false
	}
	return after, true
}

func inferYAMLListAppend(message string) (string, string, bool) {
	lower := strings.ToLower(message)
	type pair struct {
		key string
		phrases []string
	}
	pairs := []pair{
		{key: "tags", phrases: []string{"add tag ", "add the tag "}},
		{key: "includes_resolvers", phrases: []string{"add resolver ", "include resolver "}},
		{key: "depends", phrases: []string{"add dependency ", "depend on ", "add depends "}},
	}
	for _, item := range pairs {
		for _, phrase := range item.phrases {
			if idx := strings.Index(lower, phrase); idx >= 0 {
				raw := strings.TrimSpace(message[idx+len(phrase):])
				value := normalizePackageNameToken(firstToken(raw))
				if value != "" {
					return item.key, value, true
				}
			}
		}
	}
	return "", "", false
}

func inferFileCreateSpec(message string) (string, string, bool) {
	lower := strings.ToLower(message)
	if !(strings.Contains(lower, "create file") || strings.Contains(lower, "new file") || strings.Contains(lower, "make file")) {
		return "", "", false
	}
	target := ""
	targetLower := ""
	for _, marker := range []string{"create file ", "new file ", "make file ", "create a file ", "make a file "} {
		if idx := strings.Index(lower, marker); idx >= 0 {
			target = strings.TrimSpace(message[idx+len(marker):])
			targetLower = strings.TrimSpace(lower[idx+len(marker):])
			break
		}
	}
	if target == "" {
		target = extractQuotedText(message)
		targetLower = strings.ToLower(target)
	}
	for _, stop := range []string{" with content ", " saying ", " containing "} {
		if idx := strings.Index(targetLower, stop); idx >= 0 {
			target = strings.TrimSpace(target[:idx])
			break
		}
	}
	target = strings.Trim(strings.TrimSpace(target), "`\"'")
	target = filepath.ToSlash(target)
	if target == "" {
		return "", "", false
	}
	content := ""
	for _, marker := range []string{" with content ", " saying ", " containing "} {
		if idx := strings.Index(lower, marker); idx >= 0 {
			content = strings.TrimSpace(message[idx+len(marker):])
			break
		}
	}
	if content == "" {
		content = "TODO: fill this in.\n"
	} else {
		content = strings.TrimSpace(strings.Trim(content, "`\"'")) + "\n"
	}
	return target, content, true
}

func inferAnchorInsertSpec(message string) (string, string, bool) {
	lower := strings.ToLower(message)
	if !(strings.Contains(lower, "after ") && (strings.Contains(lower, "insert ") || strings.Contains(lower, "add "))) {
		return "", "", false
	}
	anchor := ""
	anchorLower := ""
	for _, marker := range []string{"after ", "below "} {
		if idx := strings.Index(lower, marker); idx >= 0 {
			anchor = strings.TrimSpace(message[idx+len(marker):])
			anchorLower = strings.TrimSpace(lower[idx+len(marker):])
			for _, stop := range []string{" insert ", " add ", " saying ", " containing "} {
				if cut := strings.Index(anchorLower, stop); cut >= 0 {
					anchor = strings.TrimSpace(anchor[:cut])
					break
				}
			}
			break
		}
	}
	anchor = strings.Trim(anchor, "`\"'")
	if anchor == "" {
		return "", "", false
	}
	block := ""
	for _, marker := range []string{" insert ", " add ", " saying ", " containing "} {
		if idx := strings.Index(lower, marker); idx >= 0 {
			block = strings.TrimSpace(message[idx+len(marker):])
			break
		}
	}
	if block == "" {
		block = extractQuotedText(message)
	}
	block = strings.TrimSpace(strings.Trim(block, "`\"'"))
	if block == "" {
		return "", "", false
	}
	return anchor, block, true
}

func appendYAMLListItem(before, key, value string) (string, bool) {
	lines := strings.Split(strings.ReplaceAll(before, "\r\n", "\n"), "\n")
	for _, line := range lines {
		if strings.TrimSpace(line) == "- "+value {
			return before, false
		}
	}
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, key+": [") && strings.HasSuffix(trimmed, "]") {
			start := strings.Index(trimmed, "[")
			end := strings.LastIndex(trimmed, "]")
			if start >= 0 && end > start {
				items := strings.Split(trimmed[start+1:end], ",")
				var cleaned []string
				exists := false
				for _, item := range items {
					part := strings.TrimSpace(item)
					if part == "" {
						continue
					}
					if part == value {
						exists = true
					}
					cleaned = append(cleaned, part)
				}
				if exists {
					return before, false
				}
				cleaned = append(cleaned, value)
				prefix := line[:len(line)-len(strings.TrimLeft(line, " "))]
				lines[i] = fmt.Sprintf("%s%s: [%s]", prefix, key, strings.Join(cleaned, ", "))
				return strings.Join(lines, "\n"), true
			}
		}
		if trimmed == key+":" {
			insertAt := i + 1
			for insertAt < len(lines) {
				next := lines[insertAt]
				if strings.TrimSpace(next) == "" {
					insertAt++
					continue
				}
				if !strings.HasPrefix(next, "  - ") {
					break
				}
				insertAt++
			}
			newLine := "  - " + value
			lines = append(lines[:insertAt], append([]string{newLine}, lines[insertAt:]...)...)
			return strings.Join(lines, "\n"), true
		}
	}
	return before, false
}

func inferMarkdownSection(message string) (string, string, bool) {
	lower := strings.ToLower(message)
	if !strings.Contains(lower, "section") {
		return "", "", false
	}
	title := ""
	for _, marker := range []string{"section called ", "section named ", "section titled "} {
		if idx := strings.Index(lower, marker); idx >= 0 {
			title = strings.TrimSpace(message[idx+len(marker):])
			for _, stop := range []string{" saying ", " with text ", " with body "} {
				if cut := strings.Index(strings.ToLower(title), stop); cut >= 0 {
					title = strings.TrimSpace(title[:cut])
					break
				}
			}
			break
		}
	}
	if title == "" {
		title = extractQuotedText(message)
	}
	title = cleanPrimitiveValue(title)
	if title == "" {
		return "", "", false
	}
	body := "Add details here."
	for _, marker := range []string{"saying ", "with text ", "with body "} {
		if idx := strings.Index(lower, marker); idx >= 0 {
			body = cleanPrimitiveValue(message[idx+len(marker):])
			break
		}
	}
	return title, body, true
}

func appendMarkdownSection(before, title, body string) (string, bool) {
	heading := "## " + title
	if strings.Contains(before, heading) {
		return before, false
	}
	section := strings.TrimSpace(heading + "\n\n" + body) + "\n"
	trimmed := strings.TrimRight(before, "\n")
	if trimmed == "" {
		return section, true
	}
	return trimmed + "\n\n" + section, true
}

func insertBlockAfterAnchor(before, anchor, block string) (string, bool) {
	normalized := strings.ReplaceAll(before, "\r\n", "\n")
	idx := strings.Index(normalized, anchor)
	if idx < 0 {
		return before, false
	}
	insertPos := idx + len(anchor)
	if insertPos < len(normalized) && normalized[insertPos] == '\n' {
		insertPos++
	}
	insertion := block
	if !strings.HasSuffix(insertion, "\n") {
		insertion += "\n"
	}
	after := normalized[:insertPos] + insertion + normalized[insertPos:]
	if after == before {
		return before, false
	}
	return after, true
}

func cleanPrimitiveValue(raw string) string {
	value := strings.TrimSpace(raw)
	value = strings.Trim(value, "`\"' ")
	for _, suffix := range []string{" in ", " for ", " on ", " to "} {
		if idx := strings.Index(strings.ToLower(value), suffix); idx > 0 {
			value = strings.TrimSpace(value[:idx])
		}
	}
	return strings.TrimSpace(value)
}

func containsExt(text, ext string) bool {
	return strings.Contains(strings.ToLower(text), ext)
}

func quoteYAMLValue(value string) string {
	if value == "" {
		return `""`
	}
	if strings.ContainsAny(value, ":#[]{}") || strings.Contains(value, "  ") || strings.Contains(value, "http://") || strings.Contains(value, "https://") {
		return `"` + strings.ReplaceAll(value, `"`, `\"`) + `"`
	}
	return value
}
