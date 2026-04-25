package infrastructure

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestIsSafeDockerContainerName(t *testing.T) {
	t.Parallel()
	cases := []struct {
		s    string
		want bool
	}{
		{"dockpipe-cursor-dev-12345", true},
		{"a", true},
		{"", false},
		{"-bad", false},
		{"foo;bar", false},
		{strings.Repeat("a", 300), false},
	}
	for _, tc := range cases {
		got := isSafeDockerContainerName(tc.s)
		if got != tc.want {
			t.Errorf("%q: got %v want %v", tc.s, got, tc.want)
		}
	}
}

func TestHostCleanupSkip(t *testing.T) {
	t.Parallel()
	env := []string{"DOCKPIPE_SKIP_HOST_CLEANUP=1", "DOCKPIPE_WORKDIR=/tmp"}
	if !hostCleanupSkip(env) {
		t.Fatal("expected skip")
	}
}

func TestIsValidHostRunID(t *testing.T) {
	t.Parallel()
	cases := []struct {
		id   string
		want bool
	}{
		{"a1b2c3d4", true},
		{"A1B2C3D4", true},
		{"", false},
		{"a1b2c3d", false},
		{"a1b2c3d45", false},
		{"../etc/pw", false},
		{"g1b2c3d4", false},
	}
	for _, tc := range cases {
		got := isValidHostRunID(tc.id)
		if got != tc.want {
			t.Errorf("%q: got %v want %v", tc.id, got, tc.want)
		}
	}
}

func TestRemoveCleanupMarkersForContainerName(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	cleanup := filepath.Join(root, DockpipeDirRel, "cleanup")
	if err := os.MkdirAll(cleanup, 0o755); err != nil {
		t.Fatal(err)
	}
	legacy := filepath.Join(root, DockpipeDirRel, "cursor-dev")
	if err := os.MkdirAll(legacy, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cleanup, "docker-session"), []byte("keep-other\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cleanup, "docker-code-server"), []byte("match-me\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacy, "session_container"), []byte("match-me\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	removeCleanupMarkersForContainerName(root, "match-me")
	if _, err := os.Stat(filepath.Join(cleanup, "docker-session")); err != nil {
		t.Fatal("expected docker-session to remain:", err)
	}
	if _, err := os.Stat(filepath.Join(cleanup, "docker-code-server")); err == nil {
		t.Fatal("expected docker-code-server removed")
	}
	if _, err := os.Stat(filepath.Join(legacy, "session_container")); err == nil {
		t.Fatal("expected session_container removed")
	}
}

func TestApplyHostCleanup_RunScopedOnlyStopsTrackedContainer(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	runs := HostRunsDir(root)
	cleanup := filepath.Join(root, DockpipeDirRel, "cleanup")
	legacy := filepath.Join(root, DockpipeDirRel, "cursor-dev")
	for _, dir := range []string{runs, cleanup, legacy} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	runID := "a1b2c3d4"
	if err := os.WriteFile(filepath.Join(runs, runID+".container"), []byte("tracked-container\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cleanup, "docker-session"), []byte("tracked-container\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cleanup, "docker-other"), []byte("other-container\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacy, "session_container"), []byte("tracked-container\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var got [][]string
	old := hostCleanupExecCommandFn
	hostCleanupExecCommandFn = func(ctx context.Context, name string, arg ...string) *exec.Cmd {
		got = append(got, append([]string{name}, arg...))
		return exec.CommandContext(ctx, "bash", "-lc", "exit 0")
	}
	defer func() { hostCleanupExecCommandFn = old }()

	ApplyHostCleanup([]string{
		"DOCKPIPE_WORKDIR=" + root,
		"DOCKPIPE_RUN_ID=" + runID,
	})

	want := [][]string{{"docker", "stop", "-t", "10", "tracked-container"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("docker calls mismatch:\n got %#v\nwant %#v", got, want)
	}
	if _, err := os.Stat(filepath.Join(runs, runID+".container")); !os.IsNotExist(err) {
		t.Fatalf("expected run sidecar removed, got %v", err)
	}
	if _, err := os.Stat(filepath.Join(cleanup, "docker-session")); !os.IsNotExist(err) {
		t.Fatalf("expected tracked cleanup marker removed, got %v", err)
	}
	if _, err := os.Stat(filepath.Join(legacy, "session_container")); !os.IsNotExist(err) {
		t.Fatalf("expected legacy marker removed, got %v", err)
	}
	if _, err := os.Stat(filepath.Join(cleanup, "docker-other")); err != nil {
		t.Fatalf("expected unrelated cleanup marker to remain, got %v", err)
	}
}

func TestApplyHostCleanup_InvalidRunIDDoesNothing(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	runs := HostRunsDir(root)
	if err := os.MkdirAll(runs, 0o755); err != nil {
		t.Fatal(err)
	}
	sidecar := filepath.Join(runs, "not-valid.container")
	if err := os.WriteFile(sidecar, []byte("tracked-container\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	called := false
	old := hostCleanupExecCommandFn
	hostCleanupExecCommandFn = func(ctx context.Context, name string, arg ...string) *exec.Cmd {
		called = true
		return exec.CommandContext(ctx, "bash", "-lc", "exit 0")
	}
	defer func() { hostCleanupExecCommandFn = old }()

	ApplyHostCleanup([]string{
		"DOCKPIPE_WORKDIR=" + root,
		"DOCKPIPE_RUN_ID=not-valid",
	})

	if called {
		t.Fatal("expected no docker calls for invalid run id")
	}
	if _, err := os.Stat(sidecar); err != nil {
		t.Fatalf("expected sidecar untouched for invalid run id, got %v", err)
	}
}
