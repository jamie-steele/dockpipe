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

func TestPipeLangMaterializeUsesTypesMappingFromWorkflowYAML(t *testing.T) {
	project := t.TempDir()
	wfDir := filepath.Join(project, "wf")
	models := filepath.Join(wfDir, "models")
	if err := os.MkdirAll(models, 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := `name: mapped
types:
  - models/IConfig
`
	if err := os.WriteFile(filepath.Join(wfDir, "config.yml"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	iface := `public Interface IConfig { public string IMAGE; }`
	class := `public Struct AppConfig : IConfig { public string IMAGE = "nginx"; }`
	if err := os.WriteFile(filepath.Join(models, "IConfig.pipe"), []byte(iface), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(models, "AppConfig.pipe"), []byte(class), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := cmdPipeLang([]string{"materialize", "--from", wfDir, "--force"}); err != nil {
		t.Fatalf("materialize: %v", err)
	}
	for _, rel := range []string{
		"wf/models/.pipelang/IConfig.AppConfig.workflow.yml",
		"wf/models/.pipelang/IConfig.AppConfig.bindings.json",
		"wf/models/.pipelang/IConfig.AppConfig.bindings.env",
	} {
		if _, err := os.Stat(filepath.Join(project, rel)); err != nil {
			t.Fatalf("missing mapped artifact %s: %v", rel, err)
		}
	}
}
