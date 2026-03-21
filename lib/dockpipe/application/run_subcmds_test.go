package application

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
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
	writeFile(t, filepath.Join(repoRoot, "templates", "init", "config.yml"), "name: init\n", 0o644)
	writeFile(t, filepath.Join(repoRoot, "templates", "init", "resolvers", "default"), "DOCKPIPE_RESOLVER_TEMPLATE=codex\n", 0o644)
	writeFile(t, filepath.Join(repoRoot, "templates", "llm-worktree", "config.yml"), "name: llm-worktree\n", 0o644)
	writeFile(t, filepath.Join(repoRoot, "templates", "llm-worktree", "resolvers", "claude"), "DOCKPIPE_RESOLVER_TEMPLATE=claude\n", 0o644)
	writeFile(t, filepath.Join(repoRoot, "scripts", "commit-worktree.sh"), "#!/usr/bin/env bash\n", 0o755)
	writeFile(t, filepath.Join(repoRoot, "scripts", "clone-worktree.sh"), "#!/usr/bin/env bash\n", 0o755)
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
	sh := filepath.Join(dest, "resolvers", "claude")
	if _, err := os.Stat(sh); err != nil {
		t.Fatalf("expected copied resolver file: %v", err)
	}
}

// TestCmdInitLikeScriptCreateAndFromBundled covers dockpipe action init default and --from bundled script.
func TestCmdInitLikeScriptCreateAndFromBundled(t *testing.T) {
	repoRoot := mkRepoRootForSubcmdTests(t)
	t.Setenv("DOCKPIPE_REPO_ROOT", repoRoot)
	writeFile(t, filepath.Join(repoRoot, "scripts", "print-summary.sh"), "#!/usr/bin/env bash\necho ok\n", 0o755)

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

// TestCmdInitCreatesWorkspaceAndTemplate creates workspace layout and templates/<name> from init template.
func TestCmdInitCreatesWorkspaceAndTemplate(t *testing.T) {
	repoRoot := mkRepoRootForSubcmdTests(t)
	t.Setenv("DOCKPIPE_REPO_ROOT", repoRoot)

	dest := filepath.Join(t.TempDir(), "workspace")
	if err := cmdInit([]string{"demo", dest}); err != nil {
		t.Fatalf("cmdInit failed: %v", err)
	}
	checks := []string{
		filepath.Join(dest, "README.md"),
		filepath.Join(dest, "scripts"),
		filepath.Join(dest, "images"),
		filepath.Join(dest, "templates", "demo", "config.yml"),
		filepath.Join(dest, "templates", "demo", "resolvers", "default"),
	}
	for _, p := range checks {
		if _, err := os.Stat(p); err != nil {
			t.Fatalf("expected created path %q: %v", p, err)
		}
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

// TestCmdInitErrorsOnNonEmptyDestination refuses to init into a non-empty directory.
func TestCmdInitErrorsOnNonEmptyDestination(t *testing.T) {
	repoRoot := mkRepoRootForSubcmdTests(t)
	t.Setenv("DOCKPIPE_REPO_ROOT", repoRoot)

	dest := filepath.Join(t.TempDir(), "workspace")
	if err := os.MkdirAll(dest, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dest, "existing.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := cmdInit([]string{"demo", dest})
	if err == nil || !strings.Contains(err.Error(), "destination exists and is not empty") {
		t.Fatalf("expected non-empty destination error, got %v", err)
	}
}

// TestDoctorHelp runs dockpipe doctor --help without requiring Docker.
func TestDoctorHelp(t *testing.T) {
	if err := Run([]string{"doctor", "--help"}, nil); err != nil {
		t.Fatalf("doctor --help: %v", err)
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
