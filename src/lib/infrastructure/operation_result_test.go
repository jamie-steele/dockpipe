package infrastructure

import (
	"errors"
	"os"
	"strings"
	"testing"
	"time"
)

func TestRunOperationLogsStableResultLines(t *testing.T) {
	oldNow := timeNowDockerFn
	oldTTY := isTerminalDockerFn
	t.Cleanup(func() {
		timeNowDockerFn = oldNow
		isTerminalDockerFn = oldTTY
	})
	now := time.Unix(1000, 0)
	timeNowDockerFn = func() time.Time {
		current := now
		now = now.Add(25 * time.Millisecond)
		return current
	}
	isTerminalDockerFn = func(fd int) bool { return false }
	stderr, err := os.CreateTemp(t.TempDir(), "stderr")
	if err != nil {
		t.Fatal(err)
	}
	defer stderr.Close()

	if err := RunOperation(stderr, "session.volume.seed", "Seeding session workspace volume…", map[string]string{
		"session":   "run-1842",
		"volume":    "dockpipe-ws-demo",
		"workspace": "demo",
	}, func() error {
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
	got := string(out)
	if !strings.Contains(got, "[dockpipe] unit=session.volume.seed status=start session=run-1842 volume=dockpipe-ws-demo workspace=demo") {
		t.Fatalf("expected start result line, got:\n%s", got)
	}
	if !strings.Contains(got, "[dockpipe] unit=session.volume.seed status=done duration_ms=25 session=run-1842 volume=dockpipe-ws-demo workspace=demo") {
		t.Fatalf("expected done result line with duration, got:\n%s", got)
	}
}

func TestRunOperationLogsFailureResultLine(t *testing.T) {
	oldNow := timeNowDockerFn
	oldTTY := isTerminalDockerFn
	t.Cleanup(func() {
		timeNowDockerFn = oldNow
		isTerminalDockerFn = oldTTY
	})
	now := time.Unix(1000, 0)
	timeNowDockerFn = func() time.Time {
		current := now
		now = now.Add(10 * time.Millisecond)
		return current
	}
	isTerminalDockerFn = func(fd int) bool { return false }
	stderr, err := os.CreateTemp(t.TempDir(), "stderr")
	if err != nil {
		t.Fatal(err)
	}
	defer stderr.Close()

	wantErr := errors.New("git clone failed")
	err = RunOperation(stderr, "session.volume.seed", "Seeding session workspace volume…", map[string]string{
		"volume": "dockpipe-ws-demo",
	}, func() error {
		return wantErr
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected wrapped operation error, got %v", err)
	}
	if err := stderr.Close(); err != nil {
		t.Fatalf("close stderr: %v", err)
	}
	out, err := os.ReadFile(stderr.Name())
	if err != nil {
		t.Fatalf("read stderr: %v", err)
	}
	got := string(out)
	if !strings.Contains(got, `[dockpipe] unit=session.volume.seed status=fail duration_ms=10 volume=dockpipe-ws-demo error="git clone failed"`) {
		t.Fatalf("expected fail result line, got:\n%s", got)
	}
}
