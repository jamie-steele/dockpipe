package application

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCmdWorkflowValidateEmitsOperationResult(t *testing.T) {
	dir := t.TempDir()
	wf := filepath.Join(dir, "config.yml")
	if err := os.WriteFile(wf, []byte("name: demo\nsteps: []\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	stderr, err := captureResultStderr(t, func() error {
		return cmdWorkflow([]string{"validate", wf})
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"unit=workflow.validate",
		"status=done",
		"path=",
	} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("expected workflow validate stderr to contain %q, got:\n%s", want, stderr)
		}
	}
	if strings.Contains(stderr, "OK: workflow") {
		t.Fatalf("expected canonical operation result instead of bespoke OK line, got:\n%s", stderr)
	}
}

func TestCmdWorkflowValidateFailureEmitsOperationResult(t *testing.T) {
	dir := t.TempDir()
	wf := filepath.Join(dir, "config.yml")
	if err := os.WriteFile(wf, []byte("name: demo\nsteps:\n  - kind: nope\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	stderr, err := captureResultStderr(t, func() error {
		return cmdWorkflow([]string{"validate", wf})
	})
	if err == nil {
		t.Fatal("expected validation error")
	}
	for _, want := range []string{
		"unit=workflow.validate",
		"status=fail",
		"path=",
		"error=",
	} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("expected workflow validate failure stderr to contain %q, got:\n%s", want, stderr)
		}
	}
}
