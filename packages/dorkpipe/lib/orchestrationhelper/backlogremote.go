package orchestrationhelper

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	pathpkg "path"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"gopkg.in/yaml.v3"
)

const (
	backlogIndexPath         = "docs/agents/task-index.yaml"
	backlogSelectionContract = "dorkpipe.backlog-selection/v1"
	backlogRequestContract   = "dorkpipe.remote-request/v1"
	backlogTaskContract      = "dorkpipe.remote-task/v1"
	backlogFollowupContract  = "dorkpipe.remote-followup/v1"
	backlogFixtureContract   = "dorkpipe.remote-dispatch-fixture/v1"
)

var (
	backlogTaskIDPattern = regexp.MustCompile(`^TASK-[0-9]{3}$`)
	backlogBaseline      = regexp.MustCompile(`^[0-9a-f]{40}$`)
	backlogOpaqueID      = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{7,127}$`)
	backlogReference     = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._/@:-]{0,199}$`)
)

type backlogIndex struct {
	Schema      int                 `yaml:"schema"`
	Description string              `yaml:"description"`
	Tasks       []backlogIndexEntry `yaml:"tasks"`
	Maintenance []string            `yaml:"maintenance"`
}

type backlogIndexEntry struct {
	ID            string `yaml:"id"`
	Topic         string `yaml:"topic"`
	Path          string `yaml:"path"`
	DispatchState string `yaml:"dispatch_state,omitempty"`
}

type backlogDispatchFixture struct {
	ContractVersion string `json:"contract_version"`
	AdapterIdentity string `json:"adapter_identity"`
	RemoteTaskID    string `json:"remote_task_id"`
	SubmittedAt     string `json:"submitted_at"`
}

type backlogRejection struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (r *backlogRejection) Error() string { return r.Code + ": " + r.Message }

func rejectBacklog(code, format string, args ...any) error {
	return &backlogRejection{Code: code, Message: fmt.Sprintf(format, args...)}
}

func inspectBacklogSelection(repoRoot, indexPath, taskID, boundedSlice, baseline, artifactRoot string) error {
	selection := map[string]any{
		"contract_version": backlogSelectionContract,
		"status":           "rejected",
		"task_id":          strings.TrimSpace(taskID),
		"bounded_slice":    strings.TrimSpace(boundedSlice),
		"baseline_commit":  strings.TrimSpace(baseline),
	}
	selectionPath, err := backlogArtifactPath(artifactRoot, "backlog-selection.json")
	if err != nil {
		return err
	}
	reject := func(cause error) error {
		var typed *backlogRejection
		if !errors.As(cause, &typed) {
			typed = &backlogRejection{Code: "inspection_failed", Message: cause.Error()}
		}
		selection["rejection"] = typed
		if writeErr := writeJSONFileAtomic(selectionPath, selection); writeErr != nil {
			return fmt.Errorf("%w; writing rejection artifact: %v", cause, writeErr)
		}
		return cause
	}

	if strings.TrimSpace(indexPath) != backlogIndexPath {
		return reject(rejectBacklog("invalid_index_path", "task index must be %s", backlogIndexPath))
	}
	if err := validateBacklogSlice(boundedSlice); err != nil {
		return reject(err)
	}
	if !backlogBaseline.MatchString(baseline) {
		return reject(rejectBacklog("invalid_baseline", "baseline commit must be exactly 40 lowercase hexadecimal characters"))
	}
	if strings.TrimSpace(taskID) == "" {
		return reject(rejectBacklog("task_id_required", "an explicit task ID is required"))
	}
	if !backlogTaskIDPattern.MatchString(taskID) {
		return reject(rejectBacklog("malformed_task_id", "task ID %q must match TASK-NNN", taskID))
	}

	resolvedIndex, err := resolveBacklogRepoPath(repoRoot, indexPath, true, false)
	if err != nil {
		return reject(rejectBacklog("invalid_index", "%v", err))
	}
	index, err := loadBacklogIndex(resolvedIndex)
	if err != nil {
		return reject(err)
	}
	matches := []backlogIndexEntry{}
	for _, entry := range index.Tasks {
		if entry.ID == taskID {
			matches = append(matches, entry)
		}
	}
	if len(matches) == 0 {
		return reject(rejectBacklog("unknown_task_id", "task ID %q is not present in the open task index", taskID))
	}
	if len(matches) != 1 {
		return reject(rejectBacklog("ambiguous_task_id", "task ID %q has %d index entries", taskID, len(matches)))
	}
	entry := matches[0]
	linkedMatches := 0
	for _, candidate := range index.Tasks {
		if candidate.Path == entry.Path {
			linkedMatches++
		}
	}
	if linkedMatches != 1 {
		return reject(rejectBacklog("ambiguous_linked_task", "linked task path %q has %d index entries", entry.Path, linkedMatches))
	}
	switch entry.DispatchState {
	case "blocked":
		return reject(rejectBacklog("task_blocked", "task ID %q is explicitly marked blocked", taskID))
	case "external_active":
		return reject(rejectBacklog("task_externally_active", "task ID %q is explicitly marked externally active", taskID))
	case "closed":
		return reject(rejectBacklog("task_closed", "task ID %q is explicitly marked closed", taskID))
	}
	if strings.HasPrefix(entry.Path, "docs/agents/tasks/closed/") {
		return reject(rejectBacklog("task_closed", "task ID %q links to the closed task tree", taskID))
	}
	resolvedTask, err := resolveBacklogRepoPath(repoRoot, entry.Path, true, false)
	if err != nil {
		return reject(rejectBacklog("invalid_linked_task", "%v", err))
	}
	taskRaw, err := os.ReadFile(resolvedTask)
	if err != nil {
		return reject(rejectBacklog("invalid_linked_task", "linked task %q cannot be read: %v", entry.Path, err))
	}
	if !linkedTaskHeadingMatches(taskRaw, taskID) {
		return reject(rejectBacklog("mismatched_linked_task", "linked task %q does not begin with the selected task ID", entry.Path))
	}
	indexRaw, err := os.ReadFile(resolvedIndex)
	if err != nil {
		return reject(rejectBacklog("invalid_index", "task index cannot be read: %v", err))
	}
	selection = map[string]any{
		"contract_version": backlogSelectionContract,
		"status":           "selected",
		"task_id":          entry.ID,
		"topic":            entry.Topic,
		"linked_task_path": entry.Path,
		"bounded_slice":    boundedSlice,
		"baseline_commit":  baseline,
		"task_index_path":  backlogIndexPath,
		"dispatch_state":   "open",
		"source_digests": map[string]any{
			"task_index":  sha256String(indexRaw),
			"linked_task": sha256String(taskRaw),
		},
	}
	return writeJSONFileAtomic(selectionPath, selection)
}

func loadBacklogIndex(path string) (*backlogIndex, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, rejectBacklog("invalid_index", "task index cannot be read: %v", err)
	}
	decoder := yaml.NewDecoder(bytes.NewReader(raw))
	decoder.KnownFields(true)
	index := &backlogIndex{}
	if err := decoder.Decode(index); err != nil {
		return nil, rejectBacklog("malformed_index", "task index is not strict YAML: %v", err)
	}
	extra := &backlogIndex{}
	if err := decoder.Decode(extra); err != io.EOF {
		return nil, rejectBacklog("malformed_index", "task index must contain exactly one document")
	}
	if index.Schema != 1 || strings.TrimSpace(index.Description) == "" || len(index.Tasks) == 0 {
		return nil, rejectBacklog("malformed_index", "task index requires schema 1, a description, and at least one task")
	}
	for i, entry := range index.Tasks {
		if !backlogTaskIDPattern.MatchString(entry.ID) || strings.TrimSpace(entry.Topic) == "" || strings.TrimSpace(entry.Path) == "" {
			return nil, rejectBacklog("malformed_index_entry", "task index entry %d requires a TASK-NNN id, topic, and path", i+1)
		}
		if entry.ID != strings.TrimSpace(entry.ID) || entry.Topic != strings.TrimSpace(entry.Topic) || entry.Path != strings.TrimSpace(entry.Path) {
			return nil, rejectBacklog("malformed_index_entry", "task index entry %d contains surrounding whitespace", i+1)
		}
		if pathpkg.Clean(strings.ReplaceAll(entry.Path, "\\", "/")) != entry.Path || !strings.HasPrefix(entry.Path, "docs/agents/tasks/") {
			return nil, rejectBacklog("malformed_index_entry", "task index entry %d has a non-canonical linked path", i+1)
		}
		switch entry.DispatchState {
		case "", "open", "blocked", "external_active", "closed":
		default:
			return nil, rejectBacklog("malformed_index_entry", "task index entry %d has unsupported dispatch_state %q", i+1, entry.DispatchState)
		}
	}
	return index, nil
}

func validateBacklogSlice(value string) error {
	if value == "" || value != strings.TrimSpace(value) {
		return rejectBacklog("invalid_bounded_slice", "bounded slice must be an explicit trimmed description")
	}
	if !utf8.ValidString(value) || len(value) < 12 || len(value) > 500 || strings.ContainsAny(value, "\r\n") {
		return rejectBacklog("invalid_bounded_slice", "bounded slice must be one valid UTF-8 line between 12 and 500 bytes")
	}
	for _, r := range value {
		if unicode.IsControl(r) {
			return rejectBacklog("invalid_bounded_slice", "bounded slice contains a control character")
		}
	}
	return nil
}

func linkedTaskHeadingMatches(raw []byte, taskID string) bool {
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		return line == "# "+taskID || strings.HasPrefix(line, "# "+taskID+" ")
	}
	return false
}

func compileBacklogRemoteRequest(repoRoot, artifactRoot, environmentRef, branchRef, allowedJSON, boundariesJSON, validationJSON, sourcesJSON string) error {
	if err := validateBacklogReference("environment", environmentRef); err != nil {
		return err
	}
	if err := validateBacklogReference("branch", branchRef); err != nil {
		return err
	}
	allowed, err := parseBacklogStringList("allowed paths", allowedJSON, true)
	if err != nil {
		return err
	}
	boundaries, err := parseBacklogStringList("hard boundaries", boundariesJSON, true)
	if err != nil {
		return err
	}
	validation, err := parseBacklogStringList("required validation", validationJSON, true)
	if err != nil {
		return err
	}
	routed, err := parseBacklogStringList("routed sources", sourcesJSON, false)
	if err != nil {
		return err
	}
	selectionPath, err := backlogArtifactPath(artifactRoot, "backlog-selection.json")
	if err != nil {
		return err
	}
	selection, err := readStrictJSONMap(selectionPath)
	if err != nil {
		return fmt.Errorf("backlog selection cannot be loaded: %w", err)
	}
	if stringValue(selection["contract_version"]) != backlogSelectionContract || stringValue(selection["status"]) != "selected" {
		return errors.New("backlog selection is not a selected dorkpipe.backlog-selection/v1 artifact")
	}
	for _, rel := range allowed {
		if isForbiddenBacklogPath(rel) || rel == "." {
			return fmt.Errorf("allowed path %q is broad, generated, secret-like, or provider-private", rel)
		}
		if _, err := resolveBacklogRepoPath(repoRoot, rel, false, true); err != nil {
			return fmt.Errorf("allowed path %q is invalid: %w", rel, err)
		}
	}
	linkedPath := stringValue(selection["linked_task_path"])
	mandatory := []string{"AGENTS.md", backlogIndexPath, linkedPath}
	sourcePaths := appendUniqueStrings(mandatory, routed...)
	sourceFiles := make([]any, 0, len(sourcePaths))
	for _, rel := range sourcePaths {
		if isForbiddenBacklogPath(rel) {
			return fmt.Errorf("routed source %q is generated, secret-like, or provider-private", rel)
		}
		resolved, err := resolveBacklogRepoPath(repoRoot, rel, true, false)
		if err != nil {
			return fmt.Errorf("routed source %q is invalid: %w", rel, err)
		}
		raw, err := os.ReadFile(resolved)
		if err != nil {
			return fmt.Errorf("routed source %q cannot be read: %w", rel, err)
		}
		sourceFiles = append(sourceFiles, map[string]any{"path": rel, "sha256": sha256String(raw)})
	}
	digests := mapValue(selection["source_digests"])
	if sourceDigest(sourceFiles, backlogIndexPath) != stringValue(digests["task_index"]) || sourceDigest(sourceFiles, linkedPath) != stringValue(digests["linked_task"]) {
		return errors.New("task index or linked task changed after backlog inspection")
	}
	payload := map[string]any{
		"contract_version": backlogRequestContract,
		"selection": map[string]any{
			"task_id":          stringValue(selection["task_id"]),
			"topic":            stringValue(selection["topic"]),
			"linked_task_path": linkedPath,
			"bounded_slice":    stringValue(selection["bounded_slice"]),
			"baseline_commit":  stringValue(selection["baseline_commit"]),
		},
		"target":              map[string]any{"environment_ref": environmentRef, "branch_ref": branchRef},
		"scope":               map[string]any{"allowed_paths": anyStrings(allowed), "hard_boundaries": anyStrings(boundaries)},
		"required_validation": anyStrings(validation),
		"source_files":        sourceFiles,
		"execution": map[string]any{
			"adapter_mode": "fixture_only", "attempts": 1, "cloud_spend": "disabled",
			"live_provider": false, "status_polling": false, "diff_retrieval": false,
			"apply": false, "commit": false, "push": false, "publication": false,
		},
	}
	markdown := renderBacklogRequestMarkdown(payload)
	fingerprint, err := backlogRequestFingerprint(payload, markdown)
	if err != nil {
		return err
	}
	payload["request_fingerprint"] = fingerprint
	requestJSON, err := backlogArtifactPath(artifactRoot, "remote-request.json")
	if err != nil {
		return err
	}
	requestMarkdown, err := backlogArtifactPath(artifactRoot, "remote-request.md")
	if err != nil {
		return err
	}
	if err := writeJSONFileAtomic(requestJSON, payload); err != nil {
		return err
	}
	return writeTextFileAtomic(requestMarkdown, markdown)
}

func renderBacklogRequestMarkdown(payload map[string]any) string {
	selection := mapValue(payload["selection"])
	target := mapValue(payload["target"])
	scope := mapValue(payload["scope"])
	lines := []string{
		"# Bounded remote backlog request", "",
		"Complete only the explicitly selected backlog slice. Do not widen scope or infer readiness, ownership, or activity from prose.", "",
		"- Task ID: `" + stringValue(selection["task_id"]) + "`",
		"- Linked task: `" + stringValue(selection["linked_task_path"]) + "`",
		"- Bounded slice: " + stringValue(selection["bounded_slice"]),
		"- Baseline commit: `" + stringValue(selection["baseline_commit"]) + "`",
		"- Environment reference: `" + stringValue(target["environment_ref"]) + "`",
		"- Branch reference: `" + stringValue(target["branch_ref"]) + "`", "",
		"## Allowed paths", "",
	}
	for _, item := range stringList(scope["allowed_paths"]) {
		lines = append(lines, "- `"+item+"`")
	}
	lines = append(lines, "", "## Hard boundaries", "")
	for _, item := range stringList(scope["hard_boundaries"]) {
		lines = append(lines, "- "+item)
	}
	lines = append(lines, "", "## Required validation", "")
	for _, item := range stringList(payload["required_validation"]) {
		lines = append(lines, "- `"+item+"`")
	}
	lines = append(lines, "", "## Source of truth", "")
	for _, raw := range listValue(payload["source_files"]) {
		item := mapValue(raw)
		lines = append(lines, "- `"+stringValue(item["path"])+"` ("+stringValue(item["sha256"])+")")
	}
	lines = append(lines, "", "This request permits no live submission, status polling, diff retrieval, apply, commit, push, or publication in the fixture-only slice.", "")
	return strings.Join(lines, "\n")
}

func dispatchBacklogFixture(artifactRoot, fixturePath string) error {
	request, _, err := loadAndVerifyBacklogRequest(artifactRoot)
	if err != nil {
		return err
	}
	fixtureRaw, err := os.ReadFile(fixturePath)
	if err != nil {
		return fmt.Errorf("dispatch fixture cannot be read: %w", err)
	}
	decoder := json.NewDecoder(bytes.NewReader(fixtureRaw))
	decoder.DisallowUnknownFields()
	fixture := backlogDispatchFixture{}
	if err := decoder.Decode(&fixture); err != nil {
		return fmt.Errorf("dispatch fixture is malformed: %w", err)
	}
	if err := ensureJSONEOF(decoder); err != nil {
		return fmt.Errorf("dispatch fixture is malformed: %w", err)
	}
	if fixture.ContractVersion != backlogFixtureContract || !backlogOpaqueID.MatchString(fixture.RemoteTaskID) || !backlogOpaqueID.MatchString(fixture.AdapterIdentity) {
		return errors.New("dispatch fixture has an invalid contract, task ID, or adapter identity")
	}
	parsedTime, err := time.Parse(time.RFC3339, fixture.SubmittedAt)
	if err != nil || parsedTime.Format(time.RFC3339) != fixture.SubmittedAt {
		return errors.New("dispatch fixture submitted_at must be canonical RFC3339")
	}
	payload := map[string]any{
		"contract_version":    backlogTaskContract,
		"remote_task_id":      fixture.RemoteTaskID,
		"request_fingerprint": stringValue(request["request_fingerprint"]),
		"target":              mapValue(request["target"]),
		"submitted_at":        fixture.SubmittedAt,
		"adapter": map[string]any{
			"identity": fixture.AdapterIdentity, "mode": "fixture", "provider_invoked": false,
		},
		"request_artifacts": map[string]any{"json": "remote-request.json", "markdown": "remote-request.md"},
		"capabilities": map[string]any{
			"status": false, "diff": false, "result": false, "apply": false, "commit": false, "push": false, "publication": false,
		},
	}
	dispatchFingerprint, err := backlogJSONFingerprint(payload)
	if err != nil {
		return err
	}
	payload["dispatch_fingerprint"] = dispatchFingerprint
	taskPath, err := backlogArtifactPath(artifactRoot, "remote-task.json")
	if err != nil {
		return err
	}
	if existingRaw, err := os.ReadFile(taskPath); err == nil {
		existing := map[string]any{}
		if json.Unmarshal(existingRaw, &existing) == nil && jsonMapsEqual(existing, payload) {
			return nil
		}
		return errors.New("remote-task.json already exists with different dispatch identity")
	} else if !os.IsNotExist(err) {
		return err
	}
	return writeJSONFileAtomic(taskPath, payload)
}

func loadBacklogFollowup(artifactRoot string) (map[string]any, error) {
	request, _, err := loadAndVerifyBacklogRequest(artifactRoot)
	if err != nil {
		return nil, err
	}
	taskPath, err := backlogArtifactPath(artifactRoot, "remote-task.json")
	if err != nil {
		return nil, err
	}
	task, err := readStrictJSONMap(taskPath)
	if err != nil {
		return nil, fmt.Errorf("remote task cannot be loaded: %w", err)
	}
	if stringValue(task["contract_version"]) != backlogTaskContract || stringValue(task["request_fingerprint"]) != stringValue(request["request_fingerprint"]) {
		return nil, errors.New("remote task does not match the immutable request fingerprint")
	}
	withoutDispatchFingerprint := copyMap(task)
	delete(withoutDispatchFingerprint, "dispatch_fingerprint")
	dispatchFingerprint, err := backlogJSONFingerprint(withoutDispatchFingerprint)
	if err != nil || stringValue(task["dispatch_fingerprint"]) != dispatchFingerprint {
		return nil, errors.New("remote task dispatch fingerprint does not match its immutable identity")
	}
	if !jsonMapsEqual(mapValue(task["target"]), mapValue(request["target"])) {
		return nil, errors.New("remote task target does not match the immutable request")
	}
	adapter := mapValue(task["adapter"])
	if !backlogOpaqueID.MatchString(stringValue(task["remote_task_id"])) || stringValue(adapter["mode"]) != "fixture" || adapter["provider_invoked"] != false {
		return nil, errors.New("remote task fixture identity is malformed or claims provider invocation")
	}
	return map[string]any{
		"contract_version":    backlogFollowupContract,
		"remote_task_id":      stringValue(task["remote_task_id"]),
		"request_fingerprint": stringValue(task["request_fingerprint"]),
		"target":              mapValue(task["target"]),
		"submitted_at":        stringValue(task["submitted_at"]),
		"adapter":             mapValue(task["adapter"]),
		"request_artifacts":   mapValue(task["request_artifacts"]),
	}, nil
}

func loadAndVerifyBacklogRequest(artifactRoot string) (map[string]any, string, error) {
	requestPath, err := backlogArtifactPath(artifactRoot, "remote-request.json")
	if err != nil {
		return nil, "", err
	}
	request, err := readStrictJSONMap(requestPath)
	if err != nil {
		return nil, "", fmt.Errorf("remote request cannot be loaded: %w", err)
	}
	if stringValue(request["contract_version"]) != backlogRequestContract {
		return nil, "", errors.New("remote request has an unsupported contract")
	}
	markdownPath, err := backlogArtifactPath(artifactRoot, "remote-request.md")
	if err != nil {
		return nil, "", err
	}
	markdownRaw, err := os.ReadFile(markdownPath)
	if err != nil {
		return nil, "", fmt.Errorf("remote request markdown cannot be read: %w", err)
	}
	withoutFingerprint := copyMap(request)
	delete(withoutFingerprint, "request_fingerprint")
	expectedMarkdown := renderBacklogRequestMarkdown(withoutFingerprint)
	if string(markdownRaw) != expectedMarkdown {
		return nil, "", errors.New("remote request markdown does not match remote-request.json")
	}
	fingerprint, err := backlogRequestFingerprint(withoutFingerprint, expectedMarkdown)
	if err != nil {
		return nil, "", err
	}
	if stringValue(request["request_fingerprint"]) != fingerprint {
		return nil, "", errors.New("remote request fingerprint does not match immutable request artifacts")
	}
	return request, expectedMarkdown, nil
}

func backlogRequestFingerprint(payload map[string]any, markdown string) (string, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	hash := sha256.New()
	_, _ = hash.Write(raw)
	_, _ = hash.Write([]byte("\n---remote-request.md---\n"))
	_, _ = hash.Write([]byte(markdown))
	return "sha256:" + hex.EncodeToString(hash.Sum(nil)), nil
}

func backlogJSONFingerprint(payload map[string]any) (string, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	return sha256String(raw), nil
}

func resolveBacklogRepoPath(repoRoot, rel string, requireFile, allowDir bool) (string, error) {
	if strings.TrimSpace(repoRoot) == "" || rel == "" || rel != strings.TrimSpace(rel) || filepath.IsAbs(rel) || filepath.VolumeName(rel) != "" || strings.Contains(rel, "\\") {
		return "", fmt.Errorf("path %q must be a canonical repo-relative path", rel)
	}
	clean := pathpkg.Clean(rel)
	if clean != rel || clean == "." || clean == ".." || strings.HasPrefix(clean, "../") {
		return "", fmt.Errorf("path %q escapes or names the repository root", rel)
	}
	root, err := filepath.Abs(repoRoot)
	if err != nil {
		return "", err
	}
	candidate, err := filepath.Abs(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil || !withinRoot(root, candidate) {
		return "", fmt.Errorf("path %q escapes the repository", rel)
	}
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(candidate)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("path %q does not exist", rel)
		}
		return "", err
	}
	if !withinRoot(resolvedRoot, resolved) {
		return "", fmt.Errorf("path %q escapes the repository through a link", rel)
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return "", err
	}
	if requireFile && !info.Mode().IsRegular() {
		return "", fmt.Errorf("path %q is not a regular file", rel)
	}
	if !requireFile && !allowDir && info.IsDir() {
		return "", fmt.Errorf("path %q is not a file", rel)
	}
	return resolved, nil
}

func backlogArtifactPath(root, name string) (string, error) {
	if strings.TrimSpace(root) == "" {
		return "", errors.New("backlog artifact root is required")
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(abs, 0o755); err != nil {
		return "", err
	}
	return filepath.Join(abs, name), nil
}

func parseBacklogStringList(label, raw string, required bool) ([]string, error) {
	values := []string{}
	decoder := json.NewDecoder(strings.NewReader(raw))
	if err := decoder.Decode(&values); err != nil {
		return nil, fmt.Errorf("%s must be a JSON string array: %w", label, err)
	}
	if err := ensureJSONEOF(decoder); err != nil {
		return nil, fmt.Errorf("%s must be one JSON string array: %w", label, err)
	}
	if required && len(values) == 0 {
		return nil, fmt.Errorf("%s must contain at least one item", label)
	}
	seen := map[string]bool{}
	for _, value := range values {
		if value == "" || value != strings.TrimSpace(value) || strings.ContainsAny(value, "\r\n") || len(value) > 500 || !utf8.ValidString(value) {
			return nil, fmt.Errorf("%s contains an empty, untrimmed, multiline, oversized, or invalid item", label)
		}
		if seen[value] {
			return nil, fmt.Errorf("%s contains duplicate item %q", label, value)
		}
		seen[value] = true
	}
	return values, nil
}

func validateBacklogReference(label, value string) error {
	if value != strings.TrimSpace(value) || !backlogReference.MatchString(value) {
		return fmt.Errorf("%s reference must use only letters, numbers, dot, underscore, slash, at, colon, or hyphen", label)
	}
	return nil
}

func isForbiddenBacklogPath(value string) bool {
	lower := strings.ToLower("/" + strings.ReplaceAll(value, "\\", "/") + "/")
	for _, token := range []string{"/.git/", "/.dockpipe/", "/.dorkpipe/", "/bin/.dockpipe/", "/.env/", "/secrets/", "/credentials/", "/provider-transcripts/"} {
		if strings.Contains(lower, token) {
			return true
		}
	}
	base := strings.ToLower(pathpkg.Base(value))
	return base == ".env" || base == "secrets.json" || base == "credentials.json" || base == "token.json" || strings.HasSuffix(base, ".pem") || strings.HasSuffix(base, ".key")
}

func readStrictJSONMap(path string) (map[string]any, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	payload := map[string]any{}
	if err := decoder.Decode(&payload); err != nil {
		return nil, err
	}
	if err := ensureJSONEOF(decoder); err != nil {
		return nil, err
	}
	return payload, nil
}

func ensureJSONEOF(decoder *json.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return errors.New("multiple JSON values are not allowed")
		}
		return err
	}
	return nil
}

func writeTextFileAtomic(path, content string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), ".backlog-remote-*.tmp")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if _, err = temporary.WriteString(content); err == nil {
		err = temporary.Sync()
	}
	if closeErr := temporary.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return os.Rename(temporaryPath, path)
}

func sha256String(raw []byte) string {
	sum := sha256.Sum256(raw)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func appendUniqueStrings(base []string, additional ...string) []string {
	out := append([]string{}, base...)
	seen := map[string]bool{}
	for _, value := range out {
		seen[value] = true
	}
	for _, value := range additional {
		if !seen[value] {
			seen[value] = true
			out = append(out, value)
		}
	}
	return out
}

func sourceDigest(files []any, path string) string {
	for _, raw := range files {
		file := mapValue(raw)
		if stringValue(file["path"]) == path {
			return stringValue(file["sha256"])
		}
	}
	return ""
}

func anyStrings(values []string) []any {
	out := make([]any, 0, len(values))
	for _, value := range values {
		out = append(out, value)
	}
	return out
}

func jsonMapsEqual(left, right map[string]any) bool {
	leftRaw, leftErr := json.Marshal(left)
	rightRaw, rightErr := json.Marshal(right)
	return leftErr == nil && rightErr == nil && bytes.Equal(leftRaw, rightRaw)
}
