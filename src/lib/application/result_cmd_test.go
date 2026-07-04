package application

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dockpipe/src/lib/infrastructure"
)

func captureResultStderr(t *testing.T, fn func() error) (string, error) {
	t.Helper()
	old := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stderr = w
	runErr := fn()
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	os.Stderr = old
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatal(err)
	}
	if err := r.Close(); err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(buf.String()), runErr
}

func TestCmdResultEmitsCanonicalLineAndEvent(t *testing.T) {
	eventLog := filepath.Join(t.TempDir(), "events.jsonl")
	stderr, err := captureResultStderr(t, func() error {
		return cmdResult([]string{
			"--unit", "package.compile.workflow",
			"--status", infrastructure.OperationStatusDone,
			"--duration-ms", "42",
			"--id", "package=dorkpipe",
			"--id", "workflow=brain.optimize",
			"--event-log", eventLog,
		})
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"[dockpipe]",
		"unit=package.compile.workflow",
		"status=done",
		"duration_ms=42",
		"package=dorkpipe",
		"workflow=brain.optimize",
	} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("expected stderr to contain %q, got:\n%s", want, stderr)
		}
	}
	events, err := infrastructure.ReadOperationEvents(eventLog)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Fatalf("events len = %d want 1: %+v", len(events), events)
	}
	if events[0].Schema != infrastructure.OperationEventSchemaV1 || events[0].Unit != "package.compile.workflow" || events[0].Status != infrastructure.OperationStatusDone {
		t.Fatalf("unexpected event: %+v", events[0])
	}
	if events[0].DurationMs == nil || *events[0].DurationMs != 42 {
		t.Fatalf("event duration = %v want 42", events[0].DurationMs)
	}
	if events[0].IDs["package"] != "dorkpipe" || events[0].IDs["workflow"] != "brain.optimize" {
		t.Fatalf("event ids = %+v", events[0].IDs)
	}
}

func TestCmdResultValidatesRequiredFields(t *testing.T) {
	if err := cmdResult([]string{"--status", infrastructure.OperationStatusDone}); err == nil || !strings.Contains(err.Error(), "--unit is required") {
		t.Fatalf("expected missing unit error, got %v", err)
	}
	if err := cmdResult([]string{"--unit", "x", "--status", "ok"}); err == nil || !strings.Contains(err.Error(), "--status must be") {
		t.Fatalf("expected invalid status error, got %v", err)
	}
	if err := cmdResult([]string{"--unit", "x", "--status", infrastructure.OperationStatusDone, "--id", "nope"}); err == nil || !strings.Contains(err.Error(), "--id requires key=value") {
		t.Fatalf("expected invalid id error, got %v", err)
	}
}

func TestRunResultDispatchDoesNotRequireRepoRoot(t *testing.T) {
	stderr, err := captureResultStderr(t, func() error {
		return Run([]string{"result", "--unit", "script.step", "--status", infrastructure.OperationStatusStart}, nil)
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stderr, "unit=script.step") || !strings.Contains(stderr, "status=start") {
		t.Fatalf("unexpected result dispatch stderr:\n%s", stderr)
	}
}
