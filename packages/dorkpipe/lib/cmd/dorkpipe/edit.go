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
	"strings"
	"time"
)

const (
	defaultEditModel      = "llama3.2"
	maxEditContextChars   = 18000
	maxEditSelectionChars = 2400
	maxEditSnippetPerFile = 5000
	maxEditCandidateCount = 6
	editContractVersion   = "v1"
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

func editCmd(argv []string) {
	fs := flag.NewFlagSet("edit", flag.ExitOnError)
	workdir := fs.String("workdir", "", "working directory (default cwd)")
	message := fs.String("message", "", "edit request text")
	activeFile := fs.String("active-file", "", "repo-relative active file hint")
	selectionText := fs.String("selection-text", "", "active selection hint")
	apply := fs.Bool("apply", false, "apply the verified patch to the working tree")
	model := fs.String("model", "", "Ollama model override")
	ollamaHost := fs.String("ollama-host", "", "Ollama host override")
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

	reqID := fmt.Sprintf("req_%d", time.Now().UnixNano())
	artifactsDir := filepath.Join(absWd, ".dorkpipe", "edit", reqID)
	if err := os.MkdirAll(artifactsDir, 0o755); err != nil {
		emitEditError(reqID, "INTERNAL_ERROR", fmt.Sprintf("Could not create artifact directory: %v", err), false)
		os.Exit(1)
	}

	ctx := context.Background()
	emitEditEvent(reqID, "received", "Received edit request", 0.05, nil)

	artifact, patchPath, artifactsDir, err := prepareEditArtifact(ctx, reqID, absWd, strings.TrimSpace(*message), strings.TrimSpace(*activeFile), strings.TrimSpace(*selectionText), host, chosenModel, artifactsDir)
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

func prepareEditArtifact(ctx context.Context, reqID, root, message, activeFile, selectionText, host, chosenModel, artifactsDir string) (*editModelArtifact, string, string, error) {
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
	snippets := readCandidateSnippets(root, candidates)
	requestRecord := editRequestRecord{
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
	writeJSON(filepath.Join(artifactsDir, "request.json"), requestRecord)

	prompt := buildEditPrompt(requestRecord, contextText, snippets)
	if err := os.WriteFile(filepath.Join(artifactsDir, "prompt.md"), []byte(prompt), 0o644); err != nil {
		emitEditError(reqID, "INTERNAL_ERROR", fmt.Sprintf("Could not write prompt artifact: %v", err), false)
		return nil, "", "", err
	}
	emitEditEvent(reqID, "routed", "Routing to model for patch artifact", 0.32, map[string]any{
		"model": chosenModel,
	})

	modelText, err := runEditModel(ctx, host, chosenModel, prompt)
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
		emitEditError(reqID, "MODEL_OUTPUT_INVALID", fmt.Sprintf("Model output was not a valid edit artifact: %v", err), true)
		return nil, "", "", err
	}
	if err := validateEditArtifact(artifact); err != nil {
		emitEditError(reqID, "MODEL_OUTPUT_INVALID", fmt.Sprintf("Edit artifact validation failed: %v", err), true)
		return nil, "", "", err
	}
	writeJSON(filepath.Join(artifactsDir, "artifact.json"), artifact)

	patchPath := filepath.Join(artifactsDir, "patch.diff")
	if err := os.WriteFile(patchPath, []byte(artifact.Patch), 0o644); err != nil {
		emitEditError(reqID, "INTERNAL_ERROR", fmt.Sprintf("Could not write patch artifact: %v", err), false)
		return nil, "", "", err
	}

	emitEditEvent(reqID, "validating", "Checking patch applicability", 0.55, nil)
	verifyOutput, err := runRepoScript(ctx, root, "packages/dorkpipe/resolvers/dorkpipe/assets/scripts/verify-patch-applies.sh", patchPath, root)
	if err != nil {
		_ = os.WriteFile(filepath.Join(artifactsDir, "verify-patch.log"), []byte(verifyOutput), 0o644)
		emitEditError(reqID, "VALIDATION_FAILED", "The generated patch did not apply cleanly.", true)
		return nil, "", "", err
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

func buildEditPrompt(req editRequestRecord, contextText, snippets string) string {
	applyLine := "Do not assume the patch will be applied automatically."
	if req.Apply {
		applyLine = "The user explicitly requested an applied edit; keep the patch minimal and safe."
	}
	return strings.TrimSpace(fmt.Sprintf(`
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
- %s

User request:
%s

Active file:
%s

Selection:
%s

Context bundle:
%s

Candidate file snippets:
%s
`, applyLine,
		req.UserMessage,
		emptyFallback(req.ActiveFile, "(none)"),
		emptyFallback(req.SelectionText, "(none)"),
		emptyFallback(contextText, "(no context bundle available)"),
		emptyFallback(snippets, "(no candidate file snippets available)")))
}

func runEditModel(ctx context.Context, host, model, prompt string) (string, error) {
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
