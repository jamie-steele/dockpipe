package application

import (
	"os"
	"path/filepath"
	"testing"

	"dockpipe/lib/dockpipe/domain"
)

func TestBuildWorkflowEnvIntoPrecedenceAndOverrides(t *testing.T) {
	tmp := t.TempDir()
	wfRoot := filepath.Join(tmp, "wf")
	repoRoot := filepath.Join(tmp, "repo")
	if err := os.MkdirAll(wfRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(repoRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(p, s string) {
		t.Helper()
		if err := os.WriteFile(p, []byte(s), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(wfRoot, ".env"), "WF=wf\nSHARED=wf\n")
	write(filepath.Join(repoRoot, ".env"), "REPO=repo\nSHARED=repo\n")
	custom := filepath.Join(tmp, "custom.env")
	write(custom, "CUSTOM=1\nSHARED=custom\n")
	envFile := filepath.Join(tmp, "env-file.env")
	write(envFile, "ENVFILE=1\nSHARED=envfile\n")

	t.Setenv("DOCKPIPE_ENV_FILE", envFile)
	env := map[string]string{"SHARED": "existing"}
	wf := &domain.Workflow{Vars: map[string]string{"WFVAR": "x", "SHARED": "fromwfvar"}}
	opts := &CliOpts{
		EnvFiles:     []string{custom},
		VarOverrides: []string{"SHARED=override", "CLI=ok"},
	}
	buildWorkflowEnvInto(env, wf, wfRoot, repoRoot, opts)

	if env["WF"] != "wf" || env["REPO"] != "repo" || env["CUSTOM"] != "1" || env["ENVFILE"] != "1" || env["WFVAR"] != "x" {
		t.Fatalf("expected merged env sources, got %#v", env)
	}
	if env["SHARED"] != "override" {
		t.Fatalf("cli override should win, got %q", env["SHARED"])
	}
	if env["CLI"] != "ok" {
		t.Fatalf("missing CLI var override: %#v", env)
	}
}

func TestLockedKeysAndApplyOutputsFile(t *testing.T) {
	tmp := t.TempDir()
	outFile := filepath.Join(tmp, "outputs.env")
	if err := os.WriteFile(outFile, []byte("A=2\nB=2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	envMap := map[string]string{"A": "1"}
	dockerEnv := map[string]string{"A": "1"}
	locked := lockedKeys([]string{"A=lock", "X=1"})
	applyOutputsFile(outFile, envMap, dockerEnv, locked, nil, "")

	if envMap["A"] != "1" || dockerEnv["A"] != "1" {
		t.Fatalf("locked key should not change: env=%#v docker=%#v", envMap, dockerEnv)
	}
	if envMap["B"] != "2" || dockerEnv["B"] != "2" {
		t.Fatalf("unlocked output key should merge: env=%#v docker=%#v", envMap, dockerEnv)
	}
	if _, err := os.Stat(outFile); !os.IsNotExist(err) {
		t.Fatalf("outputs file should be removed, err=%v", err)
	}
}
