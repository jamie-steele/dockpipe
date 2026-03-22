package infrastructure

import (
	"path/filepath"
	"testing"
)

// TestEmbeddedWorkflowConfigExists matches known bundled template names.
func TestEmbeddedWorkflowConfigExists(t *testing.T) {
	if !EmbeddedWorkflowConfigExists("test") {
		t.Fatal("expected test")
	}
	if !EmbeddedWorkflowConfigExists("test-demo") {
		t.Fatal("expected test-demo")
	}
	if !EmbeddedWorkflowConfigExists("run") {
		t.Fatal("expected run")
	}
	if !EmbeddedWorkflowConfigExists("run-apply") {
		t.Fatal("expected run-apply")
	}
	if !EmbeddedWorkflowConfigExists("run-apply-validate") {
		t.Fatal("expected run-apply-validate")
	}
	if !EmbeddedWorkflowConfigExists("init") {
		t.Fatal("expected init")
	}
	if !EmbeddedWorkflowConfigExists("dogfood-codex-pav") {
		t.Fatal("expected dogfood-codex-pav")
	}
	if !EmbeddedWorkflowConfigExists("dogfood-codex-security") {
		t.Fatal("expected dogfood-codex-security")
	}
	for _, name := range []string{"vscode", "cursor-dev", "claude", "codex", "code-server"} {
		if !EmbeddedWorkflowConfigExists(name) {
			t.Fatalf("expected resolver delegate %s", name)
		}
	}
	if EmbeddedWorkflowConfigExists("") {
		t.Fatal("empty name should be false")
	}
	if EmbeddedWorkflowConfigExists("../x") {
		t.Fatal("path traversal should be false")
	}
	if EmbeddedWorkflowConfigExists("nope-not-a-real-template-xyz") {
		t.Fatal("unknown template should be false")
	}
}

func TestMapEmbeddedToMaterializedPath(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in, want string
	}{
		{"VERSION", "version"},
		{"lib/entrypoint.sh", "lib/entrypoint.sh"},
		{"templates", "dockpipe"},
		{"templates/core", "dockpipe/core"},
		{"templates/core/runtimes/docker/profile", "dockpipe/core/runtimes/docker/profile"},
		{"templates/test/config.yml", filepath.Join("dockpipe", "workflows", "test", "config.yml")},
		{"templates/test-demo/config.yml", filepath.Join("dockpipe", "workflows", "test-demo", "config.yml")},
		{filepath.Join("dockpipe", "workflows", "dogfood-codex-pav", "config.yml"), filepath.Join("dockpipe", "workflows", "dogfood-codex-pav", "config.yml")},
	}
	for _, tc := range cases {
		got := mapEmbeddedToMaterializedPath(tc.in)
		if got != tc.want {
			t.Fatalf("mapEmbeddedToMaterializedPath(%q): got %q want %q", tc.in, got, tc.want)
		}
	}
}
