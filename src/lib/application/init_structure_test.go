package application

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

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

// TestCmdInitFromSelfAnalysisWorkflowAppendsAgentsPointer ensures init --from dorkpipe-self-analysis updates AGENTS.md once.
func TestCmdInitFromSelfAnalysisWorkflowAppendsAgentsPointer(t *testing.T) {
	repoRoot := mkRepoRootForSubcmdTests(t)
	writeFile(t, filepath.Join(repoRoot, "templates", "dorkpipe-self-analysis", "config.yml"), "name: dorkpipe-self-analysis\nsteps: []\n", 0o644)
	t.Setenv("DOCKPIPE_REPO_ROOT", repoRoot)
	project := t.TempDir()
	oldWd, _ := os.Getwd()
	if err := os.Chdir(project); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldWd) })

	if err := cmdInit([]string{"myself", "--from", "dorkpipe-self-analysis"}); err != nil {
		t.Fatalf("cmdInit: %v", err)
	}
	agentsPath := filepath.Join(project, "AGENTS.md")
	b, err := os.ReadFile(agentsPath)
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	if !strings.Contains(s, agentsSelfAnalysisMarker) {
		t.Fatalf("expected self-analysis marker in AGENTS.md, got:\n%s", s)
	}
	changed, err := ensureAgentsSelfAnalysisPointer(project)
	if err != nil {
		t.Fatal(err)
	}
	if changed {
		t.Fatal("second ensureAgentsSelfAnalysisPointer should be no-op")
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

// TestCmdInitMergedCoreTopLevelMatchesModel verifies dockpipe init merges only the four category dirs at templates/core/.
func TestCmdInitMergedCoreTopLevelMatchesModel(t *testing.T) {
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
	core := filepath.Join(project, "templates", "core")
	ents, err := os.ReadDir(core)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"assets", "resolvers", "runtimes", "strategies"}
	var names []string
	for _, e := range ents {
		if e.IsDir() && e.Name()[0] != '.' {
			names = append(names, e.Name())
		}
	}
	slices.Sort(names)
	if !slices.Equal(names, want) {
		t.Fatalf("templates/core top-level want %v got %v", want, names)
	}
	assets := filepath.Join(core, "assets")
	aents, err := os.ReadDir(assets)
	if err != nil {
		t.Fatal(err)
	}
	assetWant := []string{"compose", "images", "scripts"}
	var anames []string
	for _, e := range aents {
		if e.IsDir() && e.Name()[0] != '.' {
			anames = append(anames, e.Name())
		}
	}
	slices.Sort(anames)
	if !slices.Equal(anames, assetWant) {
		t.Fatalf("templates/core/assets want %v got %v", assetWant, anames)
	}
}
