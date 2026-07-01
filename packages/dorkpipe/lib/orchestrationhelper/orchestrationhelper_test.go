package orchestrationhelper

import (
	"os"
	"path/filepath"
	"runtime"
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
