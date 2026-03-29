package infrastructure

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolvePackagedWorkflowConfigPath(t *testing.T) {
	repo := t.TempDir()
	projectCfg := `{"schema":1,"compile":{"workflows":[".staging/packages"]}}`
	if err := os.WriteFile(filepath.Join(repo, "dockpipe.config.json"), []byte(projectCfg), 0o644); err != nil {
		t.Fatal(err)
	}
	wfDir := filepath.Join(repo, ".staging", "packages", "mywf")
	if err := os.MkdirAll(wfDir, 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := `name: mywf
namespace: dockpipe.demo
steps: []
`
	if err := os.WriteFile(filepath.Join(wfDir, "config.yml"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := ResolvePackagedWorkflowConfigPath(repo, repo, "mywf", "dockpipe.demo")
	if err != nil {
		t.Fatal(err)
	}
	if got != filepath.Join(wfDir, "config.yml") {
		t.Fatalf("got %s want %s", got, filepath.Join(wfDir, "config.yml"))
	}
	_, err = ResolvePackagedWorkflowConfigPath(repo, repo, "mywf", "wrong.ns")
	if err == nil {
		t.Fatal("expected error for wrong namespace")
	}
}

func TestNormalizeRuntimeProfileName(t *testing.T) {
	if got := NormalizeRuntimeProfileName("kubepod"); got != "dockerimage" {
		t.Fatalf("got %q", got)
	}
	if got := NormalizeRuntimeProfileName("cli"); got != "dockerimage" {
		t.Fatalf("got %q", got)
	}
	if got := NormalizeRuntimeProfileName("docker"); got != "dockerimage" {
		t.Fatalf("got %q", got)
	}
	if got := NormalizeRuntimeProfileName("dockerimage"); got != "dockerimage" {
		t.Fatalf("got %q", got)
	}
}
