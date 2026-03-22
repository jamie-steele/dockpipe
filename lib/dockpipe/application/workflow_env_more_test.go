package application

import (
	"os"
	"path/filepath"
	"testing"

	"dockpipe/lib/dockpipe/domain"
)

// TestBuildWorkflowEnvIntoPrecedenceAndOverrides merges wf .env, repo .env, --env-file, DOCKPIPE_ENV_FILE, vars, --var.
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

// TestLockedKeysAndApplyOutputsFile merges step outputs into env except CLI-locked keys and deletes the file after read.
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

// TestApplyOutputsFileDoesNotWipeSecretAPIKeys prevents step outputs from clearing OPENAI_API_KEY etc. with empty values.
func TestApplyOutputsFileDoesNotWipeSecretAPIKeys(t *testing.T) {
	tmp := t.TempDir()
	outFile := filepath.Join(tmp, "outputs.env")
	if err := os.WriteFile(outFile, []byte("OPENAI_API_KEY=\nOTHER=2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	envMap := map[string]string{"OPENAI_API_KEY": "sk-from-host", "OTHER": "1"}
	dockerEnv := map[string]string{"OPENAI_API_KEY": "sk-from-host", "OTHER": "1"}
	applyOutputsFile(outFile, envMap, dockerEnv, nil, nil, "")

	if envMap["OPENAI_API_KEY"] != "sk-from-host" || dockerEnv["OPENAI_API_KEY"] != "sk-from-host" {
		t.Fatalf("secret should not be wiped: env=%#v docker=%#v", envMap, dockerEnv)
	}
	if envMap["OTHER"] != "2" || dockerEnv["OTHER"] != "2" {
		t.Fatalf("non-secret should merge: env=%#v docker=%#v", envMap, dockerEnv)
	}
}
