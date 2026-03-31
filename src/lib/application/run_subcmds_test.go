package application

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dockpipe/src/lib/domain"
)

func writeFile(t *testing.T, path, content string, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), mode); err != nil {
		t.Fatal(err)
	}
}

func mkRepoRootForSubcmdTests(t *testing.T) string {
	t.Helper()
	repoRoot := t.TempDir()
	writeFile(t, filepath.Join(repoRoot, "templates", "init", "config.yml"), `name: example
description: Starter typed pipeline with a prepare step, an isolated run step, and a host summary step.
types:
  - models/IExampleWorkflowConfig.pipe
vars:
  EXAMPLE_MESSAGE: "hello from dockpipe"
  EXAMPLE_IMAGE: "alpine:3.22"
steps:
  - id: prepare
    skip_container: true
    cmd: echo prepare
    outputs: bin/.dockpipe/example.env
  - id: run
    isolate: ${EXAMPLE_IMAGE}
    cmd: echo run
  - id: report
    skip_container: true
    cmd: echo report
`, 0o644)
	writeFile(t, filepath.Join(repoRoot, "templates", "init", "README.md"), "# init\n", 0o644)
	writeFile(t, filepath.Join(repoRoot, "templates", "init", "models", "IExampleWorkflowConfig.pipe"), "public Interface IExampleWorkflowConfig { public string ExampleMessage; public string ExampleImage; }\n", 0o644)
	writeFile(t, filepath.Join(repoRoot, "templates", "init", "models", "ExampleWorkflowConfig.pipe"), "public Class ExampleWorkflowConfig : IExampleWorkflowConfig { public string ExampleMessage = \"hello from dockpipe\"; public string ExampleImage = \"alpine:3.22\"; }\n", 0o644)
	writeFile(t, filepath.Join(repoRoot, "templates", "run", "config.yml"), "name: run\nrun: []\n", 0o644)
	writeFile(t, filepath.Join(repoRoot, "templates", "core", "resolvers", "default"), "DOCKPIPE_RESOLVER_TEMPLATE=codex\n", 0o644)
	writeFile(t, filepath.Join(repoRoot, "templates", "core", "resolvers", "claude"), "DOCKPIPE_RESOLVER_TEMPLATE=claude\n", 0o644)
	writeFile(t, filepath.Join(repoRoot, "templates", "core", "assets", "scripts", "commit-worktree.sh"), "#!/usr/bin/env bash\n", 0o755)
	writeFile(t, filepath.Join(repoRoot, "templates", "core", "assets", "scripts", "clone-worktree.sh"), "#!/usr/bin/env bash\n", 0o755)
	return repoRoot
}

// TestCmdTemplateUsageAndUnknownTemplate checks template init usage and --from validation.
func TestCmdTemplateUsageAndUnknownTemplate(t *testing.T) {
	repoRoot := mkRepoRootForSubcmdTests(t)
	t.Setenv("DOCKPIPE_REPO_ROOT", repoRoot)

	if err := cmdTemplate([]string{}); err == nil || !strings.Contains(err.Error(), "usage: dockpipe template init") {
		t.Fatalf("expected usage error, got %v", err)
	}
	if err := cmdTemplate([]string{"init", "--from", "missing", "x"}); err == nil || !strings.Contains(err.Error(), "unknown bundled template") {
		t.Fatalf("expected unknown bundled template error, got %v", err)
	}
}

// TestCmdTemplateCreatesFromBundled copies a bundled template into a new directory.
func TestCmdTemplateCreatesFromBundled(t *testing.T) {
	repoRoot := mkRepoRootForSubcmdTests(t)
	t.Setenv("DOCKPIPE_REPO_ROOT", repoRoot)

	wd := t.TempDir()
	oldWd, _ := os.Getwd()
	if err := os.Chdir(wd); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldWd) })

	if err := cmdTemplate([]string{"init", "my-workflow"}); err != nil {
		t.Fatalf("cmdTemplate init failed: %v", err)
	}
	dest := filepath.Join(wd, "my-workflow")
	if _, err := os.Stat(filepath.Join(dest, "config.yml")); err != nil {
		t.Fatalf("expected copied config.yml: %v", err)
	}
	coreClaude := filepath.Join(wd, "templates", "core", "resolvers", "claude")
	if _, err := os.Stat(coreClaude); err != nil {
		t.Fatalf("expected shared templates/core/resolvers/claude: %v", err)
	}
}

// TestCmdInitLikeScriptCreateAndFromBundled covers dockpipe action init default and --from bundled script.
func TestCmdInitLikeScriptCreateAndFromBundled(t *testing.T) {
	repoRoot := mkRepoRootForSubcmdTests(t)
	t.Setenv("DOCKPIPE_REPO_ROOT", repoRoot)
	writeFile(t, filepath.Join(repoRoot, "templates", "core", "assets", "scripts", "print-summary.sh"), "#!/usr/bin/env bash\necho ok\n", 0o755)

	wd := t.TempDir()
	oldWd, _ := os.Getwd()
	if err := os.Chdir(wd); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldWd) })

	if err := cmdAction([]string{"init"}); err != nil {
		t.Fatalf("cmdAction init failed: %v", err)
	}
	created := filepath.Join(wd, "my-action.sh")
	b, err := os.ReadFile(created)
	if err != nil {
		t.Fatalf("expected generated action script: %v", err)
	}
	if !strings.Contains(string(b), "dockpipe action") {
		t.Fatalf("expected action boilerplate, got: %q", string(b))
	}

	if err := cmdAction([]string{"init", "--from", "print-summary", "from-bundle.sh"}); err != nil {
		t.Fatalf("cmdAction --from failed: %v", err)
	}
	fromBundle := filepath.Join(wd, "from-bundle.sh")
	if _, err := os.Stat(fromBundle); err != nil {
		t.Fatalf("expected bundled script copy: %v", err)
	}
	if err := cmdAction([]string{"init", "--from"}); err == nil || !strings.Contains(err.Error(), "--from requires argument") {
		t.Fatalf("expected --from validation error, got %v", err)
	}
}

// TestCmdPreInitCreatesDefaultScript writes my-pre.sh boilerplate in cwd.
func TestCmdPreInitCreatesDefaultScript(t *testing.T) {
	repoRoot := mkRepoRootForSubcmdTests(t)
	t.Setenv("DOCKPIPE_REPO_ROOT", repoRoot)

	wd := t.TempDir()
	oldWd, _ := os.Getwd()
	if err := os.Chdir(wd); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldWd) })

	if err := cmdPre([]string{"init"}); err != nil {
		t.Fatalf("cmdPre init failed: %v", err)
	}
	created := filepath.Join(wd, "my-pre.sh")
	b, err := os.ReadFile(created)
	if err != nil {
		t.Fatalf("expected generated pre script: %v", err)
	}
	if !strings.Contains(string(b), "dockpipe pre-script") {
		t.Fatalf("expected pre boilerplate, got: %q", string(b))
	}
}

// TestMergeBundledTemplatesCoreCopiesCoreTree copies templates/core from repoRoot into dest,
// including nested assets (full copy when source has no workflows/ subdir).
func TestMergeBundledTemplatesCoreCopiesCoreTree(t *testing.T) {
	repo := t.TempDir()
	dest := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, "templates", "core", "assets", "scripts"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "templates", "core", "assets", "scripts", "merge-marker.txt"), []byte("ok\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := mergeBundledTemplatesCore(repo, dest); err != nil {
		t.Fatalf("mergeBundledTemplatesCore: %v", err)
	}
	b, err := os.ReadFile(filepath.Join(dest, "templates", "core", "assets", "scripts", "merge-marker.txt"))
	if err != nil || string(b) != "ok\n" {
		t.Fatalf("got %v %q", err, string(b))
	}
}

// TestCmdInitCreatesWorkspaceAndMinimalWorkflow creates the minimal root scaffold and workflows/<name>/config.yml as a blank starter.
func TestCmdInitCreatesWorkspaceAndMinimalWorkflow(t *testing.T) {
	repoRoot := mkRepoRootForSubcmdTests(t)
	t.Setenv("DOCKPIPE_REPO_ROOT", repoRoot)

	project := t.TempDir()
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(project); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldWd) })

	if err := cmdInit([]string{"demo"}); err != nil {
		t.Fatalf("cmdInit failed: %v", err)
	}
	checks := []string{
		filepath.Join(project, "README.md"),
		filepath.Join(project, domain.DockpipeProjectConfigFileName),
		filepath.Join(project, ".env.vault.template.example"),
		filepath.Join(project, "workflows"),
		filepath.Join(project, "workflows", "demo", "config.yml"),
	}
	for _, p := range checks {
		if _, err := os.Stat(p); err != nil {
			t.Fatalf("expected created path %q: %v", p, err)
		}
	}
	b, err := os.ReadFile(filepath.Join(project, "workflows", "demo", "config.yml"))
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	if !strings.Contains(s, "demo") || !strings.Contains(s, "Dockpipe workflow") {
		t.Fatalf("expected blank starter config, got:\n%s", s)
	}
	if _, err := os.Stat(filepath.Join(project, "workflows", "demo", "README.md")); err == nil {
		t.Fatal("default init <name> should not copy bundled init template README; use --from init")
	}
	for _, p := range []string{
		filepath.Join(project, "scripts"),
		filepath.Join(project, "images"),
		filepath.Join(project, "templates", "core"),
	} {
		if _, err := os.Stat(p); err == nil {
			t.Fatalf("did not expect legacy scaffold path %q", p)
		}
	}
}

// TestCmdInitFromInitCopiesBundledInitTemplate restores the legacy copy of templates/init into workflows/<name>/.
func TestCmdInitFromInitCopiesBundledInitTemplate(t *testing.T) {
	repoRoot := mkRepoRootForSubcmdTests(t)
	t.Setenv("DOCKPIPE_REPO_ROOT", repoRoot)
	project := t.TempDir()
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(project); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldWd) })

	if err := cmdInit([]string{"legacy", "--from", "init"}); err != nil {
		t.Fatalf("cmdInit: %v", err)
	}
	if _, err := os.Stat(filepath.Join(project, "workflows", "legacy", "README.md")); err != nil {
		t.Fatalf("expected bundled init README copied: %v", err)
	}
}

// TestCmdInitErrorsOnUnknownOption rejects unsupported flags to dockpipe init.
func TestCmdInitErrorsOnUnknownOption(t *testing.T) {
	repoRoot := mkRepoRootForSubcmdTests(t)
	t.Setenv("DOCKPIPE_REPO_ROOT", repoRoot)

	err := cmdInit([]string{"--nope"})
	if err == nil || !strings.Contains(err.Error(), "unknown option") {
		t.Fatalf("expected unknown option error, got %v", err)
	}
}

// TestCmdInitRejectsLegacyTemplatesCollision refuses to create under workflows/ when templates/<name> already exists (legacy).
func TestCmdInitRejectsLegacyTemplatesCollision(t *testing.T) {
	repoRoot := mkRepoRootForSubcmdTests(t)
	t.Setenv("DOCKPIPE_REPO_ROOT", repoRoot)

	project := t.TempDir()
	if err := os.MkdirAll(filepath.Join(project, "templates", "demo"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project, "templates", "demo", "config.yml"), []byte("name: demo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(project); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldWd) })

	err = cmdInit([]string{"demo"})
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("expected existing template error, got %v", err)
	}
}

// TestCmdInitAppliesResolverRuntimeStrategy writes optional fields into config.yml.
func TestCmdInitAppliesResolverRuntimeStrategy(t *testing.T) {
	repoRoot := mkRepoRootForSubcmdTests(t)
	t.Setenv("DOCKPIPE_REPO_ROOT", repoRoot)

	project := t.TempDir()
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(project); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldWd) })

	if err := cmdInit([]string{"demo", "--resolver", "claude", "--runtime", "vscode", "--strategy", "worktree"}); err != nil {
		t.Fatalf("cmdInit failed: %v", err)
	}
	b, err := os.ReadFile(filepath.Join(project, "workflows", "demo", "config.yml"))
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	if !strings.Contains(s, "claude") || !strings.Contains(s, "vscode") || !strings.Contains(s, "worktree") {
		t.Fatalf("expected resolver/runtime/strategy in config, got:\n%s", s)
	}
}

// TestCmdInitRequiresNameForFrom errors when --from is set without a workflow name.
func TestCmdInitRequiresNameForFrom(t *testing.T) {
	repoRoot := mkRepoRootForSubcmdTests(t)
	t.Setenv("DOCKPIPE_REPO_ROOT", repoRoot)

	project := t.TempDir()
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(project); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldWd) })

	err = cmdInit([]string{"--from", "run"})
	if err == nil || !strings.Contains(err.Error(), "requires a workflow name") {
		t.Fatalf("expected --from requires name error, got %v", err)
	}
}

// TestCmdInitBareCreatesMinimalScaffold verifies dockpipe init (no name) creates root metadata plus a starter workflows/example/.
func TestCmdInitBareCreatesMinimalScaffold(t *testing.T) {
	repoRoot := testRepoRoot(t)
	t.Setenv("DOCKPIPE_REPO_ROOT", repoRoot)
	project := t.TempDir()
	oldWd, _ := os.Getwd()
	if err := os.Chdir(project); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldWd) })

	if err := cmdInit([]string{}); err != nil {
		t.Fatalf("cmdInit: %v", err)
	}
	markers := []string{
		filepath.Join(project, "README.md"),
		filepath.Join(project, domain.DockpipeProjectConfigFileName),
		filepath.Join(project, ".env.vault.template.example"),
		filepath.Join(project, "workflows"),
		filepath.Join(project, "workflows", "example", "config.yml"),
	}
	for _, p := range markers {
		if _, err := os.Stat(p); err != nil {
			t.Fatalf("expected scaffold path %q: %v", p, err)
		}
	}
	for _, p := range []string{
		filepath.Join(project, "scripts"),
		filepath.Join(project, "images"),
		filepath.Join(project, "templates", "core"),
	} {
		if _, err := os.Stat(p); err == nil {
			t.Fatalf("did not expect legacy scaffold path %q", p)
		}
	}
}

// TestCmdInitFromURLRejected ensures init --from does not accept git URLs (project-local sources only).
func TestCmdInitFromURLRejected(t *testing.T) {
	repoRoot := mkRepoRootForSubcmdTests(t)
	t.Setenv("DOCKPIPE_REPO_ROOT", repoRoot)
	project := t.TempDir()
	oldWd, _ := os.Getwd()
	if err := os.Chdir(project); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldWd) })

	err := cmdInit([]string{"demo", "--from", "https://example.com/repo.git"})
	if err == nil || !strings.Contains(err.Error(), "not a URL") {
		t.Fatalf("expected URL rejection, got %v", err)
	}
}

// TestDoctorHelp runs dockpipe doctor --help without requiring Docker.
func TestDoctorHelp(t *testing.T) {
	if err := Run([]string{"doctor", "--help"}, nil); err != nil {
		t.Fatalf("doctor --help: %v", err)
	}
}

// TestInitHelp runs dockpipe init --help without touching the project layout.
func TestInitHelp(t *testing.T) {
	if err := Run([]string{"init", "--help"}, nil); err != nil {
		t.Fatalf("init --help: %v", err)
	}
}

// TestRunHelpAndMissingWorkflow prints help without error and errors on missing workflow name.
func TestRunHelpAndMissingWorkflow(t *testing.T) {
	repoRoot := mkRepoRootForSubcmdTests(t)
	t.Setenv("DOCKPIPE_REPO_ROOT", repoRoot)

	if err := Run([]string{"--help"}, []string{}); err != nil {
		t.Fatalf("Run --help should return nil, got %v", err)
	}
	err := Run([]string{"--workflow", "nope", "--", "echo", "x"}, []string{})
	if err == nil || !strings.Contains(err.Error(), `workflow "nope" not found`) {
		t.Fatalf("expected missing workflow error, got %v", err)
	}
}
