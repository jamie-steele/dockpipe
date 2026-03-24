package infrastructure

import (
	"path/filepath"
	"testing"
)

// TestEmbeddedWorkflowConfigExists checks bundled user templates (src/templates/*/config.yml) and
// resolver delegates (templates/core/resolvers/*/config.yml). It does not assert maintainer-only
// shipyard/workflows/* names — those churn independently of the core embed contract.
func TestEmbeddedWorkflowConfigExists(t *testing.T) {
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
		{"assets/entrypoint.sh", "assets/entrypoint.sh"},
		{EmbeddedTemplatesPrefix, ShipyardDir},
		{EmbeddedTemplatesPrefix + "/core", filepath.Join(ShipyardDir, "core")},
		{EmbeddedTemplatesPrefix + "/core/runtimes/docker/profile", filepath.Join(ShipyardDir, "core/runtimes/docker/profile")},
		{EmbeddedTemplatesPrefix + "/run/config.yml", filepath.Join(ShipyardDir, "workflows", "run", "config.yml")},
		// Already-materialized paths pass through unchanged (bundle cache layout uses ShipyardDir).
		{filepath.Join(ShipyardDir, "workflows", "init", "config.yml"), filepath.Join(ShipyardDir, "workflows", "init", "config.yml")},
	}
	for _, tc := range cases {
		got := mapEmbeddedToMaterializedPath(tc.in)
		if got != tc.want {
			t.Fatalf("mapEmbeddedToMaterializedPath(%q): got %q want %q", tc.in, got, tc.want)
		}
	}
}
