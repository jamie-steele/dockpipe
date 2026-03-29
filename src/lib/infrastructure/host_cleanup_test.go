package infrastructure

import (
	"os"
	"path/filepath"
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
