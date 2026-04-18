package application

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stderr = w
	t.Cleanup(func() { os.Stderr = old })
	fn()
	_ = w.Close()
	b, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	_ = r.Close()
	os.Stderr = old
	return string(b)
}

// TestCmdInitFromBlankScaffold produces a minimal workflow YAML when --from blank.
func TestCmdInitFromBlankScaffold(t *testing.T) {
	repoRoot := mkRepoRootForSubcmdTests(t)
	t.Setenv("DOCKPIPE_REPO_ROOT", repoRoot)
	project := t.TempDir()
	oldWd, _ := os.Getwd()
	if err := os.Chdir(project); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldWd) })

	if err := cmdInit([]string{"blankflow", "--from", "blank"}); err != nil {
		t.Fatalf("cmdInit: %v", err)
	}
	b, err := os.ReadFile(filepath.Join(project, "workflows", "blankflow", "config.yml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "blankflow") {
		t.Fatalf("expected workflow name in config, got:\n%s", string(b))
	}
}

// TestCmdInitBareCreatesStarterWorkflow ensures bare init seeds workflows/example/ for first-run discoverability.
func TestCmdInitBareCreatesStarterWorkflow(t *testing.T) {
	repoRoot := mkRepoRootForSubcmdTests(t)
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
	b, err := os.ReadFile(filepath.Join(project, "workflows", "example", "config.yml"))
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	if !strings.Contains(s, "example") || !strings.Contains(s, "steps:") || !strings.Contains(s, "types:") || !strings.Contains(s, "EXAMPLE_IMAGE") {
		t.Fatalf("expected starter workflow config, got:\n%s", string(b))
	}
	if _, err := os.Stat(filepath.Join(project, "workflows", "example", "models", "IExampleWorkflowConfig.pipe")); err != nil {
		t.Fatalf("expected example model interface copied: %v", err)
	}
	if _, err := os.Stat(filepath.Join(project, "workflows", "example", "models", "ExampleWorkflowConfig.pipe")); err != nil {
		t.Fatalf("expected example model class copied: %v", err)
	}
}

// TestCmdInitFromRunTemplate copies an existing bundled workflow template by name.
func TestCmdInitFromRunTemplate(t *testing.T) {
	repoRoot := mkRepoRootForSubcmdTests(t)
	t.Setenv("DOCKPIPE_REPO_ROOT", repoRoot)
	project := t.TempDir()
	oldWd, _ := os.Getwd()
	if err := os.Chdir(project); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldWd) })

	if err := cmdInit([]string{"myrun", "--from", "run"}); err != nil {
		t.Fatalf("cmdInit: %v", err)
	}
	b, err := os.ReadFile(filepath.Join(project, "workflows", "myrun", "config.yml"))
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	if !strings.Contains(s, "myrun") {
		t.Fatalf("expected patched name myrun in config, got:\n%s", s)
	}
}

// TestCmdInitDoesNotCreateGitDir ensures init never bootstraps a git repository in the project tree.
func TestCmdInitDoesNotCreateGitDir(t *testing.T) {
	repoRoot := mkRepoRootForSubcmdTests(t)
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
	if _, err := os.Stat(filepath.Join(project, ".git")); err == nil {
		t.Fatal("init must not create .git (no clone/bootstrap)")
	}
	if _, err := os.Stat(filepath.Join(project, ".env.vault.template.example")); err != nil {
		t.Fatalf("expected .env.vault.template.example from init scaffold: %v", err)
	}
}

// TestCmdInitDoesNotCopyLegacyTemplateTree verifies dockpipe init no longer copies templates/core.
func TestCmdInitDoesNotCopyLegacyTemplateTree(t *testing.T) {
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
	for _, p := range []string{
		filepath.Join(project, "templates"),
		filepath.Join(project, "templates", "core"),
		filepath.Join(project, "scripts"),
		filepath.Join(project, "images"),
	} {
		if _, err := os.Stat(p); err == nil {
			t.Fatalf("did not expect legacy scaffold path %q", p)
		}
	}
}

func TestCmdInitBareDoesNotWarnForEmptyWorkflowsSubdirs(t *testing.T) {
	repoRoot := mkRepoRootForSubcmdTests(t)
	t.Setenv("DOCKPIPE_REPO_ROOT", repoRoot)
	project := t.TempDir()
	if err := os.MkdirAll(filepath.Join(project, "workflows", "github", "nested"), 0o755); err != nil {
		t.Fatal(err)
	}
	oldWd, _ := os.Getwd()
	if err := os.Chdir(project); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldWd) })

	stderr := captureStderr(t, func() {
		if err := cmdInit([]string{}); err != nil {
			t.Fatalf("cmdInit: %v", err)
		}
	})
	if strings.Contains(stderr, "has no DockPipe workflow folders") {
		t.Fatalf("did not expect workflows warning for empty directory tree, got:\n%s", stderr)
	}
}

func TestCmdInitBareWarnsForNonDockpipeWorkflowFiles(t *testing.T) {
	repoRoot := mkRepoRootForSubcmdTests(t)
	t.Setenv("DOCKPIPE_REPO_ROOT", repoRoot)
	project := t.TempDir()
	writeFile(t, filepath.Join(project, "workflows", "ci", "deploy.yml"), "name: deploy\n", 0o644)
	oldWd, _ := os.Getwd()
	if err := os.Chdir(project); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldWd) })

	stderr := captureStderr(t, func() {
		if err := cmdInit([]string{}); err != nil {
			t.Fatalf("cmdInit: %v", err)
		}
	})
	if !strings.Contains(stderr, "has no DockPipe workflow folders") {
		t.Fatalf("expected workflows warning for non-DockPipe workflow files, got:\n%s", stderr)
	}
}
