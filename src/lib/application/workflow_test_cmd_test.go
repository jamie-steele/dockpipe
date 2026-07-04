package application

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunWorkflowTestsFromFlagsRunsWorkflowLocalTests(t *testing.T) {
	repo := t.TempDir()
	if err := os.WriteFile(filepath.Join(repo, "dockpipe.config.json"), []byte(`{"compile":{"workflows":["workflows"]}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	wfDir := filepath.Join(repo, "workflows", "demo")
	if err := os.MkdirAll(filepath.Join(wfDir, "tests"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wfDir, "config.yml"), []byte("name: demo\nsteps: []\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wfDir, "tests", "run.sh"), []byte("#!/usr/bin/env bash\nset -euo pipefail\nprintf ok > \"$DOCKPIPE_WORKDIR/demo.workflow.test\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	stderr, err := captureResultStderr(t, func() error {
		return RunWorkflowTestsFromFlags(repo, "")
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"unit=workflow.test.workflows",
		"unit=workflow.test.workflow",
		"workflow=demo",
		"script=tests/run.sh",
		"status=done",
	} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("expected workflow test stderr to contain %q, got:\n%s", want, stderr)
		}
	}
	if _, err := os.Stat(filepath.Join(repo, "demo.workflow.test")); err != nil {
		t.Fatalf("expected workflow test output: %v", err)
	}
}

func TestRunWorkflowTestsFromFlagsNoMatchEmitsNoopResult(t *testing.T) {
	repo := t.TempDir()
	if err := os.WriteFile(filepath.Join(repo, "dockpipe.config.json"), []byte(`{"compile":{"workflows":["workflows"]}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	stderr, err := captureResultStderr(t, func() error {
		return RunWorkflowTestsFromFlags(repo, "missing")
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"unit=workflow.test.workflows",
		"workflow=missing",
		"result=noop",
		"status=done",
	} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("expected noop workflow test stderr to contain %q, got:\n%s", want, stderr)
		}
	}
}

func TestRunWorkflowTestsFromFlagsUsesConfiguredRoots(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	if err := os.WriteFile(filepath.Join(repo, "dockpipe.config.json"), []byte(`{"compile":{"workflows":["custom-workflows"]}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	wfDir := filepath.Join(repo, "custom-workflows", "alpha")
	if err := os.MkdirAll(filepath.Join(wfDir, "tests"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wfDir, "config.yml"), []byte("name: alpha\nsteps: []\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wfDir, "tests", "run.sh"), []byte("#!/usr/bin/env bash\nset -euo pipefail\nprintf ok > \"$DOCKPIPE_WORKDIR/alpha.workflow.test\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := RunWorkflowTestsFromFlags(repo, "alpha"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(repo, "alpha.workflow.test")); err != nil {
		t.Fatalf("expected configured-root workflow test output: %v", err)
	}
}
