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
	backlogIndexPath                    = "docs/agents/task-index.yaml"
	backlogSelectionContract            = "dorkpipe.backlog-selection/v1"
	backlogRequestContract              = "dorkpipe.remote-request/v1"
	backlogTaskContract                 = "dorkpipe.remote-task/v1"
	backlogFollowupContract             = "dorkpipe.remote-followup/v1"
	backlogFixtureContract              = "dorkpipe.remote-dispatch-fixture/v1"
	backlogCompatibilityFixtureContract = "dorkpipe.codex-cloud-cli-compatibility-fixture/v1"
	backlogCompatibilityContract        = "dorkpipe.remote-adapter-compatibility/v1"
	backlogCompletionFixtureContract    = "dorkpipe.remote-completion-candidate-fixture/v1"
	backlogCompletionCandidateContract  = "dorkpipe.remote-completion-candidate/v1"
	backlogStatusFixtureContract        = "dorkpipe.remote-status-observation-fixture/v1"
	backlogStatusContract               = "dorkpipe.remote-status/v1"
)

var (
	backlogTaskIDPattern = regexp.MustCompile(`^TASK-[0-9]{3}$`)
	backlogBaseline      = regexp.MustCompile(`^[0-9a-f]{40}$`)
	backlogOpaqueID      = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{7,127}$`)
	backlogReference     = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._/@:-]{0,199}$`)
	backlogFingerprint   = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)
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

type backlogCompatibilityFixture struct {
	ContractVersion   string                        `json:"contract_version"`
	AdapterIdentity   string                        `json:"adapter_identity"`
	CLI               backlogCompatibilityCLI       `json:"cli"`
	InspectedCommands []backlogCompatibilityCommand `json:"inspected_commands"`
	RecognizedInputs  []backlogCompatibilityInput   `json:"recognized_inputs"`
	SubmissionReceipt backlogCompatibilityReceipt   `json:"submission_receipt"`
	ExactGap          string                        `json:"exact_gap"`
}

type backlogCompatibilityCLI struct {
	Reference string `json:"reference"`
	Version   string `json:"version"`
}

type backlogCompatibilityCommand struct {
	Argv    []string `json:"argv"`
	Fixture string   `json:"fixture"`
}

type backlogCompatibilityInput struct {
	Name     string `json:"name"`
	Flag     string `json:"flag"`
	Value    string `json:"value"`
	Required bool   `json:"required"`
}

type backlogCompatibilityReceipt struct {
	MachineReadableDocumented     bool `json:"machine_readable_documented"`
	StableOpaqueTaskIDRecoverable bool `json:"stable_opaque_task_id_recoverable"`
}

type backlogCompletionFixture struct {
	ContractVersion       string `json:"contract_version"`
	CandidateID           string `json:"candidate_id"`
	ReplayIdentity        string `json:"replay_identity"`
	AdapterIdentity       string `json:"adapter_identity"`
	RemoteTaskID          string `json:"remote_task_id"`
	RequestFingerprint    string `json:"request_fingerprint"`
	DispatchFingerprint   string `json:"dispatch_fingerprint"`
	EnvironmentRef        string `json:"environment_ref"`
	BranchRef             string `json:"branch_ref"`
	ObservedAt            string `json:"observed_at"`
	ClaimedTerminalSignal string `json:"claimed_terminal_signal"`
}

type backlogStatusFixture struct {
	ContractVersion                string `json:"contract_version"`
	ObservationID                  string `json:"observation_id"`
	ReplayIdentity                 string `json:"replay_identity"`
	CompletionCandidateID          string `json:"completion_candidate_id"`
	CompletionCandidateFingerprint string `json:"completion_candidate_fingerprint"`
	AdapterIdentity                string `json:"adapter_identity"`
	RemoteTaskID                   string `json:"remote_task_id"`
	RequestFingerprint             string `json:"request_fingerprint"`
	DispatchFingerprint            string `json:"dispatch_fingerprint"`
	EnvironmentRef                 string `json:"environment_ref"`
	BranchRef                      string `json:"branch_ref"`
	ObservedAt                     string `json:"observed_at"`
	ClaimedRemoteStatus            string `json:"claimed_remote_status"`
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

func preflightBacklogRemoteCompatibility(artifactRoot, fixtureRoot string) error {
	request, _, err := loadAndVerifyBacklogRequest(artifactRoot)
	if err != nil {
		return err
	}
	compatibilityPath, err := backlogArtifactPath(artifactRoot, "remote-adapter-compatibility.json")
	if err != nil {
		return err
	}
	binding := map[string]any{
		"request_fingerprint": stringValue(request["request_fingerprint"]),
		"environment_ref":     stringValue(mapValue(request["target"])["environment_ref"]),
		"branch_ref":          stringValue(mapValue(request["target"])["branch_ref"]),
	}
	fail := func(cause error) error {
		payload := map[string]any{
			"contract_version":          backlogCompatibilityContract,
			"request_binding":           binding,
			"enabled_dispatch_adapters": []any{},
			"live_submission_enabled":   false,
			"compatibility": map[string]any{
				"status": "error", "reason_code": "invalid_compatibility_fixture", "reason": cause.Error(),
			},
		}
		if writeErr := writeJSONFileAtomic(compatibilityPath, payload); writeErr != nil {
			return fmt.Errorf("%w; writing compatibility failure artifact: %v", cause, writeErr)
		}
		return cause
	}

	fixture, fixtureRaw, outputs, err := loadBacklogCompatibilityFixture(fixtureRoot)
	if err != nil {
		return fail(err)
	}
	if fixture.SubmissionReceipt.MachineReadableDocumented || fixture.SubmissionReceipt.StableOpaqueTaskIDRecoverable {
		return fail(errors.New("compatibility fixture claims a receipt contract that this fail-closed slice does not implement"))
	}
	commandArtifacts := make([]any, 0, len(fixture.InspectedCommands))
	digestInput := append([]byte{}, fixtureRaw...)
	for i, command := range fixture.InspectedCommands {
		raw := outputs[i]
		digestInput = append(digestInput, []byte("\n---"+command.Fixture+"---\n")...)
		digestInput = append(digestInput, raw...)
		commandArtifacts = append(commandArtifacts, map[string]any{
			"argv": anyStrings(command.Argv), "fixture": command.Fixture, "sha256": sha256String(raw),
		})
	}
	inputs := map[string]any{}
	for _, input := range fixture.RecognizedInputs {
		inputs[input.Name] = map[string]any{"flag": input.Flag, "value": input.Value, "required": input.Required}
	}
	payload := map[string]any{
		"contract_version": backlogCompatibilityContract,
		"adapter_identity": fixture.AdapterIdentity,
		"inspected_cli": map[string]any{
			"reference": fixture.CLI.Reference, "version": fixture.CLI.Version,
			"fixture_contract": fixture.ContractVersion, "fixture_digest": sha256String(digestInput),
		},
		"required_command_surface": commandArtifacts,
		"recognized_inputs":        inputs,
		"submission_receipt": map[string]any{
			"machine_readable_documented": false, "stable_opaque_task_id_recoverable": false,
		},
		"request_binding": binding,
		"compatibility": map[string]any{
			"status": "unsupported", "reason_code": "machine_readable_submission_receipt_not_documented", "reason": fixture.ExactGap,
		},
		"enabled_dispatch_adapters": []any{"fixture_only"},
		"live_submission_enabled":   false,
	}
	return writeJSONFileAtomic(compatibilityPath, payload)
}

func loadBacklogCompatibilityFixture(root string) (*backlogCompatibilityFixture, []byte, [][]byte, error) {
	if strings.TrimSpace(root) == "" {
		return nil, nil, nil, errors.New("compatibility fixture root is required")
	}
	contractPath := filepath.Join(root, "contract.json")
	contractRaw, err := os.ReadFile(contractPath)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("compatibility contract cannot be read: %w", err)
	}
	decoder := json.NewDecoder(bytes.NewReader(contractRaw))
	decoder.DisallowUnknownFields()
	fixture := &backlogCompatibilityFixture{}
	if err := decoder.Decode(fixture); err != nil {
		return nil, nil, nil, fmt.Errorf("compatibility contract is malformed: %w", err)
	}
	if err := ensureJSONEOF(decoder); err != nil {
		return nil, nil, nil, fmt.Errorf("compatibility contract is malformed: %w", err)
	}
	if fixture.ContractVersion != backlogCompatibilityFixtureContract || fixture.AdapterIdentity != "codex-cloud-cli" || fixture.CLI.Reference != "codex" || strings.TrimSpace(fixture.CLI.Version) == "" {
		return nil, nil, nil, errors.New("compatibility contract has an invalid contract, adapter identity, CLI reference, or version")
	}
	expectedCommands := []backlogCompatibilityCommand{
		{Argv: []string{"codex", "--version"}, Fixture: "codex-version.txt"},
		{Argv: []string{"codex", "cloud", "--help"}, Fixture: "codex-cloud-help.txt"},
		{Argv: []string{"codex", "cloud", "exec", "--help"}, Fixture: "codex-cloud-exec-help.txt"},
	}
	if len(fixture.InspectedCommands) != len(expectedCommands) {
		return nil, nil, nil, errors.New("compatibility contract must inspect exactly codex --version, codex cloud --help, and codex cloud exec --help")
	}
	outputs := make([][]byte, 0, len(expectedCommands))
	for i, expected := range expectedCommands {
		actual := fixture.InspectedCommands[i]
		if !stringSlicesEqual(actual.Argv, expected.Argv) || actual.Fixture != expected.Fixture {
			return nil, nil, nil, errors.New("compatibility contract command surface is not the required Codex Cloud inspection surface")
		}
		raw, err := os.ReadFile(filepath.Join(root, expected.Fixture))
		if err != nil {
			return nil, nil, nil, fmt.Errorf("compatibility help fixture %q cannot be read: %w", expected.Fixture, err)
		}
		outputs = append(outputs, raw)
	}
	if string(outputs[0]) != fixture.CLI.Version+"\n" {
		return nil, nil, nil, errors.New("codex version fixture does not match the declared inspected CLI version")
	}
	cloudHelp := string(outputs[1])
	execHelp := string(outputs[2])
	if !strings.Contains(cloudHelp, "Usage: codex cloud [OPTIONS] [COMMAND]") || !strings.Contains(cloudHelp, "exec    Submit a new Codex Cloud task without launching the TUI") {
		return nil, nil, nil, errors.New("codex cloud help fixture does not document the required exec command surface")
	}
	if !strings.Contains(execHelp, "Usage: codex cloud exec [OPTIONS] --env <ENV_ID> [QUERY]") || !strings.Contains(execHelp, "--env <ENV_ID>") || !strings.Contains(execHelp, "--branch <BRANCH>") {
		return nil, nil, nil, errors.New("codex cloud exec help fixture does not document the required environment and branch inputs")
	}
	expectedInputs := []backlogCompatibilityInput{
		{Name: "environment", Flag: "--env", Value: "ENV_ID", Required: true},
		{Name: "branch", Flag: "--branch", Value: "BRANCH", Required: false},
	}
	if len(fixture.RecognizedInputs) != len(expectedInputs) {
		return nil, nil, nil, errors.New("compatibility contract must recognize exactly the documented environment and branch inputs")
	}
	for i, expected := range expectedInputs {
		if fixture.RecognizedInputs[i] != expected {
			return nil, nil, nil, errors.New("compatibility contract environment or branch input does not match documented help")
		}
	}
	if fixture.ExactGap == "" || fixture.ExactGap != strings.TrimSpace(fixture.ExactGap) || strings.ContainsAny(fixture.ExactGap, "\r\n") {
		return nil, nil, nil, errors.New("compatibility contract requires one exact trimmed fail-closed gap")
	}
	return fixture, contractRaw, outputs, nil
}

func stringSlicesEqual(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

func dispatchBacklogFixture(artifactRoot, fixturePath string) error {
	request, _, err := loadAndVerifyBacklogRequest(artifactRoot)
	if err != nil {
		return err
	}
	_, compatibilityFingerprint, err := loadAndVerifyBacklogCompatibility(artifactRoot, request)
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
		"contract_version":          backlogTaskContract,
		"remote_task_id":            fixture.RemoteTaskID,
		"request_fingerprint":       stringValue(request["request_fingerprint"]),
		"compatibility_fingerprint": compatibilityFingerprint,
		"target":                    mapValue(request["target"]),
		"submitted_at":              fixture.SubmittedAt,
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
	_, compatibilityFingerprint, err := loadAndVerifyBacklogCompatibility(artifactRoot, request)
	if err != nil {
		return nil, err
	}
	task, err := loadAndVerifyBacklogDispatch(artifactRoot, request, compatibilityFingerprint)
	if err != nil {
		return nil, err
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

func loadAndVerifyBacklogCompatibility(artifactRoot string, request map[string]any) (map[string]any, string, error) {
	compatibilityPath, err := backlogArtifactPath(artifactRoot, "remote-adapter-compatibility.json")
	if err != nil {
		return nil, "", err
	}
	compatibility, err := readStrictJSONMap(compatibilityPath)
	if err != nil {
		return nil, "", fmt.Errorf("remote adapter compatibility cannot be loaded: %w", err)
	}
	binding := mapValue(compatibility["request_binding"])
	target := mapValue(request["target"])
	receipt := mapValue(compatibility["submission_receipt"])
	status := mapValue(compatibility["compatibility"])
	if stringValue(compatibility["contract_version"]) != backlogCompatibilityContract ||
		stringValue(compatibility["adapter_identity"]) != "codex-cloud-cli" ||
		stringValue(binding["request_fingerprint"]) != stringValue(request["request_fingerprint"]) ||
		stringValue(binding["environment_ref"]) != stringValue(target["environment_ref"]) ||
		stringValue(binding["branch_ref"]) != stringValue(target["branch_ref"]) ||
		compatibility["live_submission_enabled"] != false ||
		!stringSlicesEqual(stringList(compatibility["enabled_dispatch_adapters"]), []string{"fixture_only"}) ||
		stringValue(status["status"]) != "unsupported" ||
		stringValue(status["reason_code"]) != "machine_readable_submission_receipt_not_documented" ||
		receipt["machine_readable_documented"] != false ||
		receipt["stable_opaque_task_id_recoverable"] != false {
		return nil, "", errors.New("remote adapter compatibility is malformed, unsupported, or does not match the immutable request")
	}
	fingerprint, err := backlogJSONFingerprint(compatibility)
	if err != nil {
		return nil, "", err
	}
	return compatibility, fingerprint, nil
}

func loadAndVerifyBacklogDispatch(artifactRoot string, request map[string]any, compatibilityFingerprint string) (map[string]any, error) {
	taskPath, err := backlogArtifactPath(artifactRoot, "remote-task.json")
	if err != nil {
		return nil, err
	}
	task, err := readStrictJSONMap(taskPath)
	if err != nil {
		return nil, fmt.Errorf("remote task cannot be loaded: %w", err)
	}
	if stringValue(task["contract_version"]) != backlogTaskContract ||
		stringValue(task["request_fingerprint"]) != stringValue(request["request_fingerprint"]) ||
		stringValue(task["compatibility_fingerprint"]) != compatibilityFingerprint {
		return nil, errors.New("remote task does not match the immutable request or compatibility fingerprint")
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
	if !backlogOpaqueID.MatchString(stringValue(task["remote_task_id"])) ||
		!backlogOpaqueID.MatchString(stringValue(adapter["identity"])) ||
		stringValue(adapter["mode"]) != "fixture" || adapter["provider_invoked"] != false {
		return nil, errors.New("remote task fixture identity is malformed or claims provider invocation")
	}
	parsedTime, err := time.Parse(time.RFC3339, stringValue(task["submitted_at"]))
	if err != nil || parsedTime.Format(time.RFC3339) != stringValue(task["submitted_at"]) {
		return nil, errors.New("remote task submitted_at is not canonical RFC3339")
	}
	capabilities := mapValue(task["capabilities"])
	for _, name := range []string{"status", "diff", "result", "apply", "commit", "push", "publication"} {
		if capabilities[name] != false {
			return nil, fmt.Errorf("remote task unexpectedly enables %s", name)
		}
	}
	return task, nil
}

func ingestBacklogCompletionCandidate(artifactRoot, fixturePath string) error {
	request, _, err := loadAndVerifyBacklogRequest(artifactRoot)
	if err != nil {
		return rejectBacklog("completion_candidate_request_invalid", "%v", err)
	}
	_, compatibilityFingerprint, err := loadAndVerifyBacklogCompatibility(artifactRoot, request)
	if err != nil {
		return rejectBacklog("completion_candidate_compatibility_invalid", "%v", err)
	}
	task, err := loadAndVerifyBacklogDispatch(artifactRoot, request, compatibilityFingerprint)
	if err != nil {
		return rejectBacklog("completion_candidate_dispatch_invalid", "%v", err)
	}

	fixtureRaw, err := os.ReadFile(fixturePath)
	if err != nil {
		return rejectBacklog("completion_candidate_fixture_malformed", "completion candidate fixture cannot be read: %v", err)
	}
	decoder := json.NewDecoder(bytes.NewReader(fixtureRaw))
	decoder.DisallowUnknownFields()
	fixture := backlogCompletionFixture{}
	if err := decoder.Decode(&fixture); err != nil {
		return rejectBacklog("completion_candidate_fixture_malformed", "completion candidate fixture is malformed: %v", err)
	}
	if err := ensureJSONEOF(decoder); err != nil {
		return rejectBacklog("completion_candidate_fixture_malformed", "completion candidate fixture is malformed: %v", err)
	}
	if fixture.ContractVersion != backlogCompletionFixtureContract ||
		!backlogOpaqueID.MatchString(fixture.CandidateID) ||
		!backlogOpaqueID.MatchString(fixture.ReplayIdentity) ||
		fixture.CandidateID == fixture.ReplayIdentity ||
		!backlogOpaqueID.MatchString(fixture.AdapterIdentity) ||
		!backlogOpaqueID.MatchString(fixture.RemoteTaskID) ||
		!backlogFingerprint.MatchString(fixture.RequestFingerprint) ||
		!backlogFingerprint.MatchString(fixture.DispatchFingerprint) {
		return rejectBacklog("completion_candidate_identity_invalid", "completion candidate contract or identity fields are invalid")
	}
	if err := validateBacklogReference("environment", fixture.EnvironmentRef); err != nil {
		return rejectBacklog("completion_candidate_identity_invalid", "%v", err)
	}
	if err := validateBacklogReference("branch", fixture.BranchRef); err != nil {
		return rejectBacklog("completion_candidate_identity_invalid", "%v", err)
	}
	if fixture.ClaimedTerminalSignal != "completed" {
		return rejectBacklog("completion_candidate_claim_invalid", "fixture completion candidates must carry exactly the untrusted completed claim")
	}

	target := mapValue(task["target"])
	adapter := mapValue(task["adapter"])
	if fixture.RemoteTaskID != stringValue(task["remote_task_id"]) ||
		fixture.RequestFingerprint != stringValue(task["request_fingerprint"]) ||
		fixture.DispatchFingerprint != stringValue(task["dispatch_fingerprint"]) ||
		fixture.AdapterIdentity != stringValue(adapter["identity"]) ||
		fixture.EnvironmentRef != stringValue(target["environment_ref"]) ||
		fixture.BranchRef != stringValue(target["branch_ref"]) {
		return rejectBacklog("completion_candidate_binding_mismatch", "completion candidate does not match the immutable task, request, dispatch, adapter, environment, and branch identity")
	}
	observedAt, err := time.Parse(time.RFC3339, fixture.ObservedAt)
	if err != nil || observedAt.Format(time.RFC3339) != fixture.ObservedAt {
		return rejectBacklog("completion_candidate_observation_invalid", "completion candidate observed_at must be canonical RFC3339")
	}
	submittedAt, _ := time.Parse(time.RFC3339, stringValue(task["submitted_at"]))
	if !observedAt.After(submittedAt) {
		return rejectBacklog("completion_candidate_stale", "completion candidate observed_at must be later than the immutable dispatch time")
	}

	candidatePath, err := backlogArtifactPath(artifactRoot, "completion-candidate.json")
	if err != nil {
		return err
	}
	if _, err := os.Stat(candidatePath); err == nil {
		existing, readErr := readStrictJSONMap(candidatePath)
		if readErr != nil || stringValue(existing["contract_version"]) != backlogCompletionCandidateContract || stringValue(existing["state"]) != "completion_candidate" {
			return rejectBacklog("completion_candidate_artifact_invalid", "existing completion-candidate.json is malformed or tampered")
		}
		identity := mapValue(existing["identity"])
		switch {
		case stringValue(identity["candidate_id"]) == fixture.CandidateID:
			return rejectBacklog("completion_candidate_duplicate", "candidate identity %q was already ingested", fixture.CandidateID)
		case stringValue(identity["replay_identity"]) == fixture.ReplayIdentity:
			return rejectBacklog("completion_candidate_replay", "replay identity %q was already ingested", fixture.ReplayIdentity)
		default:
			return rejectBacklog("completion_candidate_already_recorded", "one completion candidate is already recorded for the immutable dispatch")
		}
	} else if !os.IsNotExist(err) {
		return err
	}

	payload := map[string]any{
		"contract_version": backlogCompletionCandidateContract,
		"state":            "completion_candidate",
		"identity": map[string]any{
			"candidate_id": fixture.CandidateID, "replay_identity": fixture.ReplayIdentity,
		},
		"binding": map[string]any{
			"remote_task_id": fixture.RemoteTaskID, "request_fingerprint": fixture.RequestFingerprint,
			"dispatch_fingerprint": fixture.DispatchFingerprint, "adapter_identity": fixture.AdapterIdentity,
			"environment_ref": fixture.EnvironmentRef, "branch_ref": fixture.BranchRef,
		},
		"observed_at":             fixture.ObservedAt,
		"claimed_terminal_signal": fixture.ClaimedTerminalSignal,
		"source": map[string]any{
			"mode": "fixture", "provider_invoked": false, "terminal_claim_trusted": false,
		},
		"lifecycle": map[string]any{
			"ready_for_review": false, "status_retrieval": false, "diff_retrieval": false,
			"result_retrieval": false, "validation": false, "apply": false, "commit": false,
			"push": false, "publication": false,
		},
	}
	return writeJSONFileAtomic(candidatePath, payload)
}

func retrieveBacklogRemoteStatusFixture(artifactRoot, fixturePath string) error {
	request, _, err := loadAndVerifyBacklogRequest(artifactRoot)
	if err != nil {
		return rejectBacklog("remote_status_request_invalid", "%v", err)
	}
	_, compatibilityFingerprint, err := loadAndVerifyBacklogCompatibility(artifactRoot, request)
	if err != nil {
		return rejectBacklog("remote_status_compatibility_invalid", "%v", err)
	}
	task, err := loadAndVerifyBacklogDispatch(artifactRoot, request, compatibilityFingerprint)
	if err != nil {
		return rejectBacklog("remote_status_dispatch_invalid", "%v", err)
	}
	candidate, candidateFingerprint, err := loadAndVerifyBacklogCompletionCandidate(artifactRoot, task)
	if err != nil {
		return rejectBacklog("remote_status_candidate_invalid", "%v", err)
	}

	fixtureRaw, err := os.ReadFile(fixturePath)
	if err != nil {
		return rejectBacklog("remote_status_fixture_malformed", "remote status fixture cannot be read: %v", err)
	}
	decoder := json.NewDecoder(bytes.NewReader(fixtureRaw))
	decoder.DisallowUnknownFields()
	fixture := backlogStatusFixture{}
	if err := decoder.Decode(&fixture); err != nil {
		return rejectBacklog("remote_status_fixture_malformed", "remote status fixture is malformed: %v", err)
	}
	if err := ensureJSONEOF(decoder); err != nil {
		return rejectBacklog("remote_status_fixture_malformed", "remote status fixture is malformed: %v", err)
	}
	if fixture.ContractVersion != backlogStatusFixtureContract ||
		!backlogOpaqueID.MatchString(fixture.ObservationID) ||
		!backlogOpaqueID.MatchString(fixture.ReplayIdentity) ||
		fixture.ObservationID == fixture.ReplayIdentity ||
		!backlogOpaqueID.MatchString(fixture.CompletionCandidateID) ||
		!backlogFingerprint.MatchString(fixture.CompletionCandidateFingerprint) ||
		!backlogOpaqueID.MatchString(fixture.AdapterIdentity) ||
		!backlogOpaqueID.MatchString(fixture.RemoteTaskID) ||
		!backlogFingerprint.MatchString(fixture.RequestFingerprint) ||
		!backlogFingerprint.MatchString(fixture.DispatchFingerprint) {
		return rejectBacklog("remote_status_identity_invalid", "remote status contract or identity fields are invalid")
	}
	if err := validateBacklogReference("environment", fixture.EnvironmentRef); err != nil {
		return rejectBacklog("remote_status_identity_invalid", "%v", err)
	}
	if err := validateBacklogReference("branch", fixture.BranchRef); err != nil {
		return rejectBacklog("remote_status_identity_invalid", "%v", err)
	}
	if fixture.ClaimedRemoteStatus != "completed" {
		return rejectBacklog("remote_status_claim_invalid", "fixture status observations must carry exactly the untrusted completed claim")
	}

	candidateIdentity := mapValue(candidate["identity"])
	target := mapValue(task["target"])
	adapter := mapValue(task["adapter"])
	if fixture.CompletionCandidateID != stringValue(candidateIdentity["candidate_id"]) ||
		fixture.CompletionCandidateFingerprint != candidateFingerprint ||
		fixture.RemoteTaskID != stringValue(task["remote_task_id"]) ||
		fixture.RequestFingerprint != stringValue(task["request_fingerprint"]) ||
		fixture.DispatchFingerprint != stringValue(task["dispatch_fingerprint"]) ||
		fixture.AdapterIdentity != stringValue(adapter["identity"]) ||
		fixture.EnvironmentRef != stringValue(target["environment_ref"]) ||
		fixture.BranchRef != stringValue(target["branch_ref"]) {
		return rejectBacklog("remote_status_binding_mismatch", "remote status observation does not match the accepted candidate, task, request, dispatch, adapter, environment, and branch identity")
	}
	observedAt, err := time.Parse(time.RFC3339, fixture.ObservedAt)
	if err != nil || observedAt.Format(time.RFC3339) != fixture.ObservedAt {
		return rejectBacklog("remote_status_observation_invalid", "remote status observed_at must be canonical RFC3339")
	}
	submittedAt, _ := time.Parse(time.RFC3339, stringValue(task["submitted_at"]))
	candidateObservedAt, _ := time.Parse(time.RFC3339, stringValue(candidate["observed_at"]))
	if !observedAt.After(submittedAt) || !observedAt.After(candidateObservedAt) {
		return rejectBacklog("remote_status_stale", "remote status observed_at must be later than both dispatch and completion-candidate observation times")
	}

	statusPath, err := backlogArtifactPath(artifactRoot, "remote-status.json")
	if err != nil {
		return err
	}
	if _, err := os.Stat(statusPath); err == nil {
		existing, readErr := readStrictJSONMap(statusPath)
		if readErr != nil || stringValue(existing["contract_version"]) != backlogStatusContract || stringValue(existing["state"]) != "completion_candidate" {
			return rejectBacklog("remote_status_artifact_invalid", "existing remote-status.json is malformed or tampered")
		}
		identity := mapValue(existing["identity"])
		switch {
		case stringValue(identity["observation_id"]) == fixture.ObservationID:
			return rejectBacklog("remote_status_duplicate", "status observation identity %q was already ingested", fixture.ObservationID)
		case stringValue(identity["replay_identity"]) == fixture.ReplayIdentity:
			return rejectBacklog("remote_status_replay", "status replay identity %q was already ingested", fixture.ReplayIdentity)
		default:
			return rejectBacklog("remote_status_already_recorded", "one remote status observation is already recorded for the accepted completion candidate")
		}
	} else if !os.IsNotExist(err) {
		return err
	}

	payload := map[string]any{
		"contract_version": backlogStatusContract,
		"state":            "completion_candidate",
		"identity": map[string]any{
			"observation_id": fixture.ObservationID, "replay_identity": fixture.ReplayIdentity,
		},
		"completion_candidate": map[string]any{
			"candidate_id":    stringValue(candidateIdentity["candidate_id"]),
			"replay_identity": stringValue(candidateIdentity["replay_identity"]),
			"fingerprint":     candidateFingerprint,
		},
		"binding": map[string]any{
			"remote_task_id": fixture.RemoteTaskID, "request_fingerprint": fixture.RequestFingerprint,
			"dispatch_fingerprint": fixture.DispatchFingerprint, "adapter_identity": fixture.AdapterIdentity,
			"environment_ref": fixture.EnvironmentRef, "branch_ref": fixture.BranchRef,
		},
		"observed_at": fixture.ObservedAt,
		"evidence": map[string]any{
			"claimed_remote_status": fixture.ClaimedRemoteStatus, "mode": "fixture",
			"provider_invoked": false, "trusted": false, "authoritative": false,
		},
		"lifecycle": map[string]any{
			"ready_for_review": false, "diff_retrieval": false, "result_retrieval": false,
			"validation": false, "apply": false, "commit": false, "push": false, "publication": false,
		},
	}
	return writeJSONFileAtomic(statusPath, payload)
}

func loadAndVerifyBacklogCompletionCandidate(artifactRoot string, task map[string]any) (map[string]any, string, error) {
	candidatePath, err := backlogArtifactPath(artifactRoot, "completion-candidate.json")
	if err != nil {
		return nil, "", err
	}
	candidate, err := readStrictJSONMap(candidatePath)
	if err != nil {
		return nil, "", fmt.Errorf("completion candidate cannot be loaded: %w", err)
	}
	identity := mapValue(candidate["identity"])
	binding := mapValue(candidate["binding"])
	target := mapValue(task["target"])
	adapter := mapValue(task["adapter"])
	if !backlogOpaqueID.MatchString(stringValue(identity["candidate_id"])) ||
		!backlogOpaqueID.MatchString(stringValue(identity["replay_identity"])) ||
		stringValue(identity["candidate_id"]) == stringValue(identity["replay_identity"]) {
		return nil, "", errors.New("completion candidate identity is malformed")
	}
	observedAt, err := time.Parse(time.RFC3339, stringValue(candidate["observed_at"]))
	if err != nil || observedAt.Format(time.RFC3339) != stringValue(candidate["observed_at"]) {
		return nil, "", errors.New("completion candidate observed_at is not canonical RFC3339")
	}
	submittedAt, _ := time.Parse(time.RFC3339, stringValue(task["submitted_at"]))
	if !observedAt.After(submittedAt) {
		return nil, "", errors.New("completion candidate is stale relative to immutable dispatch")
	}
	expected := map[string]any{
		"contract_version": backlogCompletionCandidateContract,
		"state":            "completion_candidate",
		"identity": map[string]any{
			"candidate_id": stringValue(identity["candidate_id"]), "replay_identity": stringValue(identity["replay_identity"]),
		},
		"binding": map[string]any{
			"remote_task_id": stringValue(task["remote_task_id"]), "request_fingerprint": stringValue(task["request_fingerprint"]),
			"dispatch_fingerprint": stringValue(task["dispatch_fingerprint"]), "adapter_identity": stringValue(adapter["identity"]),
			"environment_ref": stringValue(target["environment_ref"]), "branch_ref": stringValue(target["branch_ref"]),
		},
		"observed_at":             stringValue(candidate["observed_at"]),
		"claimed_terminal_signal": "completed",
		"source": map[string]any{
			"mode": "fixture", "provider_invoked": false, "terminal_claim_trusted": false,
		},
		"lifecycle": map[string]any{
			"ready_for_review": false, "status_retrieval": false, "diff_retrieval": false,
			"result_retrieval": false, "validation": false, "apply": false, "commit": false,
			"push": false, "publication": false,
		},
	}
	if !jsonMapsEqual(candidate, expected) || !jsonMapsEqual(binding, mapValue(expected["binding"])) {
		return nil, "", errors.New("completion candidate is malformed, tampered, or does not match immutable dispatch identity")
	}
	fingerprint, err := backlogJSONFingerprint(candidate)
	if err != nil {
		return nil, "", err
	}
	return candidate, fingerprint, nil
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
