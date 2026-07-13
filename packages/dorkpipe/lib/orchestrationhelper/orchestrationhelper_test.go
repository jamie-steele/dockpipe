package orchestrationhelper

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestLoadAgentsConfigUsesNearestWorkflowParentAndSiblingOverride(t *testing.T) {
	root := t.TempDir()
	workflowPath := filepath.Join(root, "workflows", "agent", "review", "config.yml")
	writeAgentsConfig := func(path, role string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		content := "agents:\n  reviewer:\n    role: " + role + "\n"
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	writeAgentsConfig(filepath.Join(root, "agents.yml"), "outside-workflow-root")
	writeAgentsConfig(filepath.Join(root, "workflows", "agent", "agents.yml"), "shared-reviewer")
	if got := stringValue(mapValue(loadAgentsConfig(workflowPath)["reviewer"])["role"]); got != "shared-reviewer" {
		t.Fatalf("parent role = %q, want shared-reviewer", got)
	}

	writeAgentsConfig(filepath.Join(filepath.Dir(workflowPath), "agents.yml"), "local-reviewer")
	if got := stringValue(mapValue(loadAgentsConfig(workflowPath)["reviewer"])["role"]); got != "local-reviewer" {
		t.Fatalf("sibling role = %q, want local-reviewer", got)
	}
}

func TestLoadAgentsConfigUsesPackageAuthoringRoot(t *testing.T) {
	root := t.TempDir()
	packageRoot := filepath.Join(root, "packages", "example")
	workflowPath := filepath.Join(packageRoot, "workflows", "review", "config.yml")
	if err := os.MkdirAll(filepath.Dir(workflowPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(packageRoot, "package.yml"), []byte("name: example\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "agents.yml"), []byte("agents:\n  reviewer:\n    role: outside-package-root\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(packageRoot, "agents.yml"), []byte("agents:\n  reviewer:\n    role: package-reviewer\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if got := stringValue(mapValue(loadAgentsConfig(workflowPath)["reviewer"])["role"]); got != "package-reviewer" {
		t.Fatalf("package role = %q, want package-reviewer", got)
	}
}

func TestRenderExecutionLanePromptContextIncludesSelectionAndPolicy(t *testing.T) {
	got := renderExecutionLanePromptContext(map[string]any{
		"requested": "auto",
		"lane_id":   "codex.strong",
		"provider":  "codex",
		"model":     "gpt-5.6",
		"reasons":   []string{"high-authority task favors a strong lane", "available"},
	}, map[string]any{
		"task_class": map[string]any{"name": "architecture", "authority": "high"},
		"model_policy": map[string]any{
			"attempt": map[string]any{"preference": "strong"},
		},
	})
	for _, want := range []string{
		"Execution lane (operational run metadata):",
		"Requested lane: auto",
		"Selected lane: codex.strong",
		"Provider: codex",
		"Model: gpt-5.6",
		"Work class: architecture",
		"Authority: high",
		"Selection rationale: high-authority task favors a strong lane; available",
		"Model policy: `{\"attempt\":{\"preference\":\"strong\"}}`",
		"current run only",
		"Do not substitute lane selection for source evidence",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("prompt context missing %q:\n%s", want, got)
		}
	}
}

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

func TestSelectLanePlanningWorkerPreferPinsDeclaredLane(t *testing.T) {
	task, err := applyTaskWorkerProfile(map[string]any{
		"id":          "contract_brain",
		"worker":      "ollama",
		"worker_type": "planning",
		"goal":        "distill an architecture contract",
	}, map[string]string{})
	if err != nil {
		t.Fatal(err)
	}
	lanes := []map[string]any{
		{"id": "ollama.local.default", "provider": "ollama", "resolver_hint": "ollama", "local": true, "available": true},
		{"id": "codex.cloud.default", "provider": "codex", "resolver_hint": "codex", "cloud": true, "available": true, "capabilities": []any{"strong_validation"}},
	}
	selection := selectLane(task, map[string]any{"validate": map[string]any{"preference": "strongest_available"}}, "", "", "", map[string]string{}, lanes, map[string]any{
		"local_first_bonus":          15.0,
		"cloud_cost_penalty":         2.0,
		"worker_preference_bonus":    10.0,
		"strong_validation_bonus":    8.0,
		"authority_cloud_bonus":      8.0,
		"local_architecture_penalty": 18.0,
	}, map[string]any{}, map[string]trainingEntry{}, true, nil)
	if got := stringValue(selection["provider"]); got != "ollama" {
		t.Fatalf("provider = %q, want declared planning lane ollama", got)
	}
	if got := stringValue(selection["requested"]); got != "ollama" {
		t.Fatalf("requested = %q, want ollama", got)
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

func TestEmitTaskEnvIncludesProviderPoolModelPolicy(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "task.json")
	if err := writeJSONFile(path, map[string]any{
		"id":   "review",
		"goal": "review the change",
		"model_policy": map[string]any{
			"execution_mode":        "provider_pool",
			"role":                  "reviewer",
			"session_scope":         "workflow",
			"max_active":            1,
			"queue_timeout_seconds": 2,
		},
	}); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := emitTaskEnv(path, &out); err != nil {
		t.Fatal(err)
	}
	text := out.String()
	for _, want := range []string{
		"TASK_MODEL_POLICY_EXECUTION_MODE='provider_pool'",
		"TASK_PROVIDER_POOL_ROLE='reviewer'",
		"TASK_PROVIDER_POOL_SESSION_SCOPE='workflow'",
		"TASK_PROVIDER_POOL_MAX_ACTIVE='1'",
		"TASK_PROVIDER_POOL_QUEUE_TIMEOUT_SECONDS='2'",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("task env missing %s in:\n%s", want, text)
		}
	}
}

func TestEmitProviderPoolResponseEnvWritesResponseAndMetadata(t *testing.T) {
	dir := t.TempDir()
	responseJSON := filepath.Join(dir, "provider-pool-response.json")
	responseMD := filepath.Join(dir, "response.md")
	if err := writeJSONFile(responseJSON, map[string]any{
		"state":     "ready",
		"status":    "ready",
		"text":      "pooled response",
		"exit_code": 0,
		"metadata": map[string]any{
			"session_id":     "workflow:run:node:review",
			"worker_id":      "worker-1",
			"prompt_turn_id": "turn-1",
		},
	}); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := emitProviderPoolResponseEnv(responseJSON, responseMD, &out); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(responseMD)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != "pooled response" {
		t.Fatalf("response.md = %q", string(raw))
	}
	text := out.String()
	for _, want := range []string{
		"PROVIDER_POOL_STATE='ready'",
		"PROVIDER_POOL_USED_LIVE_MODEL='true'",
		"PROVIDER_POOL_PROVIDER_SESSION_ID='workflow:run:node:review'",
		"PROVIDER_POOL_WORKER_ID='worker-1'",
		"PROVIDER_POOL_PROMPT_TURN_ID='turn-1'",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("provider pool env missing %s in:\n%s", want, text)
		}
	}
}

func TestSelectLaneDoesNotUseMismatchedRoleModel(t *testing.T) {
	task, err := applyTaskWorkerProfile(map[string]any{
		"id":     "contract_brain",
		"worker": "ollama",
		"model": map[string]any{
			"provider": "ollama",
			"model":    "llama3.2",
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
			"model":         "llama3.2",
			"local":         true,
			"available":     true,
		},
		{
			"id":            "codex.cli.default",
			"provider":      "codex",
			"resolver_hint": "codex",
			"model":         "cli",
			"cloud":         true,
			"available":     true,
		},
	}
	selection := selectLane(task, mapValue(task["model_policy"]), "codex", "", "", map[string]string{}, lanes, map[string]any{}, map[string]any{}, map[string]trainingEntry{}, true, nil)
	if got := stringValue(selection["provider"]); got != "codex" {
		t.Fatalf("provider = %q, want codex", got)
	}
	if got := stringValue(selection["model"]); got != "cli" {
		t.Fatalf("model = %q, want selected codex lane model", got)
	}
	effective := taskModelForLane(mapValue(task["model"]), selection)
	if got := stringValue(effective["provider"]); got != "codex" {
		t.Fatalf("effective provider = %q, want codex", got)
	}
	if got := stringValue(effective["model"]); got != "cli" {
		t.Fatalf("effective model = %q, want cli", got)
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

func TestEmitTaskEnvIncludesLaneAvailability(t *testing.T) {
	taskPath := filepath.Join(t.TempDir(), "task.json")
	task := `{
		"id": "patch",
		"worker": "codex",
		"lane": {
			"lane_id": "codex.cli.default",
			"provider": "codex",
			"available": false,
			"missing_commands": ["codex"],
			"setup_hint": "Install and sign in to the Codex CLI.",
			"auth_hint": "Codex CLI must be authenticated."
		}
	}`
	if err := os.WriteFile(taskPath, []byte(task), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout bytes.Buffer
	if err := emitTaskEnv(taskPath, &stdout); err != nil {
		t.Fatal(err)
	}
	got := stdout.String()
	for _, want := range []string{
		"TASK_LANE_AVAILABLE='false'",
		`TASK_LANE_MISSING_COMMANDS_JSON='["codex"]'`,
		"TASK_LANE_SETUP_HINT='Install and sign in to the Codex CLI.'",
		"TASK_LANE_AUTH_HINT='Codex CLI must be authenticated.'",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("task env missing %q:\n%s", want, got)
		}
	}
}

func TestWriteTaskResultIncludesTraceOnlyWorkerSession(t *testing.T) {
	outPath := filepath.Join(t.TempDir(), "result.json")
	err := writeTaskResult(outPath, map[string]string{
		"task_id":                 "author_index",
		"status":                  "ok",
		"resolver_hint":           "codex",
		"provider":                "codex",
		"selected_model":          "cli",
		"lane_id":                 "codex.cloud.default",
		"provider_session_id":     "abc123",
		"used_live_model":         "true",
		"budget_halt":             "false",
		"estimated_input_tokens":  "10",
		"estimated_output_tokens": "5",
		"estimated_total_tokens":  "15",
		"confidence":              "0.72",
		"issues_json":             "[]",
		"next_actions_json":       "[]",
	})
	if err != nil {
		t.Fatal(err)
	}
	result := readJSONMap(outPath)
	session := mapValue(result["worker_session"])
	if got := stringValue(session["session_id"]); got != "abc123" {
		t.Fatalf("session_id = %q want abc123", got)
	}
	if got := stringValue(session["mode"]); got != "trace_only" {
		t.Fatalf("session mode = %q want trace_only", got)
	}
}

func TestMaterializeTaskOutputsExtractsDeclaredBlocks(t *testing.T) {
	dir := t.TempDir()
	responsePath := filepath.Join(dir, "response.md")
	resultPath := filepath.Join(dir, "materialized-result.json")
	response := strings.Join([]string{
		`<!-- dorkpipe:file path="index.md" -->`,
		"# Index",
		"",
		"Start here.",
		`<!-- /dorkpipe:file -->`,
		"",
		`<!-- dorkpipe:file path="index.yaml" -->`,
		"schema: test",
		`<!-- /dorkpipe:file -->`,
	}, "\n")
	if err := os.WriteFile(responsePath, []byte(response), 0o644); err != nil {
		t.Fatal(err)
	}
	outputs := `[{"path":"index.md"},{"path":"index.yaml"}]`
	if err := materializeTaskOutputs(responsePath, dir, outputs, resultPath); err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(string(mustReadFile(t, filepath.Join(dir, "materialized", "index.md")))); got != "# Index\n\nStart here." {
		t.Fatalf("index.md = %q", got)
	}
	if got := stringValue(readJSONMap(resultPath)["status"]); got != "materialized" {
		t.Fatalf("status = %q want materialized", got)
	}
}

func TestRenderMaterializeOutputContractShowsExactBlocks(t *testing.T) {
	got := renderMaterializeOutputContract([]any{
		map[string]any{"path": "index.yaml"},
		map[string]any{"path": "design-corpus-index.yaml"},
	})
	for _, want := range []string{
		"DorkPipe materialized output contract:",
		"Required output paths: index.yaml, design-corpus-index.yaml",
		`<!-- dorkpipe:file path="index.yaml" -->`,
		`<!-- dorkpipe:file path="design-corpus-index.yaml" -->`,
		"Do not use YAML bundle/list wrappers around the file blocks.",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("contract missing %q in:\n%s", want, got)
		}
	}
}

func TestMaterializeTaskOutputsRejectsEscapingPath(t *testing.T) {
	dir := t.TempDir()
	responsePath := filepath.Join(dir, "response.md")
	if err := os.WriteFile(responsePath, []byte(`<!-- dorkpipe:file path="../bad.md" -->bad<!-- /dorkpipe:file -->`), 0o644); err != nil {
		t.Fatal(err)
	}
	err := materializeTaskOutputs(responsePath, dir, `[{"path":"../bad.md"}]`, filepath.Join(dir, "result.json"))
	if err == nil {
		t.Fatal("expected escaping materialized path to fail")
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

func TestMountedGuestRootNotesDescribesDesignNotesAsExternalMirror(t *testing.T) {
	notes := mountedGuestRootNotes("C:\\docs\\UniteHere\\UH - SePuede - Design Notes:/DesignNotes:ro\nC:\\Source\\UniteHere:/work:rw")
	got := strings.Join(notes, "\n")
	for _, want := range []string{
		"`/DesignNotes` is an external mounted design corpus",
		"Host path for this run: `C:\\docs\\UniteHere\\UH - SePuede - Design Notes`.",
		"SharePoint-backed or Windows-local design-notes mirror",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("mounted guest root notes missing %q in:\n%s", want, got)
		}
	}
	if strings.Contains(got, "`/work` is an external mounted") {
		t.Fatalf("mounted guest root notes should not emit a standalone /work mount note:\n%s", got)
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

func TestApplyResultsAllowsReviewForWorkspaceDiff(t *testing.T) {
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
	verifyPath := filepath.Join(artifactRoot, "verify.json")
	resultPath := filepath.Join(artifactRoot, "apply.json")
	if err := os.WriteFile(planPath, []byte(`{"apply":{"require_approval":true,"outputs":[{"source":"merge/first.md","path":"out/first.md"}]}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(approvalPath, []byte("- Approved: yes\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(verifyPath, []byte(`{"status":"review"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := applyResultsWithVerify(root, artifactRoot, planPath, approvalPath, resultPath, verifyPath, false); err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(filepath.Join(root, "out", "first.md")); err != nil || string(got) != "first" {
		t.Fatalf("output = %q err=%v", string(got), err)
	}
	result := readJSONMap(resultPath)
	if got := stringValue(result["status"]); got != "applied" {
		t.Fatalf("status = %q want applied", got)
	}
	if got := stringValue(result["verify_status"]); got != "review" {
		t.Fatalf("verify_status = %q want review", got)
	}
	if !boolAny(result["requires_human_review"]) {
		t.Fatal("expected requires_human_review for review-status apply")
	}
	if boolAny(result["publish_allowed"]) {
		t.Fatal("review-status apply should not mark publish_allowed")
	}
}

func TestApplyResultsSkipsWhenVerifyStatusIsFail(t *testing.T) {
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
	verifyPath := filepath.Join(artifactRoot, "verify.json")
	resultPath := filepath.Join(artifactRoot, "apply.json")
	if err := os.WriteFile(planPath, []byte(`{"apply":{"require_approval":true,"outputs":[{"source":"merge/first.md","path":"out/first.md"}]}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(approvalPath, []byte("- Approved: yes\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(verifyPath, []byte(`{"status":"fail"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := applyResultsWithVerify(root, artifactRoot, planPath, approvalPath, resultPath, verifyPath, false); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "out", "first.md")); !os.IsNotExist(err) {
		t.Fatalf("output should not be written when verify status is fail, err=%v", err)
	}
	result := readJSONMap(resultPath)
	if got := stringValue(result["status"]); got != "skipped" {
		t.Fatalf("status = %q want skipped", got)
	}
	if got := stringValue(result["verify_status"]); got != "fail" {
		t.Fatalf("verify_status = %q want fail", got)
	}
}

func TestApplyResultsInfersMaterializedOutputsFromTargetRoot(t *testing.T) {
	root := t.TempDir()
	artifactRoot := filepath.Join(root, "artifacts")
	materializedDir := filepath.Join(artifactRoot, "tasks", "writer", "materialized")
	if err := os.MkdirAll(materializedDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(materializedDir, "index.md"), []byte("index"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(materializedDir, "open-gaps.md"), []byte("gaps"), 0o644); err != nil {
		t.Fatal(err)
	}
	planPath := filepath.Join(artifactRoot, "plan.json")
	approvalPath := filepath.Join(artifactRoot, "approval.md")
	resultPath := filepath.Join(artifactRoot, "apply.json")
	if err := os.WriteFile(planPath, []byte(`{"apply":{"require_approval":true,"target_root":"docs/agents/brain","required_artifacts":["index.md"]}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(approvalPath, []byte("- Approved: yes\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := applyResults(root, artifactRoot, planPath, approvalPath, resultPath); err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(filepath.Join(root, "docs", "agents", "brain", "index.md")); err != nil || string(got) != "index" {
		t.Fatalf("index.md = %q err=%v", string(got), err)
	}
	if got, err := os.ReadFile(filepath.Join(root, "docs", "agents", "brain", "open-gaps.md")); err != nil || string(got) != "gaps" {
		t.Fatalf("open-gaps.md = %q err=%v", string(got), err)
	}
}

func TestApplyResultsFailsWhenRequiredInferredArtifactMissing(t *testing.T) {
	root := t.TempDir()
	artifactRoot := filepath.Join(root, "artifacts")
	materializedDir := filepath.Join(artifactRoot, "tasks", "writer", "materialized")
	if err := os.MkdirAll(materializedDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(materializedDir, "index.md"), []byte("index"), 0o644); err != nil {
		t.Fatal(err)
	}
	planPath := filepath.Join(artifactRoot, "plan.json")
	approvalPath := filepath.Join(artifactRoot, "approval.md")
	resultPath := filepath.Join(artifactRoot, "apply.json")
	if err := os.WriteFile(planPath, []byte(`{"apply":{"require_approval":true,"target_root":"docs/agents/brain","required_artifacts":["index.md","source-of-truth-rules.md"]}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(approvalPath, []byte("- Approved: yes\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := applyResults(root, artifactRoot, planPath, approvalPath, resultPath); err == nil {
		t.Fatal("expected missing required inferred artifact to fail")
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

func TestBuildVerifyResultAddsValueBarAndRerunTargets(t *testing.T) {
	dir := t.TempDir()
	planPath := filepath.Join(dir, "plan.json")
	graphPath := filepath.Join(dir, "graph.json")
	mergePath := filepath.Join(dir, "merge.json")
	usagePath := filepath.Join(dir, "cloud-usage.json")
	haltPath := filepath.Join(dir, "halt.json")
	outPath := filepath.Join(dir, "verify.json")
	if err := writeJSONFile(planPath, map[string]any{
		"apply": map[string]any{
			"require_approval": true,
			"outputs": []map[string]any{
				{"source": "tasks/author/response.md", "path": "/work/docs/brain.md"},
			},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := writeJSONFile(graphPath, map[string]any{
		"tasks": []map[string]any{
			{"id": "extract", "worker_type": "extraction", "provider": "ollama"},
			{"id": "architect", "worker_type": "architecture", "provider": "claude", "depends_on": []string{"extract"}},
			{"id": "author", "worker_type": "authoring", "provider": "codex", "depends_on": []string{"architect"}, "output_path": "/work/docs/brain.md"},
			{"id": "validator", "worker_type": "validation", "provider": "claude", "depends_on": []string{"author"}},
			{"id": "merge_final", "worker_type": "merge", "depends_on": []string{"validator"}},
			{"id": "verify_final", "worker_type": "verify", "depends_on": []string{"merge_final"}},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := writeJSONFile(mergePath, map[string]any{
		"average_confidence": 0.71,
		"tasks": []map[string]any{
			{"task_id": "extract", "status": "ok", "provider_actual": "ollama", "used_live_model": true, "confidence": 0.7},
			{"task_id": "architect", "status": "ok", "provider_actual": "claude", "used_live_model": true, "confidence": 0.8},
			{"task_id": "author", "status": "ok", "provider_actual": "codex", "used_live_model": true, "confidence": 0.75},
			{"task_id": "validator", "status": "ok", "provider_actual": "claude", "used_live_model": true, "confidence": 0.7},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := writeJSONFile(usagePath, map[string]any{"total_estimated_tokens": 4200}); err != nil {
		t.Fatal(err)
	}

	if err := buildVerifyResult(outPath, planPath, graphPath, mergePath, usagePath, haltPath, "pass", "0.71", `["author: markdown link target is missing: missing.md"]`, "review links", map[string]string{}); err != nil {
		t.Fatal(err)
	}
	result := readJSONMap(outPath)
	if got := stringValue(result["status"]); got != "review" {
		t.Fatalf("status = %q want review", got)
	}
	if got := stringValue(result["failure_class"]); got != "broken_references" {
		t.Fatalf("failure_class = %q want broken_references", got)
	}
	rerun := stringList(result["recommended_rerun_tasks"])
	if len(rerun) != 1 || rerun[0] != "author" {
		t.Fatalf("recommended_rerun_tasks = %#v want [author]", rerun)
	}
	valueBar := mapValue(result["value_bar"])
	if got := stringValue(valueBar["verdict"]); got != "strong_orchestration_value" {
		t.Fatalf("value_bar verdict = %q", got)
	}
	baseline := mapValue(result["direct_worker_baseline"])
	if got := stringValue(baseline["verdict"]); got != "orchestration_adds_value" {
		t.Fatalf("baseline verdict = %q", got)
	}
}

func TestBuildVerifyResultFlagsLowValueSerialGraph(t *testing.T) {
	dir := t.TempDir()
	planPath := filepath.Join(dir, "plan.json")
	graphPath := filepath.Join(dir, "graph.json")
	mergePath := filepath.Join(dir, "merge.json")
	usagePath := filepath.Join(dir, "cloud-usage.json")
	haltPath := filepath.Join(dir, "halt.json")
	outPath := filepath.Join(dir, "verify.json")
	if err := writeJSONFile(planPath, map[string]any{"apply": map[string]any{"require_approval": false}}); err != nil {
		t.Fatal(err)
	}
	if err := writeJSONFile(graphPath, map[string]any{
		"tasks": []map[string]any{
			{"id": "a", "worker_type": "analysis", "provider": "codex"},
			{"id": "b", "worker_type": "analysis", "provider": "codex", "depends_on": []string{"a"}},
			{"id": "c", "worker_type": "analysis", "provider": "codex", "depends_on": []string{"b"}},
			{"id": "d", "worker_type": "analysis", "provider": "codex", "depends_on": []string{"c"}},
			{"id": "merge_final", "worker_type": "merge", "depends_on": []string{"d"}},
			{"id": "verify_final", "worker_type": "verify", "depends_on": []string{"merge_final"}},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := writeJSONFile(mergePath, map[string]any{
		"average_confidence": 0.82,
		"tasks": []map[string]any{
			{"task_id": "a", "status": "ok", "provider_actual": "codex", "used_live_model": true, "confidence": 0.8},
			{"task_id": "b", "status": "ok", "provider_actual": "codex", "used_live_model": true, "confidence": 0.8},
			{"task_id": "c", "status": "ok", "provider_actual": "codex", "used_live_model": true, "confidence": 0.8},
			{"task_id": "d", "status": "ok", "provider_actual": "codex", "used_live_model": true, "confidence": 0.8},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := writeJSONFile(usagePath, map[string]any{"total_estimated_tokens": 9000}); err != nil {
		t.Fatal(err)
	}

	if err := buildVerifyResult(outPath, planPath, graphPath, mergePath, usagePath, haltPath, "pass", "0.82", `[]`, "review", map[string]string{}); err != nil {
		t.Fatal(err)
	}
	result := readJSONMap(outPath)
	if got := stringValue(result["status"]); got != "review" {
		t.Fatalf("status = %q want review", got)
	}
	if got := stringValue(result["failure_class"]); got != "low_value_graph" {
		t.Fatalf("failure_class = %q want low_value_graph", got)
	}
	baseline := mapValue(result["direct_worker_baseline"])
	if got := stringValue(baseline["verdict"]); got != "direct_worker_likely_better" {
		t.Fatalf("baseline verdict = %q", got)
	}
	graphLint := mapValue(result["graph_lint"])
	if got := stringValue(graphLint["status"]); got != "review" {
		t.Fatalf("graph_lint status = %q", got)
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
