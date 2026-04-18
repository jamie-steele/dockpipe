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
	if err := os.MkdirAll(filepath.Join(project, "workflows", "demo", "assets", "images"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project, "workflows", "demo", "assets", "images", "icon.png"), []byte("png"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project, "workflows", "demo", "config.yml"), []byte("name: Demo App\ncategory: app\nicon: assets/images/icon.png\n"), 0o644); err != nil {
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
	if got, want := out.Workflows[0].IconPath, filepath.Join(project, "workflows", "demo", "assets", "images", "icon.png"); got != want {
		t.Fatalf("expected icon path %q, got %q", want, got)
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

func TestBuildCatalogListOutput_FallsBackToPackageArtworkAndIcon(t *testing.T) {
	project := t.TempDir()
	cfg := `{
  "schema": 1,
  "compile": {
    "workflows": ["workflows", "packages", ".staging/packages"]
  }
}`
	if err := os.WriteFile(filepath.Join(project, "dockpipe.config.json"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := os.MkdirAll(filepath.Join(project, "packages", "ide", "resolvers", "cursor-dev", "assets", "images"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project, "packages", "ide", "resolvers", "cursor-dev", "assets", "images", "icon.png"), []byte("png"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project, "packages", "ide", "package.yml"), []byte("schema: 1\nkind: package\nname: ide\nicon: resolvers/cursor-dev/assets/images/icon.png\nartwork:\n  cursor-dev: resolvers/cursor-dev/assets/images/icon.png\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project, "packages", "ide", "resolvers", "cursor-dev", "config.yml"), []byte("name: Cursor\ncategory: app\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := os.MkdirAll(filepath.Join(project, "packages", "pipeon", "resolvers", "pipeon-dev-stack", "assets", "images"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project, "packages", "pipeon", "resolvers", "pipeon-dev-stack", "assets", "images", "icon.png"), []byte("png"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project, "packages", "pipeon", "package.yml"), []byte("schema: 1\nkind: package\nname: pipeon\nicon: resolvers/pipeon-dev-stack/assets/images/icon.png\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project, "packages", "pipeon", "resolvers", "pipeon-dev-stack", "config.yml"), []byte("name: Pipeon\ncategory: app\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := buildCatalogListOutput(project, project)
	if err != nil {
		t.Fatal(err)
	}

	icons := map[string]string{}
	for _, wf := range out.Workflows {
		icons[wf.WorkflowID] = wf.IconPath
	}

	if got, want := icons["cursor-dev"], filepath.Join(project, "packages", "ide", "resolvers", "cursor-dev", "assets", "images", "icon.png"); got != want {
		t.Fatalf("expected cursor-dev package artwork icon %q, got %q", want, got)
	}
	if got, want := icons["pipeon-dev-stack"], filepath.Join(project, "packages", "pipeon", "resolvers", "pipeon-dev-stack", "assets", "images", "icon.png"); got != want {
		t.Fatalf("expected pipeon-dev-stack package icon %q, got %q", want, got)
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
