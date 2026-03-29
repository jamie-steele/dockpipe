package application

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCmdPipeLangMaterializeUsesCompileRootsFromConfig(t *testing.T) {
	project := t.TempDir()
	cfg := `{
  "schema": 1,
  "compile": {
    "workflows": ["xroots/a", "xroots/b"]
  }
}`
	if err := os.WriteFile(filepath.Join(project, "dockpipe.config.json"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{"xroots/a/one", "xroots/b/two"} {
		d := filepath.Join(project, rel)
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(d, "config.pipe"), []byte(samplePipeLang), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := cmdPipeLang([]string{"materialize", "--workdir", project}); err != nil {
		t.Fatalf("materialize: %v", err)
	}
	for _, rel := range []string{
		"xroots/a/one/.pipelang/config.DefaultDeployConfig.workflow.yml",
		"xroots/a/one/.pipelang/config.DefaultDeployConfig.bindings.json",
		"xroots/a/one/.pipelang/config.DefaultDeployConfig.bindings.env",
		"xroots/b/two/.pipelang/config.DefaultDeployConfig.workflow.yml",
	} {
		if _, err := os.Stat(filepath.Join(project, rel)); err != nil {
			t.Fatalf("missing materialized artifact %s: %v", rel, err)
		}
	}
}

func TestCompileWorkflowsBatchSupportsConfigPipe(t *testing.T) {
	project := t.TempDir()
	wfRoot := filepath.Join(project, "wfroot")
	wfDir := filepath.Join(wfRoot, "pipe-only")
	if err := os.MkdirAll(wfDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wfDir, "config.pipe"), []byte(samplePipeLang), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := cmdPackageCompileWorkflowsBatch([]string{"--workdir", project, "--from", wfRoot, "--force"}); err != nil {
		t.Fatalf("compile workflows batch: %v", err)
	}
	glob := filepath.Join(project, "bin", ".dockpipe", "internal", "packages", "workflows", "dockpipe-workflow-*.tar.gz")
	matches, err := filepath.Glob(glob)
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) == 0 {
		t.Fatalf("expected at least one compiled workflow tarball at %s", glob)
	}
}
