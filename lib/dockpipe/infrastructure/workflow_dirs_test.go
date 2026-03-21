package infrastructure

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// TestListWorkflowNamesInRepoRoot lists templates/<name>/ (excluding templates/core).
func TestListWorkflowNamesInRepoRoot(t *testing.T) {
	tmp := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmp, "templates", "a"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "templates", "a", "config.yml"), []byte("name: a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(tmp, "templates", "b"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "templates", "b", "config.yml"), []byte("name: b\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(tmp, "templates", "emptydir"), 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := ListWorkflowNamesInRepoRoot(tmp)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(got, []string{"a", "b"}) {
		t.Fatalf("got %#v", got)
	}
}

func TestListWorkflowNamesInRepoRootIncludesDockpipeWorkflows(t *testing.T) {
	tmp := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmp, "templates", "t1"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "templates", "t1", "config.yml"), []byte("name: t1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(tmp, "dockpipe", "workflows", "local"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "dockpipe", "workflows", "local", "config.yml"), []byte("name: local\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := ListWorkflowNamesInRepoRoot(tmp)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(got, []string{"local", "t1"}) {
		t.Fatalf("got %#v", got)
	}
}

func TestResolveWorkflowConfigPathPrefersAuthoringDockpipeWorkflowsOverTemplates(t *testing.T) {
	tmp := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmp, "templates", "demo"), 0o755); err != nil {
		t.Fatal(err)
	}
	templatesWf := filepath.Join(tmp, "templates", "demo", "config.yml")
	if err := os.WriteFile(templatesWf, []byte("name: from-templates\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(tmp, "dockpipe", "workflows", "demo"), 0o755); err != nil {
		t.Fatal(err)
	}
	localWf := filepath.Join(tmp, "dockpipe", "workflows", "demo", "config.yml")
	if err := os.WriteFile(localWf, []byte("name: from-dockpipe\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := ResolveWorkflowConfigPath(tmp, "demo")
	if err != nil {
		t.Fatal(err)
	}
	if got != localWf {
		t.Fatalf("want repo-local workflow %s got %s", localWf, got)
	}
}

func TestResolveWorkflowConfigPathPrefersTemplatesWorkflow(t *testing.T) {
	tmp := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmp, "templates", "demo"), 0o755); err != nil {
		t.Fatal(err)
	}
	wf := filepath.Join(tmp, "templates", "demo", "config.yml")
	if err := os.WriteFile(wf, []byte("name: demo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Resolver delegate YAML (would be second choice).
	if err := os.MkdirAll(filepath.Join(tmp, "templates", "core", "resolvers", "demo"), 0o755); err != nil {
		t.Fatal(err)
	}
	rs := filepath.Join(tmp, "templates", "core", "resolvers", "demo", "config.yml")
	if err := os.WriteFile(rs, []byte("name: delegate\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := ResolveWorkflowConfigPath(tmp, "demo")
	if err != nil {
		t.Fatal(err)
	}
	if got != wf {
		t.Fatalf("want workflow path %s got %s", wf, got)
	}
}

func TestResolveWorkflowConfigPathFallsBackToResolverDelegate(t *testing.T) {
	tmp := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmp, "templates", "core", "resolvers", "codex"), 0o755); err != nil {
		t.Fatal(err)
	}
	rs := filepath.Join(tmp, "templates", "core", "resolvers", "codex", "config.yml")
	if err := os.WriteFile(rs, []byte("name: codex\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := ResolveWorkflowConfigPath(tmp, "codex")
	if err != nil {
		t.Fatal(err)
	}
	if got != rs {
		t.Fatalf("want resolver delegate %s got %s", rs, got)
	}
}

// TestResolveWorkflowConfigPathDoesNotSearchLegacyCoreWorkflowsDir ensures we do not load YAML
// from an obsolete nested "workflows" directory under core (not a valid workflow lookup path).
func TestResolveWorkflowConfigPathDoesNotSearchLegacyCoreWorkflowsDir(t *testing.T) {
	tmp := t.TempDir()
	legacyDir := filepath.Join(tmp, "templates", "core", "workflows", "ghost")
	if err := os.MkdirAll(legacyDir, 0o755); err != nil {
		t.Fatal(err)
	}
	legacy := filepath.Join(legacyDir, "config.yml")
	if err := os.WriteFile(legacy, []byte("name: ghost\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := ResolveWorkflowConfigPath(tmp, "ghost")
	if err == nil {
		t.Fatal("expected error: workflow ghost must not resolve from obsolete core workflows tree")
	}
}

func TestResolveEmbeddedResolverWorkflowConfigPathPrefersCoreResolverThenWorkflow(t *testing.T) {
	tmp := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmp, "templates", "core", "resolvers", "vscode"), 0o755); err != nil {
		t.Fatal(err)
	}
	coreCfg := filepath.Join(tmp, "templates", "core", "resolvers", "vscode", "config.yml")
	if err := os.WriteFile(coreCfg, []byte("steps: []\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(tmp, "templates", "vscode"), 0o755); err != nil {
		t.Fatal(err)
	}
	wfCfg := filepath.Join(tmp, "templates", "vscode", "config.yml")
	if err := os.WriteFile(wfCfg, []byte("steps: []\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := ResolveEmbeddedResolverWorkflowConfigPath(tmp, "vscode")
	if err != nil {
		t.Fatal(err)
	}
	if got != coreCfg {
		t.Fatalf("want core resolver delegate first %s got %s", coreCfg, got)
	}
}
