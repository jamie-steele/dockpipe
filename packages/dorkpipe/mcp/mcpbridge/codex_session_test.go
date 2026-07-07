package mcpbridge

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestShouldDiscoverCodexSessionID(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		resumed  bool
		known    string
		expected bool
	}{
		{name: "new session discovers id", resumed: false, known: "", expected: true},
		{name: "resumed without known id discovers id", resumed: true, known: "", expected: true},
		{name: "resumed with known id skips discovery", resumed: true, known: "abc123", expected: false},
		{name: "resumed with whitespace known id discovers id", resumed: true, known: "  ", expected: true},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := shouldDiscoverCodexSessionID(tt.resumed, tt.known); got != tt.expected {
				t.Fatalf("shouldDiscoverCodexSessionID(%v, %q) = %v, want %v", tt.resumed, tt.known, got, tt.expected)
			}
		})
	}
}

func TestCodexSessionStatePathUsesRepoLocalBridgeRoot(t *testing.T) {
	root := t.TempDir()
	t.Setenv("DOCKPIPE_MCP_REPO_ROOT", root)

	got, err := codexSessionStatePath()
	if err != nil {
		t.Fatal(err)
	}
	wantSuffix := filepath.Join("bin", ".dockpipe", "packages", "dorkpipe", "host-bridge", "codex-sessions.json")
	if filepath.Clean(got) != filepath.Clean(filepath.Join(root, wantSuffix)) {
		t.Fatalf("got %q want repo-local suffix %q under %q", got, wantSuffix, root)
	}
	if strings.Contains(filepath.ToSlash(got), "../") {
		t.Fatalf("state path should not contain traversal: %q", got)
	}
}

func TestCodexSessionStateRoundTrip(t *testing.T) {
	root := t.TempDir()
	t.Setenv("DOCKPIPE_MCP_REPO_ROOT", root)

	state := codexSessionState{Sessions: map[string]codexSessionBinding{
		"pipeon": {CodexSessionID: "codex-session", Workdir: root, Model: "config", UpdatedAt: "now"},
	}}
	if err := saveCodexSessionState(state); err != nil {
		t.Fatal(err)
	}
	got, err := loadCodexSessionState()
	if err != nil {
		t.Fatal(err)
	}
	if got.Sessions["pipeon"].CodexSessionID != "codex-session" {
		t.Fatalf("unexpected state: %#v", got)
	}
}
