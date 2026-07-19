package orchestrationhelper

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const backlogTestBaseline = "0123456789abcdef0123456789abcdef01234567"

const backlogTestPatch = "diff --git a/packages/dorkpipe/README.md b/packages/dorkpipe/README.md\nindex 1111111..2222222 100644\n--- a/packages/dorkpipe/README.md\n+++ b/packages/dorkpipe/README.md\n@@ -1 +1,2 @@\n # Fixture package\n+Untrusted remote fixture change.\n"

func TestBacklogRemoteArtifactsAreDeterministicAndRestartSafe(t *testing.T) {
	repo := writeBacklogTestRepo(t)
	compatibilityFixture := writeBacklogCompatibilityFixture(t)
	fixture := filepath.Join(t.TempDir(), "dispatch.json")
	writeBacklogTestFile(t, fixture, `{
  "contract_version": "dorkpipe.remote-dispatch-fixture/v1",
  "adapter_identity": "codex-cloud-fixture-v1",
  "remote_task_id": "remote_fixture_task_015",
  "submitted_at": "2026-07-19T00:00:00Z"
}`)

	compile := func(root string) {
		t.Helper()
		if err := inspectBacklogSelection(repo, backlogIndexPath, "TASK-015", "Implement only the bounded offline fixture dispatch slice.", backlogTestBaseline, root); err != nil {
			t.Fatal(err)
		}
		if err := compileBacklogRemoteRequest(
			repo, root, "fixture-environment", "js/dev",
			`["packages/dorkpipe","docs/agents/tasks/backlog-driven-remote-tasks.md"]`,
			`["No live provider invocation","No apply, commit, push, or publication"]`,
			`["go test ./packages/dorkpipe/lib/orchestrationhelper"]`,
			`["docs/agents/packages/package-authoring.md","docs/agents/workflows/yaml-workflows.md"]`,
		); err != nil {
			t.Fatal(err)
		}
		if err := preflightBacklogRemoteCompatibility(root, compatibilityFixture); err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(filepath.Join(root, "remote-task.json")); !os.IsNotExist(err) {
			t.Fatalf("compatibility preflight created remote-task.json: %v", err)
		}
		if err := dispatchBacklogFixture(root, fixture); err != nil {
			t.Fatal(err)
		}
		if err := dispatchBacklogFixture(root, fixture); err != nil {
			t.Fatalf("idempotent fixture dispatch failed: %v", err)
		}
	}

	first := filepath.Join(t.TempDir(), "first")
	second := filepath.Join(t.TempDir(), "second")
	compile(first)
	compile(second)
	firstCandidate := writeBacklogCompletionFixture(t, first, "completion_fixture_candidate_015", "completion_fixture_replay_015", "2026-07-19T00:01:00Z")
	secondCandidate := writeBacklogCompletionFixture(t, second, "completion_fixture_candidate_015", "completion_fixture_replay_015", "2026-07-19T00:01:00Z")
	if err := os.RemoveAll(repo); err != nil {
		t.Fatal(err)
	}
	if err := ingestBacklogCompletionCandidate(first, firstCandidate); err != nil {
		t.Fatalf("artifact-only completion candidate ingestion failed: %v", err)
	}
	if err := ingestBacklogCompletionCandidate(second, secondCandidate); err != nil {
		t.Fatalf("second clean completion candidate ingestion failed: %v", err)
	}
	firstStatus := writeBacklogStatusFixture(t, first, "status_fixture_observation_015", "status_fixture_replay_015", "2026-07-19T00:02:00Z")
	secondStatus := writeBacklogStatusFixture(t, second, "status_fixture_observation_015", "status_fixture_replay_015", "2026-07-19T00:02:00Z")
	if err := retrieveBacklogRemoteStatusFixture(first, firstStatus); err != nil {
		t.Fatalf("artifact-only remote status retrieval failed: %v", err)
	}
	if err := retrieveBacklogRemoteStatusFixture(second, secondStatus); err != nil {
		t.Fatalf("second clean remote status retrieval failed: %v", err)
	}
	immutableBeforeDiff := map[string][]byte{}
	for _, name := range []string{"remote-request.json", "remote-request.md", "remote-adapter-compatibility.json", "remote-task.json", "completion-candidate.json", "remote-status.json"} {
		immutableBeforeDiff[name] = mustReadFile(t, filepath.Join(first, name))
	}
	firstDiff := writeBacklogDiffFixture(t, first, "diff_fixture_observation_015", "diff_fixture_replay_015", "2026-07-19T00:03:00Z", backlogTestPatch)
	secondDiff := writeBacklogDiffFixture(t, second, "diff_fixture_observation_015", "diff_fixture_replay_015", "2026-07-19T00:03:00Z", backlogTestPatch)
	if err := retrieveBacklogRemoteDiffFixture(first, firstDiff); err != nil {
		t.Fatalf("artifact-only remote diff retrieval failed: %v", err)
	}
	if err := retrieveBacklogRemoteDiffFixture(second, secondDiff); err != nil {
		t.Fatalf("second clean remote diff retrieval failed: %v", err)
	}
	for name, before := range immutableBeforeDiff {
		if string(mustReadFile(t, filepath.Join(first, name))) != string(before) {
			t.Fatalf("remote diff retrieval changed immutable artifact %s", name)
		}
	}
	for _, name := range []string{"backlog-selection.json", "remote-request.json", "remote-request.md", "remote-adapter-compatibility.json", "remote-task.json", "completion-candidate.json", "remote-status.json", "remote-diff.json", "remote-diff.patch"} {
		firstRaw := mustReadFile(t, filepath.Join(first, name))
		secondRaw := mustReadFile(t, filepath.Join(second, name))
		if string(firstRaw) != string(secondRaw) {
			t.Fatalf("%s is not deterministic", name)
		}
	}
	compatibility := readJSONMap(filepath.Join(first, "remote-adapter-compatibility.json"))
	if stringValue(mapValue(compatibility["compatibility"])["status"]) != "unsupported" || backlogTestBool(compatibility["live_submission_enabled"]) {
		t.Fatalf("unexpected compatibility artifact: %#v", compatibility)
	}
	binding := mapValue(compatibility["request_binding"])
	request := readJSONMap(filepath.Join(first, "remote-request.json"))
	if stringValue(binding["request_fingerprint"]) != stringValue(request["request_fingerprint"]) || !jsonMapsEqual(map[string]any{"environment_ref": binding["environment_ref"], "branch_ref": binding["branch_ref"]}, mapValue(request["target"])) {
		t.Fatalf("compatibility artifact is not bound to the immutable request: %#v", binding)
	}
	task := readJSONMap(filepath.Join(first, "remote-task.json"))
	if backlogTestBool(mapValue(task["adapter"])["provider_invoked"]) {
		t.Fatal("fixture dispatch claims a live provider invocation")
	}
	capabilities := mapValue(task["capabilities"])
	for _, name := range []string{"status", "diff", "result", "apply", "commit", "push", "publication"} {
		if backlogTestBool(capabilities[name]) {
			t.Fatalf("fixture unexpectedly enables %s", name)
		}
	}
	candidate := readJSONMap(filepath.Join(first, "completion-candidate.json"))
	if stringValue(candidate["state"]) != "completion_candidate" || backlogTestBool(mapValue(candidate["source"])["terminal_claim_trusted"]) {
		t.Fatalf("unexpected completion candidate state or trust: %#v", candidate)
	}
	for name, value := range mapValue(candidate["lifecycle"]) {
		if backlogTestBool(value) {
			t.Fatalf("completion candidate unexpectedly enables %s", name)
		}
	}
	status := readJSONMap(filepath.Join(first, "remote-status.json"))
	statusEvidence := mapValue(status["evidence"])
	if stringValue(status["state"]) != "completion_candidate" || backlogTestBool(statusEvidence["trusted"]) || backlogTestBool(statusEvidence["authoritative"]) || backlogTestBool(statusEvidence["provider_invoked"]) {
		t.Fatalf("unexpected remote status state, trust, authority, or provider claim: %#v", status)
	}
	if stringValue(statusEvidence["claimed_remote_status"]) != "completed" {
		t.Fatalf("unexpected fixture status evidence: %#v", statusEvidence)
	}
	for name, value := range mapValue(status["lifecycle"]) {
		if backlogTestBool(value) {
			t.Fatalf("remote status unexpectedly enables %s", name)
		}
	}
	diffMetadata := readJSONMap(filepath.Join(first, "remote-diff.json"))
	patch := mapValue(diffMetadata["patch"])
	if stringValue(diffMetadata["state"]) != "completion_candidate" || !backlogTestBool(patch["opaque"]) || backlogTestBool(patch["trusted"]) || string(mustReadFile(t, filepath.Join(first, "remote-diff.patch"))) != backlogTestPatch {
		t.Fatalf("unexpected remote diff metadata or patch: %#v", diffMetadata)
	}
	for name, value := range mapValue(diffMetadata["lifecycle"]) {
		if backlogTestBool(value) {
			t.Fatalf("remote diff unexpectedly enables %s", name)
		}
	}
	followup, err := loadBacklogFollowup(first)
	if err != nil {
		t.Fatalf("artifact-only follow-up failed: %v", err)
	}
	if stringValue(followup["remote_task_id"]) != "remote_fixture_task_015" {
		t.Fatalf("unexpected follow-up identity: %#v", followup)
	}
	tamperedTaskPath := filepath.Join(second, "remote-task.json")
	tamperedTask := readJSONMap(tamperedTaskPath)
	tamperedTask["remote_task_id"] = "remote_fixture_task_tampered"
	tamperedRaw, err := json.MarshalIndent(tamperedTask, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	writeBacklogTestFile(t, tamperedTaskPath, string(tamperedRaw)+"\n")
	if _, err := loadBacklogFollowup(second); err == nil {
		t.Fatal("tampered remote task unexpectedly recovered")
	}
}

func TestBacklogInspectRejectsSelectionFailuresWithoutDispatchArtifact(t *testing.T) {
	validIndex := `schema: 1
description: Fixture open-only backlog.
tasks:
  - id: TASK-015
    topic: Backlog remote fixture
    path: docs/agents/tasks/backlog-driven-remote-tasks.md
maintenance:
  - Keep open-only.
`
	tests := []struct {
		name     string
		index    string
		taskID   string
		slice    string
		baseline string
		taskDoc  string
		wantCode string
	}{
		{name: "absent id", index: validIndex, taskID: "", slice: "Implement the bounded fixture slice.", wantCode: "task_id_required"},
		{name: "malformed id", index: validIndex, taskID: "TASK-15", slice: "Implement the bounded fixture slice.", wantCode: "malformed_task_id"},
		{name: "unknown id", index: validIndex, taskID: "TASK-014", slice: "Implement the bounded fixture slice.", wantCode: "unknown_task_id"},
		{name: "duplicate", index: strings.Replace(validIndex, "maintenance:", "  - id: TASK-015\n    topic: Duplicate\n    path: docs/agents/tasks/backlog-driven-remote-tasks.md\nmaintenance:", 1), taskID: "TASK-015", slice: "Implement the bounded fixture slice.", wantCode: "ambiguous_task_id"},
		{name: "ambiguous linked path", index: strings.Replace(validIndex, "maintenance:", "  - id: TASK-014\n    topic: Same linked task\n    path: docs/agents/tasks/backlog-driven-remote-tasks.md\nmaintenance:", 1), taskID: "TASK-015", slice: "Implement the bounded fixture slice.", wantCode: "ambiguous_linked_task"},
		{name: "malformed entry", index: strings.Replace(validIndex, "    topic: Backlog remote fixture\n", "", 1), taskID: "TASK-015", slice: "Implement the bounded fixture slice.", wantCode: "malformed_index_entry"},
		{name: "missing link", index: strings.Replace(validIndex, "backlog-driven-remote-tasks.md", "missing.md", 1), taskID: "TASK-015", slice: "Implement the bounded fixture slice.", wantCode: "invalid_linked_task"},
		{name: "escaping link", index: strings.Replace(validIndex, "docs/agents/tasks/backlog-driven-remote-tasks.md", "docs/agents/tasks/../../../outside.md", 1), taskID: "TASK-015", slice: "Implement the bounded fixture slice.", wantCode: "malformed_index_entry"},
		{name: "mismatched link", index: validIndex, taskID: "TASK-015", slice: "Implement the bounded fixture slice.", taskDoc: "# TASK-014 Wrong task\n", wantCode: "mismatched_linked_task"},
		{name: "closed path", index: strings.Replace(validIndex, "docs/agents/tasks/backlog-driven-remote-tasks.md", "docs/agents/tasks/closed/backlog-driven-remote-tasks.md", 1), taskID: "TASK-015", slice: "Implement the bounded fixture slice.", wantCode: "task_closed"},
		{name: "blocked fixture", index: strings.Replace(validIndex, "    path: docs/agents/tasks/backlog-driven-remote-tasks.md", "    path: docs/agents/tasks/backlog-driven-remote-tasks.md\n    dispatch_state: blocked", 1), taskID: "TASK-015", slice: "Implement the bounded fixture slice.", wantCode: "task_blocked"},
		{name: "externally active fixture", index: strings.Replace(validIndex, "    path: docs/agents/tasks/backlog-driven-remote-tasks.md", "    path: docs/agents/tasks/backlog-driven-remote-tasks.md\n    dispatch_state: external_active", 1), taskID: "TASK-015", slice: "Implement the bounded fixture slice.", wantCode: "task_externally_active"},
		{name: "empty slice", index: validIndex, taskID: "TASK-015", slice: "", wantCode: "invalid_bounded_slice"},
		{name: "padded slice", index: validIndex, taskID: "TASK-015", slice: " Implement the bounded fixture slice. ", wantCode: "invalid_bounded_slice"},
		{name: "multiline slice", index: validIndex, taskID: "TASK-015", slice: "Implement this slice.\nThen widen it.", wantCode: "invalid_bounded_slice"},
		{name: "invalid baseline", index: validIndex, taskID: "TASK-015", slice: "Implement the bounded fixture slice.", baseline: "not-a-commit", wantCode: "invalid_baseline"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repo := writeBacklogTestRepo(t)
			writeBacklogTestFile(t, filepath.Join(repo, filepath.FromSlash(backlogIndexPath)), test.index)
			if test.taskDoc != "" {
				writeBacklogTestFile(t, filepath.Join(repo, "docs", "agents", "tasks", "backlog-driven-remote-tasks.md"), test.taskDoc)
			}
			root := filepath.Join(t.TempDir(), "artifacts")
			baseline := test.baseline
			if baseline == "" {
				baseline = backlogTestBaseline
			}
			err := inspectBacklogSelection(repo, backlogIndexPath, test.taskID, test.slice, baseline, root)
			if err == nil || !strings.HasPrefix(err.Error(), test.wantCode+":") {
				t.Fatalf("error = %v, want code %s", err, test.wantCode)
			}
			selection := readJSONMap(filepath.Join(root, "backlog-selection.json"))
			if stringValue(mapValue(selection["rejection"])["code"]) != test.wantCode {
				t.Fatalf("rejection artifact = %#v", selection)
			}
			for _, name := range []string{"remote-request.json", "remote-request.md", "remote-task.json"} {
				if _, statErr := os.Stat(filepath.Join(root, name)); !os.IsNotExist(statErr) {
					t.Fatalf("rejected selection left %s", name)
				}
			}
		})
	}
}

func TestBacklogCompatibilityRejectsMalformedContractWithoutDispatchArtifact(t *testing.T) {
	repo := writeBacklogTestRepo(t)
	root := filepath.Join(t.TempDir(), "artifacts")
	if err := inspectBacklogSelection(repo, backlogIndexPath, "TASK-015", "Implement only the bounded compatibility preflight slice.", backlogTestBaseline, root); err != nil {
		t.Fatal(err)
	}
	if err := compileBacklogRemoteRequest(repo, root, "fixture-environment", "js/dev", `["packages/dorkpipe"]`, `["No live provider"]`, `["go test ./packages/dorkpipe/lib/orchestrationhelper"]`, `[]`); err != nil {
		t.Fatal(err)
	}
	fixtureRoot := t.TempDir()
	writeBacklogTestFile(t, filepath.Join(fixtureRoot, "contract.json"), "{}\n")
	if err := preflightBacklogRemoteCompatibility(root, fixtureRoot); err == nil {
		t.Fatal("malformed compatibility contract unexpectedly passed")
	}
	compatibility := readJSONMap(filepath.Join(root, "remote-adapter-compatibility.json"))
	status := mapValue(compatibility["compatibility"])
	if stringValue(status["status"]) != "error" || stringValue(status["reason_code"]) != "invalid_compatibility_fixture" {
		t.Fatalf("unexpected compatibility failure artifact: %#v", compatibility)
	}
	if _, err := os.Stat(filepath.Join(root, "remote-task.json")); !os.IsNotExist(err) {
		t.Fatalf("malformed compatibility contract left remote-task.json: %v", err)
	}
}

func TestBacklogFollowupRejectsTamperedImmutableRequest(t *testing.T) {
	repo := writeBacklogTestRepo(t)
	root := filepath.Join(t.TempDir(), "artifacts")
	if err := inspectBacklogSelection(repo, backlogIndexPath, "TASK-015", "Implement only the bounded offline fixture dispatch slice.", backlogTestBaseline, root); err != nil {
		t.Fatal(err)
	}
	if err := compileBacklogRemoteRequest(repo, root, "fixture-environment", "js/dev", `["packages/dorkpipe"]`, `["No live provider"]`, `["go test ./packages/dorkpipe/lib/orchestrationhelper"]`, `[]`); err != nil {
		t.Fatal(err)
	}
	requestPath := filepath.Join(root, "remote-request.json")
	request := readJSONMap(requestPath)
	request["target"] = map[string]any{"environment_ref": "tampered", "branch_ref": "js/dev"}
	raw, err := json.MarshalIndent(request, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	writeBacklogTestFile(t, requestPath, string(raw)+"\n")
	if _, _, err := loadAndVerifyBacklogRequest(root); err == nil {
		t.Fatal("tampered immutable request unexpectedly validated")
	}
}

func TestBacklogCompletionCandidateRejectsDuplicateAndReplayWithoutMutation(t *testing.T) {
	root := prepareBacklogCompletionTest(t)
	acceptedFixture := writeBacklogCompletionFixture(t, root, "completion_fixture_candidate_015", "completion_fixture_replay_015", "2026-07-19T00:01:00Z")
	if err := ingestBacklogCompletionCandidate(root, acceptedFixture); err != nil {
		t.Fatal(err)
	}
	candidatePath := filepath.Join(root, "completion-candidate.json")
	acceptedRaw := mustReadFile(t, candidatePath)
	dispatchRaw := mustReadFile(t, filepath.Join(root, "remote-task.json"))

	if err := ingestBacklogCompletionCandidate(root, acceptedFixture); err == nil || !strings.HasPrefix(err.Error(), "completion_candidate_duplicate:") {
		t.Fatalf("duplicate error = %v", err)
	}
	replayFixture := writeBacklogCompletionFixture(t, root, "completion_fixture_candidate_016", "completion_fixture_replay_015", "2026-07-19T00:02:00Z")
	if err := ingestBacklogCompletionCandidate(root, replayFixture); err == nil || !strings.HasPrefix(err.Error(), "completion_candidate_replay:") {
		t.Fatalf("replay error = %v", err)
	}
	if string(mustReadFile(t, candidatePath)) != string(acceptedRaw) {
		t.Fatal("duplicate or replay rejection changed the accepted completion candidate")
	}
	if string(mustReadFile(t, filepath.Join(root, "remote-task.json"))) != string(dispatchRaw) {
		t.Fatal("duplicate or replay rejection changed the accepted dispatch identity")
	}
}

func TestBacklogCompletionCandidateRejectsStaleMismatchedMalformedAndTamperedEvidence(t *testing.T) {
	tests := []struct {
		name     string
		wantCode string
		mutate   func(t *testing.T, root, fixturePath string)
	}{
		{name: "stale", wantCode: "completion_candidate_stale", mutate: func(t *testing.T, _, fixturePath string) {
			mutateBacklogJSONFile(t, fixturePath, func(payload map[string]any) { payload["observed_at"] = "2026-07-19T00:00:00Z" })
		}},
		{name: "wrong remote task", wantCode: "completion_candidate_binding_mismatch", mutate: func(t *testing.T, _, fixturePath string) {
			mutateBacklogJSONFile(t, fixturePath, func(payload map[string]any) { payload["remote_task_id"] = "remote_fixture_task_wrong" })
		}},
		{name: "wrong request", wantCode: "completion_candidate_binding_mismatch", mutate: func(t *testing.T, _, fixturePath string) {
			mutateBacklogJSONFile(t, fixturePath, func(payload map[string]any) { payload["request_fingerprint"] = "sha256:" + strings.Repeat("0", 64) })
		}},
		{name: "wrong dispatch", wantCode: "completion_candidate_binding_mismatch", mutate: func(t *testing.T, _, fixturePath string) {
			mutateBacklogJSONFile(t, fixturePath, func(payload map[string]any) { payload["dispatch_fingerprint"] = "sha256:" + strings.Repeat("1", 64) })
		}},
		{name: "wrong adapter", wantCode: "completion_candidate_binding_mismatch", mutate: func(t *testing.T, _, fixturePath string) {
			mutateBacklogJSONFile(t, fixturePath, func(payload map[string]any) { payload["adapter_identity"] = "codex-cloud-fixture-wrong" })
		}},
		{name: "wrong environment", wantCode: "completion_candidate_binding_mismatch", mutate: func(t *testing.T, _, fixturePath string) {
			mutateBacklogJSONFile(t, fixturePath, func(payload map[string]any) { payload["environment_ref"] = "wrong-environment" })
		}},
		{name: "wrong branch", wantCode: "completion_candidate_binding_mismatch", mutate: func(t *testing.T, _, fixturePath string) {
			mutateBacklogJSONFile(t, fixturePath, func(payload map[string]any) { payload["branch_ref"] = "wrong/branch" })
		}},
		{name: "malformed fixture", wantCode: "completion_candidate_fixture_malformed", mutate: func(t *testing.T, _, fixturePath string) {
			writeBacklogTestFile(t, fixturePath, "{\"unexpected\":true}\n")
		}},
		{name: "tampered request", wantCode: "completion_candidate_request_invalid", mutate: func(t *testing.T, root, _ string) {
			mutateBacklogJSONFile(t, filepath.Join(root, "remote-request.json"), func(payload map[string]any) { payload["request_fingerprint"] = "sha256:" + strings.Repeat("2", 64) })
		}},
		{name: "tampered compatibility", wantCode: "completion_candidate_compatibility_invalid", mutate: func(t *testing.T, root, _ string) {
			mutateBacklogJSONFile(t, filepath.Join(root, "remote-adapter-compatibility.json"), func(payload map[string]any) { payload["adapter_identity"] = "tampered-adapter" })
		}},
		{name: "tampered dispatch", wantCode: "completion_candidate_dispatch_invalid", mutate: func(t *testing.T, root, _ string) {
			mutateBacklogJSONFile(t, filepath.Join(root, "remote-task.json"), func(payload map[string]any) { payload["remote_task_id"] = "remote_fixture_task_tampered" })
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := prepareBacklogCompletionTest(t)
			fixturePath := writeBacklogCompletionFixture(t, root, "completion_fixture_candidate_015", "completion_fixture_replay_015", "2026-07-19T00:01:00Z")
			test.mutate(t, root, fixturePath)
			dispatchBefore := mustReadFile(t, filepath.Join(root, "remote-task.json"))
			err := ingestBacklogCompletionCandidate(root, fixturePath)
			if err == nil || !strings.HasPrefix(err.Error(), test.wantCode+":") {
				t.Fatalf("error = %v, want code %s", err, test.wantCode)
			}
			if _, statErr := os.Stat(filepath.Join(root, "completion-candidate.json")); !os.IsNotExist(statErr) {
				t.Fatalf("rejected candidate left completion-candidate.json: %v", statErr)
			}
			if string(mustReadFile(t, filepath.Join(root, "remote-task.json"))) != string(dispatchBefore) {
				t.Fatal("rejected candidate changed the dispatch artifact")
			}
			for _, name := range []string{"ready-for-review.json", "remote-status.json", "remote-diff.patch", "remote-result.json", "validation-receipt.json", "apply.json"} {
				if _, statErr := os.Stat(filepath.Join(root, name)); !os.IsNotExist(statErr) {
					t.Fatalf("rejected candidate left forbidden artifact %s", name)
				}
			}
		})
	}
}

func TestBacklogRemoteStatusRejectsDuplicateAndReplayWithoutMutation(t *testing.T) {
	root := prepareBacklogStatusTest(t)
	acceptedFixture := writeBacklogStatusFixture(t, root, "status_fixture_observation_015", "status_fixture_replay_015", "2026-07-19T00:02:00Z")
	if err := retrieveBacklogRemoteStatusFixture(root, acceptedFixture); err != nil {
		t.Fatal(err)
	}
	statusPath := filepath.Join(root, "remote-status.json")
	acceptedStatus := mustReadFile(t, statusPath)
	acceptedCandidate := mustReadFile(t, filepath.Join(root, "completion-candidate.json"))
	acceptedDispatch := mustReadFile(t, filepath.Join(root, "remote-task.json"))

	if err := retrieveBacklogRemoteStatusFixture(root, acceptedFixture); err == nil || !strings.HasPrefix(err.Error(), "remote_status_duplicate:") {
		t.Fatalf("duplicate error = %v", err)
	}
	replayFixture := writeBacklogStatusFixture(t, root, "status_fixture_observation_016", "status_fixture_replay_015", "2026-07-19T00:03:00Z")
	if err := retrieveBacklogRemoteStatusFixture(root, replayFixture); err == nil || !strings.HasPrefix(err.Error(), "remote_status_replay:") {
		t.Fatalf("replay error = %v", err)
	}
	if string(mustReadFile(t, statusPath)) != string(acceptedStatus) {
		t.Fatal("duplicate or replay rejection changed the accepted remote status")
	}
	if string(mustReadFile(t, filepath.Join(root, "completion-candidate.json"))) != string(acceptedCandidate) {
		t.Fatal("duplicate or replay rejection changed the accepted completion candidate")
	}
	if string(mustReadFile(t, filepath.Join(root, "remote-task.json"))) != string(acceptedDispatch) {
		t.Fatal("duplicate or replay rejection changed the accepted dispatch identity")
	}
}

func TestBacklogRemoteStatusRejectsStaleMismatchedMalformedAndTamperedEvidence(t *testing.T) {
	tests := []struct {
		name     string
		wantCode string
		mutate   func(t *testing.T, root, fixturePath string)
	}{
		{name: "stale at candidate time", wantCode: "remote_status_stale", mutate: func(t *testing.T, _, fixturePath string) {
			mutateBacklogJSONFile(t, fixturePath, func(payload map[string]any) { payload["observed_at"] = "2026-07-19T00:01:00Z" })
		}},
		{name: "wrong candidate", wantCode: "remote_status_binding_mismatch", mutate: func(t *testing.T, _, fixturePath string) {
			mutateBacklogJSONFile(t, fixturePath, func(payload map[string]any) {
				payload["completion_candidate_id"] = "completion_fixture_candidate_wrong"
			})
		}},
		{name: "wrong candidate fingerprint", wantCode: "remote_status_binding_mismatch", mutate: func(t *testing.T, _, fixturePath string) {
			mutateBacklogJSONFile(t, fixturePath, func(payload map[string]any) {
				payload["completion_candidate_fingerprint"] = "sha256:" + strings.Repeat("0", 64)
			})
		}},
		{name: "wrong remote task", wantCode: "remote_status_binding_mismatch", mutate: func(t *testing.T, _, fixturePath string) {
			mutateBacklogJSONFile(t, fixturePath, func(payload map[string]any) { payload["remote_task_id"] = "remote_fixture_task_wrong" })
		}},
		{name: "wrong request", wantCode: "remote_status_binding_mismatch", mutate: func(t *testing.T, _, fixturePath string) {
			mutateBacklogJSONFile(t, fixturePath, func(payload map[string]any) { payload["request_fingerprint"] = "sha256:" + strings.Repeat("1", 64) })
		}},
		{name: "wrong dispatch", wantCode: "remote_status_binding_mismatch", mutate: func(t *testing.T, _, fixturePath string) {
			mutateBacklogJSONFile(t, fixturePath, func(payload map[string]any) { payload["dispatch_fingerprint"] = "sha256:" + strings.Repeat("2", 64) })
		}},
		{name: "wrong adapter", wantCode: "remote_status_binding_mismatch", mutate: func(t *testing.T, _, fixturePath string) {
			mutateBacklogJSONFile(t, fixturePath, func(payload map[string]any) { payload["adapter_identity"] = "codex-cloud-fixture-wrong" })
		}},
		{name: "wrong environment", wantCode: "remote_status_binding_mismatch", mutate: func(t *testing.T, _, fixturePath string) {
			mutateBacklogJSONFile(t, fixturePath, func(payload map[string]any) { payload["environment_ref"] = "wrong-environment" })
		}},
		{name: "wrong branch", wantCode: "remote_status_binding_mismatch", mutate: func(t *testing.T, _, fixturePath string) {
			mutateBacklogJSONFile(t, fixturePath, func(payload map[string]any) { payload["branch_ref"] = "wrong/branch" })
		}},
		{name: "tampered status claim", wantCode: "remote_status_claim_invalid", mutate: func(t *testing.T, _, fixturePath string) {
			mutateBacklogJSONFile(t, fixturePath, func(payload map[string]any) { payload["claimed_remote_status"] = "ready_for_review" })
		}},
		{name: "malformed fixture", wantCode: "remote_status_fixture_malformed", mutate: func(t *testing.T, _, fixturePath string) {
			writeBacklogTestFile(t, fixturePath, "{\"unexpected\":true}\n")
		}},
		{name: "tampered request", wantCode: "remote_status_request_invalid", mutate: func(t *testing.T, root, _ string) {
			mutateBacklogJSONFile(t, filepath.Join(root, "remote-request.json"), func(payload map[string]any) { payload["request_fingerprint"] = "sha256:" + strings.Repeat("3", 64) })
		}},
		{name: "tampered compatibility", wantCode: "remote_status_compatibility_invalid", mutate: func(t *testing.T, root, _ string) {
			mutateBacklogJSONFile(t, filepath.Join(root, "remote-adapter-compatibility.json"), func(payload map[string]any) { payload["adapter_identity"] = "tampered-adapter" })
		}},
		{name: "tampered dispatch", wantCode: "remote_status_dispatch_invalid", mutate: func(t *testing.T, root, _ string) {
			mutateBacklogJSONFile(t, filepath.Join(root, "remote-task.json"), func(payload map[string]any) { payload["remote_task_id"] = "remote_fixture_task_tampered" })
		}},
		{name: "tampered candidate state", wantCode: "remote_status_candidate_invalid", mutate: func(t *testing.T, root, _ string) {
			mutateBacklogJSONFile(t, filepath.Join(root, "completion-candidate.json"), func(payload map[string]any) { payload["state"] = "ready_for_review" })
		}},
		{name: "tampered candidate lifecycle", wantCode: "remote_status_candidate_invalid", mutate: func(t *testing.T, root, _ string) {
			mutateBacklogJSONFile(t, filepath.Join(root, "completion-candidate.json"), func(payload map[string]any) { mapValue(payload["lifecycle"])["ready_for_review"] = true })
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := prepareBacklogStatusTest(t)
			fixturePath := writeBacklogStatusFixture(t, root, "status_fixture_observation_015", "status_fixture_replay_015", "2026-07-19T00:02:00Z")
			test.mutate(t, root, fixturePath)
			candidateBefore := mustReadFile(t, filepath.Join(root, "completion-candidate.json"))
			dispatchBefore := mustReadFile(t, filepath.Join(root, "remote-task.json"))
			err := retrieveBacklogRemoteStatusFixture(root, fixturePath)
			if err == nil || !strings.HasPrefix(err.Error(), test.wantCode+":") {
				t.Fatalf("error = %v, want code %s", err, test.wantCode)
			}
			if _, statErr := os.Stat(filepath.Join(root, "remote-status.json")); !os.IsNotExist(statErr) {
				t.Fatalf("rejected status observation left remote-status.json: %v", statErr)
			}
			if string(mustReadFile(t, filepath.Join(root, "completion-candidate.json"))) != string(candidateBefore) {
				t.Fatal("rejected status observation changed the accepted completion candidate")
			}
			if string(mustReadFile(t, filepath.Join(root, "remote-task.json"))) != string(dispatchBefore) {
				t.Fatal("rejected status observation changed the dispatch artifact")
			}
			for _, name := range []string{"ready-for-review.json", "remote-diff.patch", "remote-result.json", "validation-receipt.json", "apply.json"} {
				if _, statErr := os.Stat(filepath.Join(root, name)); !os.IsNotExist(statErr) {
					t.Fatalf("rejected status observation left forbidden artifact %s", name)
				}
			}
		})
	}
}

func TestBacklogRemoteDiffRejectsDuplicateAndReplayWithoutMutation(t *testing.T) {
	root := prepareBacklogDiffTest(t)
	acceptedFixture := writeBacklogDiffFixture(t, root, "diff_fixture_observation_015", "diff_fixture_replay_015", "2026-07-19T00:03:00Z", backlogTestPatch)
	if err := retrieveBacklogRemoteDiffFixture(root, acceptedFixture); err != nil {
		t.Fatal(err)
	}
	accepted := map[string][]byte{}
	for _, name := range []string{"remote-diff.json", "remote-diff.patch", "remote-status.json", "completion-candidate.json", "remote-task.json"} {
		accepted[name] = mustReadFile(t, filepath.Join(root, name))
	}
	if err := retrieveBacklogRemoteDiffFixture(root, acceptedFixture); err == nil || !strings.HasPrefix(err.Error(), "remote_diff_duplicate:") {
		t.Fatalf("duplicate error = %v", err)
	}
	replayFixture := writeBacklogDiffFixture(t, root, "diff_fixture_observation_016", "diff_fixture_replay_015", "2026-07-19T00:04:00Z", backlogTestPatch)
	if err := retrieveBacklogRemoteDiffFixture(root, replayFixture); err == nil || !strings.HasPrefix(err.Error(), "remote_diff_replay:") {
		t.Fatalf("replay error = %v", err)
	}
	for name, before := range accepted {
		if string(mustReadFile(t, filepath.Join(root, name))) != string(before) {
			t.Fatalf("duplicate or replay rejection changed %s", name)
		}
	}
}

func TestBacklogRemoteDiffRejectsStaleMismatchedMalformedMissingAndTamperedEvidence(t *testing.T) {
	tests := []struct {
		name     string
		wantCode string
		mutate   func(t *testing.T, root, fixturePath string)
	}{
		{name: "stale at status time", wantCode: "remote_diff_stale", mutate: func(t *testing.T, _, fixturePath string) {
			mutateBacklogJSONFile(t, fixturePath, func(payload map[string]any) { payload["observed_at"] = "2026-07-19T00:02:00Z" })
		}},
		{name: "wrong status observation", wantCode: "remote_diff_binding_mismatch", mutate: func(t *testing.T, _, fixturePath string) {
			mutateBacklogJSONFile(t, fixturePath, func(payload map[string]any) {
				payload["remote_status_observation_id"] = "status_fixture_observation_wrong"
			})
		}},
		{name: "wrong status fingerprint", wantCode: "remote_diff_binding_mismatch", mutate: func(t *testing.T, _, fixturePath string) {
			mutateBacklogJSONFile(t, fixturePath, func(payload map[string]any) {
				payload["remote_status_fingerprint"] = "sha256:" + strings.Repeat("0", 64)
			})
		}},
		{name: "wrong candidate", wantCode: "remote_diff_binding_mismatch", mutate: func(t *testing.T, _, fixturePath string) {
			mutateBacklogJSONFile(t, fixturePath, func(payload map[string]any) {
				payload["completion_candidate_id"] = "completion_fixture_candidate_wrong"
			})
		}},
		{name: "wrong candidate fingerprint", wantCode: "remote_diff_binding_mismatch", mutate: func(t *testing.T, _, fixturePath string) {
			mutateBacklogJSONFile(t, fixturePath, func(payload map[string]any) {
				payload["completion_candidate_fingerprint"] = "sha256:" + strings.Repeat("1", 64)
			})
		}},
		{name: "wrong remote task", wantCode: "remote_diff_binding_mismatch", mutate: func(t *testing.T, _, fixturePath string) {
			mutateBacklogJSONFile(t, fixturePath, func(payload map[string]any) { payload["remote_task_id"] = "remote_fixture_task_wrong" })
		}},
		{name: "wrong request", wantCode: "remote_diff_binding_mismatch", mutate: func(t *testing.T, _, fixturePath string) {
			mutateBacklogJSONFile(t, fixturePath, func(payload map[string]any) { payload["request_fingerprint"] = "sha256:" + strings.Repeat("2", 64) })
		}},
		{name: "wrong dispatch", wantCode: "remote_diff_binding_mismatch", mutate: func(t *testing.T, _, fixturePath string) {
			mutateBacklogJSONFile(t, fixturePath, func(payload map[string]any) { payload["dispatch_fingerprint"] = "sha256:" + strings.Repeat("3", 64) })
		}},
		{name: "wrong adapter", wantCode: "remote_diff_binding_mismatch", mutate: func(t *testing.T, _, fixturePath string) {
			mutateBacklogJSONFile(t, fixturePath, func(payload map[string]any) { payload["adapter_identity"] = "codex-cloud-fixture-wrong" })
		}},
		{name: "wrong environment", wantCode: "remote_diff_binding_mismatch", mutate: func(t *testing.T, _, fixturePath string) {
			mutateBacklogJSONFile(t, fixturePath, func(payload map[string]any) { payload["environment_ref"] = "wrong-environment" })
		}},
		{name: "wrong branch", wantCode: "remote_diff_binding_mismatch", mutate: func(t *testing.T, _, fixturePath string) {
			mutateBacklogJSONFile(t, fixturePath, func(payload map[string]any) { payload["branch_ref"] = "wrong/branch" })
		}},
		{name: "malformed fixture", wantCode: "remote_diff_fixture_malformed", mutate: func(t *testing.T, _, fixturePath string) {
			writeBacklogTestFile(t, fixturePath, "{\"unexpected\":true}\n")
		}},
		{name: "missing fixture", wantCode: "remote_diff_fixture_missing", mutate: func(t *testing.T, _, fixturePath string) {
			if err := os.Remove(fixturePath); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "missing patch", wantCode: "remote_diff_patch_missing", mutate: func(t *testing.T, _, fixturePath string) {
			if err := os.Remove(filepath.Join(filepath.Dir(fixturePath), "remote-diff.patch")); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "tampered patch bytes", wantCode: "remote_diff_patch_tampered", mutate: func(t *testing.T, _, fixturePath string) {
			writeBacklogTestFile(t, filepath.Join(filepath.Dir(fixturePath), "remote-diff.patch"), backlogTestPatch+"tampered\n")
		}},
		{name: "tampered request", wantCode: "remote_diff_request_invalid", mutate: func(t *testing.T, root, _ string) {
			mutateBacklogJSONFile(t, filepath.Join(root, "remote-request.json"), func(payload map[string]any) { payload["request_fingerprint"] = "sha256:" + strings.Repeat("4", 64) })
		}},
		{name: "tampered compatibility", wantCode: "remote_diff_compatibility_invalid", mutate: func(t *testing.T, root, _ string) {
			mutateBacklogJSONFile(t, filepath.Join(root, "remote-adapter-compatibility.json"), func(payload map[string]any) { payload["adapter_identity"] = "tampered-adapter" })
		}},
		{name: "tampered dispatch", wantCode: "remote_diff_dispatch_invalid", mutate: func(t *testing.T, root, _ string) {
			mutateBacklogJSONFile(t, filepath.Join(root, "remote-task.json"), func(payload map[string]any) { payload["remote_task_id"] = "remote_fixture_task_tampered" })
		}},
		{name: "tampered candidate", wantCode: "remote_diff_candidate_invalid", mutate: func(t *testing.T, root, _ string) {
			mutateBacklogJSONFile(t, filepath.Join(root, "completion-candidate.json"), func(payload map[string]any) { payload["state"] = "ready_for_review" })
		}},
		{name: "tampered status", wantCode: "remote_diff_status_invalid", mutate: func(t *testing.T, root, _ string) {
			mutateBacklogJSONFile(t, filepath.Join(root, "remote-status.json"), func(payload map[string]any) { mapValue(payload["lifecycle"])["ready_for_review"] = true })
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := prepareBacklogDiffTest(t)
			fixturePath := writeBacklogDiffFixture(t, root, "diff_fixture_observation_015", "diff_fixture_replay_015", "2026-07-19T00:03:00Z", backlogTestPatch)
			test.mutate(t, root, fixturePath)
			before := map[string][]byte{}
			for _, name := range []string{"remote-status.json", "completion-candidate.json", "remote-task.json"} {
				before[name] = mustReadFile(t, filepath.Join(root, name))
			}
			err := retrieveBacklogRemoteDiffFixture(root, fixturePath)
			if err == nil || !strings.HasPrefix(err.Error(), test.wantCode+":") {
				t.Fatalf("error = %v, want code %s", err, test.wantCode)
			}
			for _, name := range []string{"remote-diff.json", "remote-diff.patch", "ready-for-review.json", "remote-result.json", "validation-receipt.json", "apply.json"} {
				if _, statErr := os.Stat(filepath.Join(root, name)); !os.IsNotExist(statErr) {
					t.Fatalf("rejected diff observation left forbidden artifact %s: %v", name, statErr)
				}
			}
			for name, content := range before {
				if string(mustReadFile(t, filepath.Join(root, name))) != string(content) {
					t.Fatalf("rejected diff observation changed %s", name)
				}
			}
		})
	}
}

func prepareBacklogCompletionTest(t *testing.T) string {
	t.Helper()
	repo := writeBacklogTestRepo(t)
	root := filepath.Join(t.TempDir(), "artifacts")
	if err := inspectBacklogSelection(repo, backlogIndexPath, "TASK-015", "Implement only the bounded completion candidate slice.", backlogTestBaseline, root); err != nil {
		t.Fatal(err)
	}
	if err := compileBacklogRemoteRequest(repo, root, "fixture-environment", "js/dev", `["packages/dorkpipe"]`, `["No live provider"]`, `["go test ./packages/dorkpipe/lib/orchestrationhelper"]`, `[]`); err != nil {
		t.Fatal(err)
	}
	if err := preflightBacklogRemoteCompatibility(root, writeBacklogCompatibilityFixture(t)); err != nil {
		t.Fatal(err)
	}
	dispatchFixture := filepath.Join(t.TempDir(), "dispatch.json")
	writeBacklogTestFile(t, dispatchFixture, `{
  "contract_version": "dorkpipe.remote-dispatch-fixture/v1",
  "adapter_identity": "codex-cloud-fixture-v1",
  "remote_task_id": "remote_fixture_task_015",
  "submitted_at": "2026-07-19T00:00:00Z"
}`)
	if err := dispatchBacklogFixture(root, dispatchFixture); err != nil {
		t.Fatal(err)
	}
	return root
}

func prepareBacklogStatusTest(t *testing.T) string {
	t.Helper()
	root := prepareBacklogCompletionTest(t)
	fixture := writeBacklogCompletionFixture(t, root, "completion_fixture_candidate_015", "completion_fixture_replay_015", "2026-07-19T00:01:00Z")
	if err := ingestBacklogCompletionCandidate(root, fixture); err != nil {
		t.Fatal(err)
	}
	return root
}

func prepareBacklogDiffTest(t *testing.T) string {
	t.Helper()
	root := prepareBacklogStatusTest(t)
	fixture := writeBacklogStatusFixture(t, root, "status_fixture_observation_015", "status_fixture_replay_015", "2026-07-19T00:02:00Z")
	if err := retrieveBacklogRemoteStatusFixture(root, fixture); err != nil {
		t.Fatal(err)
	}
	return root
}

func writeBacklogCompletionFixture(t *testing.T, root, candidateID, replayIdentity, observedAt string) string {
	t.Helper()
	task := readJSONMap(filepath.Join(root, "remote-task.json"))
	target := mapValue(task["target"])
	adapter := mapValue(task["adapter"])
	payload := backlogCompletionFixture{
		ContractVersion: backlogCompletionFixtureContract, CandidateID: candidateID, ReplayIdentity: replayIdentity,
		AdapterIdentity: stringValue(adapter["identity"]), RemoteTaskID: stringValue(task["remote_task_id"]),
		RequestFingerprint: stringValue(task["request_fingerprint"]), DispatchFingerprint: stringValue(task["dispatch_fingerprint"]),
		EnvironmentRef: stringValue(target["environment_ref"]), BranchRef: stringValue(target["branch_ref"]),
		ObservedAt: observedAt, ClaimedTerminalSignal: "completed",
	}
	raw, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "completion-candidate.json")
	writeBacklogTestFile(t, path, string(raw)+"\n")
	return path
}

func writeBacklogStatusFixture(t *testing.T, root, observationID, replayIdentity, observedAt string) string {
	t.Helper()
	task := readJSONMap(filepath.Join(root, "remote-task.json"))
	target := mapValue(task["target"])
	adapter := mapValue(task["adapter"])
	candidate := readJSONMap(filepath.Join(root, "completion-candidate.json"))
	candidateFingerprint, err := backlogJSONFingerprint(candidate)
	if err != nil {
		t.Fatal(err)
	}
	payload := backlogStatusFixture{
		ContractVersion: backlogStatusFixtureContract, ObservationID: observationID, ReplayIdentity: replayIdentity,
		CompletionCandidateID: stringValue(mapValue(candidate["identity"])["candidate_id"]), CompletionCandidateFingerprint: candidateFingerprint,
		AdapterIdentity: stringValue(adapter["identity"]), RemoteTaskID: stringValue(task["remote_task_id"]),
		RequestFingerprint: stringValue(task["request_fingerprint"]), DispatchFingerprint: stringValue(task["dispatch_fingerprint"]),
		EnvironmentRef: stringValue(target["environment_ref"]), BranchRef: stringValue(target["branch_ref"]),
		ObservedAt: observedAt, ClaimedRemoteStatus: "completed",
	}
	raw, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "remote-status.json")
	writeBacklogTestFile(t, path, string(raw)+"\n")
	return path
}

func writeBacklogDiffFixture(t *testing.T, root, observationID, replayIdentity, observedAt, patch string) string {
	t.Helper()
	task := readJSONMap(filepath.Join(root, "remote-task.json"))
	target := mapValue(task["target"])
	adapter := mapValue(task["adapter"])
	candidate := readJSONMap(filepath.Join(root, "completion-candidate.json"))
	candidateFingerprint, err := backlogJSONFingerprint(candidate)
	if err != nil {
		t.Fatal(err)
	}
	status := readJSONMap(filepath.Join(root, "remote-status.json"))
	statusFingerprint, err := backlogJSONFingerprint(status)
	if err != nil {
		t.Fatal(err)
	}
	payload := backlogDiffFixture{
		ContractVersion: backlogDiffFixtureContract, ObservationID: observationID, ReplayIdentity: replayIdentity,
		RemoteStatusObservationID: stringValue(mapValue(status["identity"])["observation_id"]), RemoteStatusFingerprint: statusFingerprint,
		CompletionCandidateID: stringValue(mapValue(candidate["identity"])["candidate_id"]), CompletionCandidateFingerprint: candidateFingerprint,
		AdapterIdentity: stringValue(adapter["identity"]), RemoteTaskID: stringValue(task["remote_task_id"]),
		RequestFingerprint: stringValue(task["request_fingerprint"]), DispatchFingerprint: stringValue(task["dispatch_fingerprint"]),
		EnvironmentRef: stringValue(target["environment_ref"]), BranchRef: stringValue(target["branch_ref"]),
		ObservedAt: observedAt, PatchFile: "remote-diff.patch", PatchSHA256: sha256String([]byte(patch)),
	}
	raw, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	rootPath := t.TempDir()
	path := filepath.Join(rootPath, "remote-diff.json")
	writeBacklogTestFile(t, path, string(raw)+"\n")
	writeBacklogTestFile(t, filepath.Join(rootPath, "remote-diff.patch"), patch)
	return path
}

func mutateBacklogJSONFile(t *testing.T, path string, mutate func(map[string]any)) {
	t.Helper()
	payload := readJSONMap(path)
	mutate(payload)
	raw, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	writeBacklogTestFile(t, path, string(raw)+"\n")
}

func writeBacklogTestRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	files := map[string]string{
		"AGENTS.md":      "# Fixture agent guidance\n",
		backlogIndexPath: "schema: 1\ndescription: Fixture open-only backlog.\ntasks:\n  - id: TASK-015\n    topic: Backlog remote fixture\n    path: docs/agents/tasks/backlog-driven-remote-tasks.md\nmaintenance:\n  - Keep open-only.\n",
		"docs/agents/tasks/backlog-driven-remote-tasks.md": "# TASK-015 Backlog remote fixture\n\nFixture task body.\n",
		"docs/agents/packages/package-authoring.md":        "# Package authoring fixture\n",
		"docs/agents/workflows/yaml-workflows.md":          "# YAML workflow fixture\n",
		"packages/dorkpipe/README.md":                      "# Fixture package\n",
	}
	for rel, content := range files {
		writeBacklogTestFile(t, filepath.Join(root, filepath.FromSlash(rel)), content)
	}
	if err := os.MkdirAll(filepath.Join(root, "packages", "dorkpipe", "lib"), 0o755); err != nil {
		t.Fatal(err)
	}
	return root
}

func writeBacklogCompatibilityFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeBacklogTestFile(t, filepath.Join(root, "contract.json"), `{
  "contract_version": "dorkpipe.codex-cloud-cli-compatibility-fixture/v1",
  "adapter_identity": "codex-cloud-cli",
  "cli": {"reference": "codex", "version": "codex-cli 0.144.1"},
  "inspected_commands": [
    {"argv": ["codex", "--version"], "fixture": "codex-version.txt"},
    {"argv": ["codex", "cloud", "--help"], "fixture": "codex-cloud-help.txt"},
    {"argv": ["codex", "cloud", "exec", "--help"], "fixture": "codex-cloud-exec-help.txt"}
  ],
  "recognized_inputs": [
    {"name": "environment", "flag": "--env", "value": "ENV_ID", "required": true},
    {"name": "branch", "flag": "--branch", "value": "BRANCH", "required": false}
  ],
  "submission_receipt": {"machine_readable_documented": false, "stable_opaque_task_id_recoverable": false},
  "exact_gap": "codex cloud exec --help for codex-cli 0.144.1 documents no machine-readable submission receipt and no stable opaque task-ID response contract."
}
`)
	writeBacklogTestFile(t, filepath.Join(root, "codex-version.txt"), "codex-cli 0.144.1\n")
	writeBacklogTestFile(t, filepath.Join(root, "codex-cloud-help.txt"), "Usage: codex cloud [OPTIONS] [COMMAND]\nexec    Submit a new Codex Cloud task without launching the TUI\n")
	writeBacklogTestFile(t, filepath.Join(root, "codex-cloud-exec-help.txt"), "Usage: codex cloud exec [OPTIONS] --env <ENV_ID> [QUERY]\n--env <ENV_ID>\n--branch <BRANCH>\n")
	return root
}

func writeBacklogTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func backlogTestBool(value any) bool {
	parsed, _ := value.(bool)
	return parsed
}
