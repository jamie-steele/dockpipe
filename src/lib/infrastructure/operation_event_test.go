package infrastructure

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestOperationEventFromResultMapsCanonicalFields(t *testing.T) {
	result := OperationResult{
		Unit:       "build.compile",
		Status:     OperationStatusDone,
		Message:    "Compiling DockPipe packages...",
		StartedAt:  time.Unix(1000, 0),
		FinishedAt: time.Unix(1000, int64(25*time.Millisecond)),
		DurationMs: 25,
		IDs: map[string]string{
			"project": "dockpipe",
		},
	}
	event := OperationEventFromResult(result)
	if event.Schema != OperationEventSchemaV1 {
		t.Fatalf("Schema = %q want %q", event.Schema, OperationEventSchemaV1)
	}
	if event.Type != OperationEventKind {
		t.Fatalf("Type = %q want %q", event.Type, OperationEventKind)
	}
	if event.Timestamp != "1970-01-01T00:16:40.025Z" {
		t.Fatalf("Timestamp = %q", event.Timestamp)
	}
	if event.Unit != "build.compile" || event.Status != OperationStatusDone {
		t.Fatalf("unexpected event unit/status: %+v", event)
	}
	if event.DurationMs == nil || *event.DurationMs != 25 {
		t.Fatalf("DurationMs = %v want 25", event.DurationMs)
	}
	if event.IDs["project"] != "dockpipe" {
		t.Fatalf("IDs = %+v", event.IDs)
	}
}

func TestAppendAndReadOperationEventsJSONL(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events", "events.jsonl")
	if err := AppendOperationEvent(path, OperationEvent{
		Unit:   "build.compile",
		Status: OperationStatusStart,
		IDs: map[string]string{
			"project": "dockpipe",
		},
	}); err != nil {
		t.Fatalf("AppendOperationEvent start: %v", err)
	}
	duration := int64(12)
	if err := AppendOperationEvent(path, OperationEvent{
		Unit:       "build.compile",
		Status:     OperationStatusDone,
		DurationMs: &duration,
	}); err != nil {
		t.Fatalf("AppendOperationEvent done: %v", err)
	}
	events, err := ReadOperationEvents(path)
	if err != nil {
		t.Fatalf("ReadOperationEvents: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("len(events) = %d want 2", len(events))
	}
	if events[0].Schema != OperationEventSchemaV1 || events[0].Type != OperationEventKind {
		t.Fatalf("event defaults not applied: %+v", events[0])
	}
	if events[0].IDs["project"] != "dockpipe" || events[1].DurationMs == nil || *events[1].DurationMs != 12 {
		t.Fatalf("unexpected events: %+v", events)
	}
}

func TestLogOperationResultMirrorsToConfiguredEventLog(t *testing.T) {
	oldNow := timeNowDockerFn
	oldEnv := os.Getenv(EnvDockpipeEventLog)
	t.Cleanup(func() {
		timeNowDockerFn = oldNow
		if oldEnv == "" {
			os.Unsetenv(EnvDockpipeEventLog)
		} else {
			os.Setenv(EnvDockpipeEventLog, oldEnv)
		}
	})
	now := time.Unix(1000, 0)
	timeNowDockerFn = func() time.Time { return now }
	path := filepath.Join(t.TempDir(), "events.jsonl")
	os.Setenv(EnvDockpipeEventLog, path)
	stderr, err := os.CreateTemp(t.TempDir(), "stderr")
	if err != nil {
		t.Fatal(err)
	}
	defer stderr.Close()

	LogOperationResult(stderr, OperationResult{
		Unit:       "build.clean",
		Status:     OperationStatusDone,
		FinishedAt: now,
		DurationMs: 0,
		IDs: map[string]string{
			"result": "noop",
		},
	})

	events, err := ReadOperationEvents(path)
	if err != nil {
		t.Fatalf("ReadOperationEvents: %v", err)
	}
	if len(events) != 1 || events[0].Unit != "build.clean" || events[0].IDs["result"] != "noop" {
		t.Fatalf("unexpected mirrored events: %+v", events)
	}
}

func TestRunOperationWithSpinnerMirrorsStartEventWithoutStartLine(t *testing.T) {
	oldNow := timeNowDockerFn
	oldTTY := isTerminalDockerFn
	oldEnv := os.Getenv(EnvDockpipeEventLog)
	t.Cleanup(func() {
		timeNowDockerFn = oldNow
		isTerminalDockerFn = oldTTY
		if oldEnv == "" {
			os.Unsetenv(EnvDockpipeEventLog)
		} else {
			os.Setenv(EnvDockpipeEventLog, oldEnv)
		}
	})
	now := time.Unix(1000, 0)
	timeNowDockerFn = func() time.Time {
		current := now
		now = now.Add(10 * time.Millisecond)
		return current
	}
	isTerminalDockerFn = func(fd int) bool { return true }
	path := filepath.Join(t.TempDir(), "events.jsonl")
	os.Setenv(EnvDockpipeEventLog, path)
	stderr, err := os.CreateTemp(t.TempDir(), "stderr")
	if err != nil {
		t.Fatal(err)
	}
	defer stderr.Close()

	if err := RunOperation(stderr, "session.volume.seed", "Seeding session workspace volume...", nil, func() error {
		return nil
	}); err != nil {
		t.Fatalf("RunOperation: %v", err)
	}
	if err := stderr.Close(); err != nil {
		t.Fatalf("close stderr: %v", err)
	}
	out, err := os.ReadFile(stderr.Name())
	if err != nil {
		t.Fatalf("read stderr: %v", err)
	}
	if strings.Contains(string(out), "status=start") {
		t.Fatalf("terminal spinner path should not print start line, got:\n%s", string(out))
	}
	events, err := ReadOperationEvents(path)
	if err != nil {
		t.Fatalf("ReadOperationEvents: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("len(events) = %d want 2: %+v", len(events), events)
	}
	if events[0].Status != OperationStatusStart || events[1].Status != OperationStatusDone {
		t.Fatalf("unexpected event statuses: %+v", events)
	}
}

func TestReadOperationEventsReportsMalformedLine(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.jsonl")
	if err := os.WriteFile(path, []byte("{bad-json}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := ReadOperationEvents(path)
	if err == nil || !strings.Contains(err.Error(), ":1:") {
		t.Fatalf("expected line-numbered malformed JSON error, got %v", err)
	}
}
