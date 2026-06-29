package infrastructure

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunHostScriptRunsSubprocessWithInheritedStdio(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "x.sh")
	if err := os.WriteFile(script, []byte("#!/usr/bin/env bash\necho OUT\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := RunHostScript(script, os.Environ()); err != nil {
		t.Fatalf("RunHostScript: %v", err)
	}
}

func TestRunHostCommandUsesStepCWD(t *testing.T) {
	workdir := t.TempDir()
	cwd := filepath.Join(workdir, "artifact-root")
	if err := os.MkdirAll(cwd, 0o755); err != nil {
		t.Fatal(err)
	}
	env := append(os.Environ(),
		"DOCKPIPE_WORKDIR="+workdir,
		"DOCKPIPE_STEP_CWD="+cwd,
		"DOCKPIPE_SKIP_HOST_CLEANUP=1",
	)
	if err := RunHostCommand("pwd > pwd.txt", env); err != nil {
		t.Fatalf("RunHostCommand: %v", err)
	}
	body, err := os.ReadFile(filepath.Join(cwd, "pwd.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(string(body)); got != cwd {
		t.Fatalf("pwd = %q want %q", got, cwd)
	}
}

func TestEnvGetUsesLastDuplicate(t *testing.T) {
	env := []string{"A=old", "B=1", "A=new"}
	if got := envGet(env, "A"); got != "new" {
		t.Fatalf("envGet duplicate = %q want new", got)
	}
}
