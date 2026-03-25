package infrastructure

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"dockpipe/src/lib/dockpipe/infrastructure/packagebuild"
)

// TestListWorkflowNamesInRepoRoot lists workflows/<name>/ for a normal project.
func TestListWorkflowNamesInRepoRoot(t *testing.T) {
	tmp := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmp, "workflows", "a"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "workflows", "a", "config.yml"), []byte("name: a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(tmp, "workflows", "b"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "workflows", "b", "config.yml"), []byte("name: b\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(tmp, "workflows", "emptydir"), 0o755); err != nil {
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

func TestListWorkflowNamesInRepoRootAndPackagesMerges(t *testing.T) {
	tmp := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmp, "workflows", "a"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "workflows", "a", "config.yml"), []byte("name: a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	st := t.TempDir()
	if err := os.WriteFile(filepath.Join(st, "config.yml"), []byte("name: b\nsteps: []\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(st, "package.yml"), []byte("schema: 1\nname: b\nversion: 0.1.0\nkind: workflow\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	pw := filepath.Join(tmp, ".dockpipe", "internal", "packages", "workflows")
	if err := os.MkdirAll(pw, 0o755); err != nil {
		t.Fatal(err)
	}
	tgz := filepath.Join(pw, "dockpipe-workflow-b-0.1.0.tar.gz")
	if _, err := packagebuild.WriteDirTarGzWithPrefix(st, tgz, "workflows/b"); err != nil {
		t.Fatal(err)
	}
	got, err := ListWorkflowNamesInRepoRootAndPackages(tmp, tmp)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(got, []string{"a", "b"}) {
		t.Fatalf("got %#v", got)
	}
}

func TestListWorkflowNamesInRepoRootAndPackagesMergesGlobal(t *testing.T) {
	tmp := t.TempDir()
	glob := t.TempDir()
	t.Setenv("DOCKPIPE_GLOBAL_ROOT", glob)
	if err := os.MkdirAll(filepath.Join(tmp, "workflows", "local"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "workflows", "local", "config.yml"), []byte("name: local\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	st := t.TempDir()
	if err := os.WriteFile(filepath.Join(st, "config.yml"), []byte("name: globalwf\nsteps: []\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(st, "package.yml"), []byte("schema: 1\nname: globalwf\nversion: 0.1.0\nkind: workflow\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	pw := filepath.Join(glob, "packages", "workflows")
	if err := os.MkdirAll(pw, 0o755); err != nil {
		t.Fatal(err)
	}
	tgz := filepath.Join(pw, "dockpipe-workflow-globalwf-0.1.0.tar.gz")
	if _, err := packagebuild.WriteDirTarGzWithPrefix(st, tgz, "workflows/globalwf"); err != nil {
		t.Fatal(err)
	}
	got, err := ListWorkflowNamesInRepoRootAndPackages(tmp, "")
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(got, []string{"globalwf", "local"}) {
		t.Fatalf("got %#v", got)
	}
}

func TestResolveWorkflowConfigPathGlobalFallback(t *testing.T) {
	tmp := t.TempDir()
	glob := t.TempDir()
	t.Setenv("DOCKPIPE_GLOBAL_ROOT", glob)
	st := t.TempDir()
	if err := os.WriteFile(filepath.Join(st, "config.yml"), []byte("name: onlyglobal\nsteps: []\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(st, "package.yml"), []byte("schema: 1\nname: onlyglobal\nversion: 0.1.0\nkind: workflow\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	pw := filepath.Join(glob, "packages", "workflows")
	if err := os.MkdirAll(pw, 0o755); err != nil {
		t.Fatal(err)
	}
	tgz := filepath.Join(pw, "dockpipe-workflow-onlyglobal-0.1.0.tar.gz")
	if _, err := packagebuild.WriteDirTarGzWithPrefix(st, tgz, "workflows/onlyglobal"); err != nil {
		t.Fatal(err)
	}
	got, err := ResolveWorkflowConfigPathWithWorkdir(tmp, tmp, "onlyglobal")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(got, "tar://") {
		t.Fatalf("want tar workflow URI, got %s", got)
	}
}

func TestListWorkflowNamesInRepoRootIncludesDockpipeWorkflows(t *testing.T) {
	tmp := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmp, "workflows", "t1"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "workflows", "t1", "config.yml"), []byte("name: t1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(tmp, "workflows", "local"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "workflows", "local", "config.yml"), []byte("name: local\n"), 0o644); err != nil {
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

func TestResolveWorkflowConfigPathPrefersTemplatesWorkflow(t *testing.T) {
	tmp := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmp, "workflows", "demo"), 0o755); err != nil {
		t.Fatal(err)
	}
	wf := filepath.Join(tmp, "workflows", "demo", "config.yml")
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

func TestResolveWorkflowConfigPathPrefersWorkflowsOverLegacyTemplates(t *testing.T) {
	tmp := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmp, "templates", "demo"), 0o755); err != nil {
		t.Fatal(err)
	}
	leg := filepath.Join(tmp, "templates", "demo", "config.yml")
	if err := os.WriteFile(leg, []byte("name: legacy\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(tmp, "workflows", "demo"), 0o755); err != nil {
		t.Fatal(err)
	}
	modern := filepath.Join(tmp, "workflows", "demo", "config.yml")
	if err := os.WriteFile(modern, []byte("name: modern\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := ResolveWorkflowConfigPath(tmp, "demo")
	if err != nil {
		t.Fatal(err)
	}
	if got != modern {
		t.Fatalf("want workflows path %s got %s", modern, got)
	}
}

func TestResolveWorkflowConfigPathWithWorkdirPrefersPackagesOverLegacyTemplates(t *testing.T) {
	tmp := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmp, "templates", "core"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(tmp, "templates", "demo"), 0o755); err != nil {
		t.Fatal(err)
	}
	leg := filepath.Join(tmp, "templates", "demo", "config.yml")
	if err := os.WriteFile(leg, []byte("name: legacy\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	st := t.TempDir()
	if err := os.WriteFile(filepath.Join(st, "config.yml"), []byte("name: pkg\nsteps: []\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(st, "package.yml"), []byte("schema: 1\nname: demo\nversion: 0.1.0\nkind: workflow\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	pw := filepath.Join(tmp, ".dockpipe", "internal", "packages", "workflows")
	if err := os.MkdirAll(pw, 0o755); err != nil {
		t.Fatal(err)
	}
	tgz := filepath.Join(pw, "dockpipe-workflow-demo-0.1.0.tar.gz")
	if _, err := packagebuild.WriteDirTarGzWithPrefix(st, tgz, "workflows/demo"); err != nil {
		t.Fatal(err)
	}
	got, err := ResolveWorkflowConfigPathWithWorkdir(tmp, tmp, "demo")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(got, "tar://") {
		t.Fatalf("want tar workflow URI from package store, got %s", got)
	}
}

func TestResolveWorkflowConfigPathLegacyTemplatesWhenWorkflowsMissing(t *testing.T) {
	tmp := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmp, "templates", "demo"), 0o755); err != nil {
		t.Fatal(err)
	}
	leg := filepath.Join(tmp, "templates", "demo", "config.yml")
	if err := os.WriteFile(leg, []byte("name: demo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := ResolveWorkflowConfigPath(tmp, "demo")
	if err != nil {
		t.Fatal(err)
	}
	if got != leg {
		t.Fatalf("want legacy templates path %s got %s", leg, got)
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
