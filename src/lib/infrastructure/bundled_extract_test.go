package infrastructure

import (
	"path"
	"path/filepath"
	"testing"
)

// TestEmbeddedWorkflowConfigExists checks bundled user templates (src/core/workflows/*/config.yml) and
// resolver delegates (src/core/resolvers/*/config.yml and embedded maintainer packages for tool trees). It does not assert maintainer-only
// Extra embedded workflow names under workflows/ — those churn independently of the core embed contract.
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
	if !EmbeddedWorkflowConfigExists("secretstore") {
		t.Fatal("expected secretstore workflow template")
	}
	if !EmbeddedWorkflowConfigExists("dorkpipe-self-analysis") {
		t.Fatal("expected dorkpipe-self-analysis from maintainer dorkpipe plugin")
	}
	if !EmbeddedWorkflowConfigExists("user-insight-process") {
		t.Fatal("expected user-insight-process from maintainer dorkpipe plugin")
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
	pfx0 := embeddedPackageRootsPrefixes[0]
	pfx1 := embeddedPackageRootsPrefixes[1]
	cases := []struct {
		in, want string
	}{
		{"VERSION", "version"},
		{"assets/entrypoint.sh", "assets/entrypoint.sh"},
		{EmbeddedTemplatesPrefix, filepath.Join(ShipyardDir, "core")},
		{EmbeddedTemplatesPrefix + "/runtimes/dockerimage/profile", filepath.Join(ShipyardDir, "core/runtimes/dockerimage/profile")},
		{EmbeddedTemplatesPrefix + "/workflows/run/config.yml", filepath.Join(ShipyardDir, "workflows", "run", "config.yml")},
		{path.Join("workflows", "test", "config.yml"), filepath.Join(ShipyardDir, "workflows", "test", "config.yml")},
		{path.Join(pfx0, "cloud", "storage", "resolvers", "r2", "dockpipe.cloudflare.r2infra", "config.yml"), filepath.Join(ShipyardDir, "workflows", "dockpipe.cloudflare.r2infra", "config.yml")},
		{path.Join(pfx1, "ide", "resolvers", "vscode", "config.yml"), filepath.Join(ShipyardDir, "workflows", "vscode", "config.yml")},
		{path.Join(pfx0, "pipeon", "resolvers", "pipeon", "config.yml"), filepath.Join(ShipyardDir, "workflows", "pipeon", "config.yml")},
		{path.Join(pfx0, "dorkpipe", "resolvers", "dorkpipe", "config.yml"), filepath.Join(ShipyardDir, "workflows", "dorkpipe", "config.yml")},
		{path.Join(pfx0, "dorkpipe", "resolvers", "user-insight-process", "config.yml"), filepath.Join(ShipyardDir, "workflows", "user-insight-process", "config.yml")},
		{path.Join(pfx0, "dorkpipe", "resolvers", "dorkpipe-self-analysis", "config.yml"), filepath.Join(ShipyardDir, "workflows", "dorkpipe-self-analysis", "config.yml")},
		{"workflows", filepath.Join(ShipyardDir, "workflows")},
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
