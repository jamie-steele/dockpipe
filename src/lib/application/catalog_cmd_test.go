package application

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBuildCatalogListOutput_UsesDockpipeContracts(t *testing.T) {
	project := t.TempDir()
	cfg := `{
  "schema": 1,
  "compile": {
    "workflows": ["workflows", "packages"]
  }
}`
	if err := os.WriteFile(filepath.Join(project, "dockpipe.config.json"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := os.MkdirAll(filepath.Join(project, "workflows", "demo"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project, "workflows", "demo", "config.yml"), []byte("name: Demo App\ncategory: app\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := os.MkdirAll(filepath.Join(project, "packages", "ide", "resolvers", "cursor-dev"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project, "packages", "ide", "resolvers", "cursor-dev", "profile"), []byte("DOCKPIPE_RESOLVER_CMD=cursor-dev\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := os.MkdirAll(filepath.Join(project, "templates", "core", "strategies"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project, "templates", "core", "strategies", "commit"), []byte("DOCKPIPE_STRATEGY=commit\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := os.MkdirAll(filepath.Join(project, "templates", "core", "runtimes", "dockerimage"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project, "templates", "core", "runtimes", "dockerimage", "profile"), []byte("DOCKPIPE_RUNTIME_SUBSTRATE=dockerimage\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := buildCatalogListOutput(project, project)
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Workflows) != 1 || out.Workflows[0].WorkflowID != "demo" || out.Workflows[0].DisplayName != "Demo App" {
		t.Fatalf("unexpected workflows: %#v", out.Workflows)
	}
	if !containsString(out.Resolvers, "cursor-dev") {
		t.Fatalf("expected cursor-dev resolver in %#v", out.Resolvers)
	}
	if !containsString(out.Strategies, "commit") {
		t.Fatalf("expected commit strategy in %#v", out.Strategies)
	}
	if !containsString(out.Runtimes, "dockerimage") {
		t.Fatalf("expected dockerimage runtime in %#v", out.Runtimes)
	}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
