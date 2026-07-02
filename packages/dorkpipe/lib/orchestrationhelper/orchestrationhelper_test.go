package orchestrationhelper

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestApplyTaskWorkerProfileDefaultsToPrefer(t *testing.T) {
	task, err := applyTaskWorkerProfile(map[string]any{
		"id":     "patch",
		"worker": "codex",
	}, map[string]string{})
	if err != nil {
		t.Fatal(err)
	}
	if got := stringValue(task["resolver_hint"]); got != "" {
		t.Fatalf("resolver_hint should stay empty for seeded worker profiles, got %q", got)
	}
	if got := workerPolicyMode(task); got != "prefer" {
		t.Fatalf("worker policy mode = %q, want prefer", got)
	}
	if got := stringValue(task["worker_preferred_resolver_hint"]); got != "codex" {
		t.Fatalf("worker preferred resolver = %q, want codex", got)
	}
}

func TestApplyTaskWorkerProfileEditModeDefaultsToRequire(t *testing.T) {
	task, err := applyTaskWorkerProfile(map[string]any{
		"id":        "patch",
		"worker":    "codex",
		"work_mode": "edit",
	}, map[string]string{})
	if err != nil {
		t.Fatal(err)
	}
	if got := workerPolicyMode(task); got != "require" {
		t.Fatalf("worker policy mode = %q, want require for edit mode", got)
	}
}

func TestSelectLaneWorkerPreferAllowsFallback(t *testing.T) {
	task, err := applyTaskWorkerProfile(map[string]any{
		"id":     "analysis",
		"worker": "codex",
		"model_policy": map[string]any{
			"attempt": map[string]any{
				"preference": "cheap-first",
			},
		},
	}, map[string]string{})
	if err != nil {
		t.Fatal(err)
	}
	lanes := []map[string]any{
		{
			"id":            "ollama.local.default",
			"provider":      "ollama",
			"resolver_hint": "ollama",
			"local":         true,
			"available":     true,
		},
		{
			"id":            "codex.cloud.default",
			"provider":      "codex",
			"resolver_hint": "codex",
			"cloud":         true,
			"available":     true,
		},
	}
	selection := selectLane(task, mapValue(task["model_policy"]), "", "", "", map[string]string{}, lanes, map[string]any{
		"local_first_bonus":       15.0,
		"cloud_cost_penalty":      2.0,
		"worker_preference_bonus": 10.0,
	}, map[string]any{}, map[string]trainingEntry{}, false, nil)
	if got := stringValue(selection["provider"]); got != "ollama" {
		t.Fatalf("provider = %q, want ollama fallback under prefer policy", got)
	}
}

func TestSelectLaneWorkerRequirePinsPreferredLane(t *testing.T) {
	task, err := applyTaskWorkerProfile(map[string]any{
		"id":     "patch",
		"worker": "codex",
		"worker_policy": map[string]any{
			"mode": "require",
		},
		"model_policy": map[string]any{
			"attempt": map[string]any{
				"preference": "cheap-first",
			},
		},
	}, map[string]string{})
	if err != nil {
		t.Fatal(err)
	}
	lanes := []map[string]any{
		{
			"id":            "ollama.local.default",
			"provider":      "ollama",
			"resolver_hint": "ollama",
			"local":         true,
			"available":     true,
		},
		{
			"id":            "codex.cloud.default",
			"provider":      "codex",
			"resolver_hint": "codex",
			"cloud":         true,
			"available":     true,
		},
	}
	selection := selectLane(task, mapValue(task["model_policy"]), "", "", "", map[string]string{}, lanes, map[string]any{
		"local_first_bonus":       15.0,
		"cloud_cost_penalty":      2.0,
		"worker_preference_bonus": 10.0,
	}, map[string]any{}, map[string]trainingEntry{}, true, nil)
	if got := stringValue(selection["provider"]); got != "codex" {
		t.Fatalf("provider = %q, want codex under require policy", got)
	}
	if got := stringValue(selection["requested"]); got != "codex" {
		t.Fatalf("requested = %q, want codex", got)
	}
}

func TestSelectLaneArchitectureTaskPrefersCloudOverCheapLocal(t *testing.T) {
	task, err := applyTaskWorkerProfile(map[string]any{
		"id":              "brain_contract",
		"worker":          "claude",
		"worker_type":     "architecture",
		"goal":            "Define the architecture contract and acceptance criteria",
		"expected_output": "A durable contract and routing policy",
		"model_policy": map[string]any{
			"attempt": map[string]any{
				"preference": "cheap-first",
			},
		},
	}, map[string]string{})
	if err != nil {
		t.Fatal(err)
	}
	lanes := []map[string]any{
		{
			"id":            "ollama.local.default",
			"provider":      "ollama",
			"resolver_hint": "ollama",
			"model":         "qwen2.5:7b",
			"local":         true,
			"available":     true,
		},
		{
			"id":            "claude.cloud.default",
			"provider":      "claude",
			"resolver_hint": "claude",
			"model":         "cli",
			"cloud":         true,
			"available":     true,
			"capabilities":  []any{"review", "safety", "strong_validation"},
		},
	}
	selection := selectLane(task, mapValue(task["model_policy"]), "", "", "", map[string]string{
		"DORKPIPE_ORCH_HOST_MEMORY_GB":      "16",
		"DORKPIPE_ORCH_HOST_CPU_CORES":      "8",
		"DORKPIPE_ORCH_LOCAL_ACCELERATION":  "cpu",
		"DORKPIPE_ORCH_LOCAL_HARDWARE_TIER": "low",
	}, lanes, map[string]any{
		"local_first_bonus":                 15.0,
		"cloud_cost_penalty":                2.0,
		"worker_preference_bonus":           10.0,
		"authority_cloud_bonus":             8.0,
		"local_architecture_penalty":        18.0,
		"low_tier_local_authority_penalty":  10.0,
		"architecture_keywords":             []any{"architecture", "contract", "acceptance criteria"},
		"cloud_score_threshold":             14.0,
		"high_risk_cloud_score_threshold":   10.0,
		"explicit_hint_bypasses_cloud_gate": true,
	}, map[string]any{}, map[string]trainingEntry{}, true, nil)
	if got := stringValue(selection["provider"]); got != "claude" {
		t.Fatalf("provider = %q, want claude for architecture authority", got)
	}
}

func TestSelectLaneExtractionTaskKeepsLocalWhenModelFitsHost(t *testing.T) {
	task, err := applyTaskWorkerProfile(map[string]any{
		"id":              "repo_fact_packet",
		"worker":          "ollama",
		"worker_type":     "extraction",
		"goal":            "Extract narrow repo facts only",
		"expected_output": "A compact fact packet",
		"constraints":     []any{"facts only", "extract path groups"},
		"model_policy": map[string]any{
			"attempt": map[string]any{
				"preference": "cheap-first",
			},
		},
	}, map[string]string{})
	if err != nil {
		t.Fatal(err)
	}
	lanes := []map[string]any{
		{
			"id":            "ollama.local.default",
			"provider":      "ollama",
			"resolver_hint": "ollama",
			"model":         "qwen2.5:7b",
			"local":         true,
			"available":     true,
		},
		{
			"id":            "codex.cloud.default",
			"provider":      "codex",
			"resolver_hint": "codex",
			"model":         "cli",
			"cloud":         true,
			"available":     true,
			"capabilities":  []any{"code", "strong_validation"},
		},
	}
	selection := selectLane(task, mapValue(task["model_policy"]), "", "", "", map[string]string{
		"DORKPIPE_ORCH_HOST_MEMORY_GB":     "64",
		"DORKPIPE_ORCH_HOST_CPU_CORES":     "16",
		"DORKPIPE_ORCH_LOCAL_ACCELERATION": "gpu",
	}, lanes, map[string]any{
		"local_first_bonus":          15.0,
		"cloud_cost_penalty":         2.0,
		"worker_preference_bonus":    10.0,
		"extraction_local_bonus":     8.0,
		"local_model_fit_bonus":      3.0,
		"gpu_local_extraction_bonus": 2.0,
		"extraction_keywords":        []any{"extract", "facts only", "fact packet", "path groups"},
	}, map[string]any{}, map[string]trainingEntry{}, true, nil)
	if got := stringValue(selection["provider"]); got != "ollama" {
		t.Fatalf("provider = %q, want ollama for bounded extraction", got)
	}
}

func TestSelectLaneOversizedLocalModelLosesOnSmallHost(t *testing.T) {
	task, err := applyTaskWorkerProfile(map[string]any{
		"id":              "design_inventory",
		"worker":          "ollama",
		"worker_type":     "extraction",
		"goal":            "Extract design inventory only",
		"expected_output": "A compact inventory packet",
		"constraints":     []any{"extract path groups"},
		"model_policy": map[string]any{
			"attempt": map[string]any{
				"preference": "cheap-first",
			},
		},
	}, map[string]string{})
	if err != nil {
		t.Fatal(err)
	}
	lanes := []map[string]any{
		{
			"id":            "ollama.local.default",
			"provider":      "ollama",
			"resolver_hint": "ollama",
			"model":         "qwen2.5:32b",
			"local":         true,
			"available":     true,
		},
		{
			"id":            "claude.cloud.default",
			"provider":      "claude",
			"resolver_hint": "claude",
			"model":         "cli",
			"cloud":         true,
			"available":     true,
			"capabilities":  []any{"review", "strong_validation"},
		},
	}
	selection := selectLane(task, mapValue(task["model_policy"]), "", "", "", map[string]string{
		"DORKPIPE_ORCH_HOST_MEMORY_GB":     "16",
		"DORKPIPE_ORCH_HOST_CPU_CORES":     "8",
		"DORKPIPE_ORCH_LOCAL_ACCELERATION": "cpu",
	}, lanes, map[string]any{
		"local_first_bonus":             15.0,
		"cloud_cost_penalty":            2.0,
		"worker_preference_bonus":       10.0,
		"extraction_local_bonus":        8.0,
		"oversized_local_model_penalty": 30.0,
		"extraction_keywords":           []any{"extract", "inventory", "path groups"},
	}, map[string]any{}, map[string]trainingEntry{}, true, nil)
	if got := stringValue(selection["provider"]); got != "claude" {
		t.Fatalf("provider = %q, want claude when local model is oversized for host", got)
	}
}

func TestComparisonDisabledForRequiredWorker(t *testing.T) {
	task, err := applyTaskWorkerProfile(map[string]any{
		"id":     "patch",
		"worker": "codex",
		"worker_policy": map[string]any{
			"mode": "require",
		},
	}, map[string]string{})
	if err != nil {
		t.Fatal(err)
	}
	if comparisonEnabledForTask(task, []string{"codex", "claude"}, "auto") {
		t.Fatal("comparison should be disabled when worker_policy.mode=require")
	}
}

func TestEmitTaskEnvIncludesWorkMode(t *testing.T) {
	taskPath := filepath.Join(t.TempDir(), "task.json")
	if err := os.WriteFile(taskPath, []byte(`{"id":"patch","worker":"codex","work_mode":"edit","output_path":"/work/docs/brain.md"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout bytes.Buffer
	if err := emitTaskEnv(taskPath, &stdout); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout.String(), "TASK_WORK_MODE='edit'") {
		t.Fatalf("task env missing work mode:\n%s", stdout.String())
	}
	if !strings.Contains(stdout.String(), "TASK_OUTPUT_PATH='/work/docs/brain.md'") {
		t.Fatalf("task env missing output path:\n%s", stdout.String())
	}
}

func TestInferTaskOutputPathPrefersExplicitThenExpectedOutput(t *testing.T) {
	if got := inferTaskOutputPath(map[string]any{
		"output_path":     "/work/docs/explicit.md",
		"expected_output": "Write /work/docs/fallback.md",
	}); got != "/work/docs/explicit.md" {
		t.Fatalf("explicit output path = %q", got)
	}
	if got := inferTaskOutputPath(map[string]any{
		"expected_output": "Update canonical doc at /work/docs/agents/index.md and keep links valid.",
	}); got != "/work/docs/agents/index.md" {
		t.Fatalf("inferred output path = %q", got)
	}
}

func TestEmitRequiredAuthProviders(t *testing.T) {
	tasksDir := filepath.Join(t.TempDir(), "tasks")
	if err := os.MkdirAll(filepath.Join(tasksDir, "need-claude"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(tasksDir, "need-codex"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(tasksDir, "prefer-claude"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(tasksDir, "local-ollama"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tasksDir, "need-claude", "task.json"), []byte(`{"worker":"claude","worker_policy":{"mode":"require"},"lane":{"provider":"claude"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tasksDir, "need-codex", "task.json"), []byte(`{"worker":"codex","worker_policy":{"mode":"require"},"lane":{"provider":"codex"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tasksDir, "prefer-claude", "task.json"), []byte(`{"worker":"claude","worker_policy":{"mode":"prefer"},"lane":{"provider":"claude"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tasksDir, "local-ollama", "task.json"), []byte(`{"worker":"ollama","worker_policy":{"mode":"require"},"lane":{"provider":"ollama"}}`), 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout bytes.Buffer
	if err := emitRequiredAuthProviders(tasksDir, &stdout); err != nil {
		t.Fatal(err)
	}
	got := strings.Fields(stdout.String())
	want := []string{"claude", "codex"}
	if len(got) != len(want) {
		t.Fatalf("required providers = %#v want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("required providers = %#v want %#v", got, want)
		}
	}
}

func TestResolveDockpipeCommandPrefersEnv(t *testing.T) {
	got := resolveDockpipeCommand(t.TempDir(), map[string]string{"DOCKPIPE_BIN": "/custom/dockpipe"})
	if got != "/custom/dockpipe" {
		t.Fatalf("resolveDockpipeCommand() = %q", got)
	}
}

func TestResolveDockpipeCommandFallsBackToRepoLocalBinary(t *testing.T) {
	root := t.TempDir()
	binDir := filepath.Join(root, "src", "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(binDir, "dockpipe")
	if runtime.GOOS == "windows" {
		want += ".exe"
	}
	if err := os.WriteFile(want, []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	got := resolveDockpipeCommand(root, map[string]string{})
	if got != want {
		t.Fatalf("resolveDockpipeCommand() = %q want %q", got, want)
	}
}

func TestResolveApplyTargetPathMapsAllowedGuestMount(t *testing.T) {
	root := t.TempDir()
	uniteHere := filepath.Join(root, "UniteHere")
	designNotes := filepath.Join(root, "DesignNotes")
	t.Setenv("DOCKPIPE_CONTAINER_MOUNTS", uniteHere+":/UniteHere:ro\n"+designNotes+":/DesignNotes:ro")
	t.Setenv("DORKPIPE_ORCH_APPLY_ALLOWED_GUEST_ROOTS", "/UniteHere")

	gotPath, gotRoot, err := resolveApplyTargetPath(root, "/UniteHere/docs/agents/plans/brain.md")
	if err != nil {
		t.Fatal(err)
	}
	wantPath := filepath.Join(uniteHere, "docs", "agents", "plans", "brain.md")
	if gotPath != wantPath {
		t.Fatalf("target path = %q want %q", gotPath, wantPath)
	}
	if gotRoot != uniteHere {
		t.Fatalf("target root = %q want %q", gotRoot, uniteHere)
	}
}

func TestResolveApplyTargetPathAllowsGitBashConvertedGuestRoot(t *testing.T) {
	root := t.TempDir()
	uniteHere := filepath.Join(root, "UniteHere")
	t.Setenv("DOCKPIPE_CONTAINER_MOUNTS", uniteHere+":/UniteHere:ro")
	t.Setenv("DORKPIPE_ORCH_APPLY_ALLOWED_GUEST_ROOTS", "C:/Program Files/Git/UniteHere")

	gotPath, gotRoot, err := resolveApplyTargetPath(root, "/UniteHere/docs/agents/plans/brain.md")
	if err != nil {
		t.Fatal(err)
	}
	wantPath := filepath.Join(uniteHere, "docs", "agents", "plans", "brain.md")
	if gotPath != wantPath {
		t.Fatalf("target path = %q want %q", gotPath, wantPath)
	}
	if gotRoot != uniteHere {
		t.Fatalf("target root = %q want %q", gotRoot, uniteHere)
	}
}

func TestResolveApplyTargetPathRejectsDisallowedGuestMount(t *testing.T) {
	root := t.TempDir()
	t.Setenv("DOCKPIPE_CONTAINER_MOUNTS", filepath.Join(root, "UniteHere")+":/UniteHere:ro\n"+filepath.Join(root, "DesignNotes")+":/DesignNotes:ro")
	t.Setenv("DORKPIPE_ORCH_APPLY_ALLOWED_GUEST_ROOTS", "/UniteHere")

	if _, _, err := resolveApplyTargetPath(root, "/DesignNotes/planning/generated.md"); err == nil {
		t.Fatal("expected disallowed guest mount apply target to fail")
	}
}

func TestResolveApplyTargetPathFallsBackToWorkflowRootForWorkGuestPath(t *testing.T) {
	root := t.TempDir()
	t.Setenv("DOCKPIPE_CONTAINER_MOUNTS", filepath.Join(root, "DesignNotes")+":/DesignNotes:ro")
	t.Setenv("DORKPIPE_ORCH_APPLY_ALLOWED_GUEST_ROOTS", "/work")

	gotPath, gotRoot, err := resolveApplyTargetPath(root, "/work/docs/agents/brain/index.md")
	if err != nil {
		t.Fatal(err)
	}
	wantPath := filepath.Join(root, "docs", "agents", "brain", "index.md")
	if gotPath != wantPath {
		t.Fatalf("target path = %q want %q", gotPath, wantPath)
	}
	if gotRoot != root {
		t.Fatalf("target root = %q want %q", gotRoot, root)
	}
}

func TestHasSchedulerOutputConflictMatchesSameTarget(t *testing.T) {
	running := map[string]schedulerTask{
		"author_index": {ID: "author_index", OutputPath: "/work/docs/agents/index.md"},
	}
	if !hasSchedulerOutputConflict(schedulerTask{ID: "finalize_index", OutputPath: "/work/docs/agents/index.md"}, running) {
		t.Fatal("expected same output path to conflict")
	}
	if hasSchedulerOutputConflict(schedulerTask{ID: "author_repo", OutputPath: "/work/docs/agents/repo.md"}, running) {
		t.Fatal("did not expect different output path to conflict")
	}
}

func TestApplyResultsPreflightsAllSourcesBeforeWriting(t *testing.T) {
	root := t.TempDir()
	artifactRoot := filepath.Join(root, "artifacts")
	if err := os.MkdirAll(filepath.Join(artifactRoot, "merge"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(artifactRoot, "merge", "first.md"), []byte("first"), 0o644); err != nil {
		t.Fatal(err)
	}
	planPath := filepath.Join(artifactRoot, "plan.json")
	approvalPath := filepath.Join(artifactRoot, "approval.md")
	resultPath := filepath.Join(artifactRoot, "apply.json")
	if err := os.WriteFile(planPath, []byte(`{"apply":{"require_approval":true,"outputs":[{"source":"merge/first.md","path":"out/first.md"},{"source":"merge/missing.md","path":"out/missing.md"}]}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(approvalPath, []byte("- Approved: yes\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := applyResults(root, artifactRoot, planPath, approvalPath, resultPath); err == nil {
		t.Fatal("expected missing second source to fail")
	}
	if _, err := os.Stat(filepath.Join(root, "out", "first.md")); !os.IsNotExist(err) {
		t.Fatalf("first output should not be written before all sources preflight, err=%v", err)
	}
}

func TestEmitVerifyApplyCoherenceFlagsBrokenMarkdownAndYamlTargets(t *testing.T) {
	root := t.TempDir()
	artifactRoot := filepath.Join(root, "artifacts")
	if err := os.MkdirAll(filepath.Join(artifactRoot, "merge"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(artifactRoot, "merge", "index.md"), []byte("[Missing](./missing.md)\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(artifactRoot, "merge", "index.yaml"), []byte("canonical: ./missing.md\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	planPath := filepath.Join(artifactRoot, "plan.json")
	if err := os.WriteFile(planPath, []byte(`{"apply":{"outputs":[{"source":"merge/index.md","path":"docs/index.md"},{"source":"merge/index.yaml","path":"docs/index.yaml"}]}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	var out strings.Builder
	if err := emitVerifyApplyCoherence(root, artifactRoot, planPath, `[]`, &out); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	if !strings.Contains(got, "VERIFY_APPLY_STATUS='review'") {
		t.Fatalf("expected review status, got:\n%s", got)
	}
	for _, want := range []string{"markdown link target is missing", "yaml reference target is missing"} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in:\n%s", want, got)
		}
	}
}

func TestEmitVerifyApplyCoherenceFlagsContradictoryValidationClaim(t *testing.T) {
	root := t.TempDir()
	artifactRoot := filepath.Join(root, "artifacts")
	if err := os.MkdirAll(filepath.Join(artifactRoot, "merge"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "docs"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "docs", "source-of-truth.md"), []byte("still here\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(artifactRoot, "merge", "validation.md"), []byte("- **Removed `source-of-truth.md`**\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	planPath := filepath.Join(artifactRoot, "plan.json")
	if err := os.WriteFile(planPath, []byte(`{"apply":{"outputs":[{"source":"merge/validation.md","path":"docs/validation.md"}]}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	var out strings.Builder
	if err := emitVerifyApplyCoherence(root, artifactRoot, planPath, `[]`, &out); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	if !strings.Contains(got, "still exists") {
		t.Fatalf("expected contradictory validation claim issue, got:\n%s", got)
	}
}

func TestBuildMergeResultUsesTaskResultObjects(t *testing.T) {
	dir := t.TempDir()
	mainPath := filepath.Join(dir, "main.json")
	planPath := filepath.Join(dir, "planning.json")
	outPath := filepath.Join(dir, "merge.json")
	if err := os.WriteFile(mainPath, []byte(`{"task_id":"codex_brain_plan","provider_actual":"codex","summary":"done","confidence":0.8,"estimated_input_tokens":10,"estimated_output_tokens":5,"estimated_total_tokens":15,"duration_ms":100}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(planPath, []byte(`{"task_id":"repo_knowledge","provider_actual":"ollama","summary":"planned","confidence":0.6,"estimated_input_tokens":4,"estimated_output_tokens":2,"estimated_total_tokens":6,"duration_ms":20}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := buildMergeResult(outPath, []string{mainPath, "--planning", planPath}); err != nil {
		t.Fatal(err)
	}
	result := readJSONMap(outPath)
	tasks := listValue(result["tasks"])
	if len(tasks) != 1 {
		t.Fatalf("tasks length = %d want 1", len(tasks))
	}
	task := mapValue(tasks[0])
	if got := stringValue(task["task_id"]); got != "codex_brain_plan" {
		t.Fatalf("task_id = %q", got)
	}
	if got := intAny(result["total_estimated_task_tokens"]); got != 15 {
		t.Fatalf("total_estimated_task_tokens = %d", got)
	}
	planning := listValue(result["planning_tasks"])
	if len(planning) != 1 {
		t.Fatalf("planning length = %d want 1", len(planning))
	}
}

func TestOllamaChatRequestAndResponseHelpers(t *testing.T) {
	dir := t.TempDir()
	promptPath := filepath.Join(dir, "prompt.md")
	requestPath := filepath.Join(dir, "request.json")
	responsePath := filepath.Join(dir, "response.json")
	outPath := filepath.Join(dir, "response.md")
	if err := os.WriteFile(promptPath, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := writeOllamaChatRequest("llama-test", promptPath, requestPath); err != nil {
		t.Fatal(err)
	}
	request := readJSONMap(requestPath)
	if got := stringValue(request["model"]); got != "llama-test" {
		t.Fatalf("model = %q", got)
	}
	if err := os.WriteFile(responsePath, []byte(`{"message":{"content":"useful response"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := writeOllamaChatResponse(responsePath, outPath); err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(string(mustReadFile(t, outPath))); got != "useful response" {
		t.Fatalf("response = %q", got)
	}
}

func TestEmitVerifySummaryEnvCountsFallbackTasks(t *testing.T) {
	dir := t.TempDir()
	mergePath := filepath.Join(dir, "merge.json")
	if err := os.WriteFile(mergePath, []byte(`{"average_confidence":0.55,"tasks":[{"used_live_model":false},{"used_live_model":true},{"used_live_model":false}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	var out strings.Builder
	if err := emitVerifySummaryEnv(mergePath, &out); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	for _, want := range []string{"VERIFY_LIVE_COUNT='1'", "VERIFY_FALLBACK_COUNT='2'", "VERIFY_AVG_CONFIDENCE='0.55'"} {
		if !strings.Contains(got, want) {
			t.Fatalf("verify env missing %q in:\n%s", want, got)
		}
	}
}

func TestExecutableSearchPathEntriesNormalizesGitBashWindowsPath(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Git Bash PATH normalization is Windows-specific")
	}
	entries := executableSearchPathEntries("/c/Program Files/Docker/Docker/resources/bin:/usr/bin")
	if len(entries) == 0 || entries[0] != `C:\Program Files\Docker\Docker\resources\bin` {
		t.Fatalf("entries = %#v", entries)
	}
}

func TestWindowsExecutableFallbackDirsIncludesDockerDesktop(t *testing.T) {
	dirs := windowsExecutableFallbackDirs("docker")
	if len(dirs) == 0 {
		t.Fatal("expected docker fallback dirs")
	}
	if dirs[0] != `C:\Program Files\Docker\Docker\resources\bin` {
		t.Fatalf("first fallback dir = %q", dirs[0])
	}
}

func mustReadFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
