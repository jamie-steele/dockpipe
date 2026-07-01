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
