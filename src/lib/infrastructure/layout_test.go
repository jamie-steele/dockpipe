package infrastructure

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWorkflowsRootDirAuthoringUserProject(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmp, "templates", "core"), 0o755); err != nil {
		t.Fatal(err)
	}
	if got, want := WorkflowsRootDir(tmp), filepath.Join(tmp, "workflows"); got != want {
		t.Fatalf("WorkflowsRootDir = %q, want %q", got, want)
	}
	if got, want := CoreDir(tmp), filepath.Join(tmp, "templates", "core"); got != want {
		t.Fatalf("CoreDir = %q, want %q", got, want)
	}
	if UsesBundledAssetLayout(tmp) {
		t.Fatal("UsesBundledAssetLayout should be false without bundle/core")
	}
}

func TestWorkflowsRootDirAuthoringSrcCoreLayout(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmp, "src", "core", "runtimes"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(tmp, "src", "core", "workflows"), 0o755); err != nil {
		t.Fatal(err)
	}
	if got, want := WorkflowsRootDir(tmp), filepath.Join(tmp, "src", "core", "workflows"); got != want {
		t.Fatalf("WorkflowsRootDir = %q, want %q", got, want)
	}
	if got, want := CoreDir(tmp), filepath.Join(tmp, "src", "core"); got != want {
		t.Fatalf("CoreDir = %q, want %q", got, want)
	}
}

func TestWorkflowsRootDirAuthoringDogfoodPrefersRepoWorkflowsWhenPresent(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmp, "src", "core", "runtimes"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(tmp, "src", "core", "workflows"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(tmp, "workflows", "demo"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "workflows", "demo", "config.yml"), []byte("name: demo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got, want := WorkflowsRootDir(tmp), filepath.Join(tmp, "workflows"); got != want {
		t.Fatalf("WorkflowsRootDir = %q, want %q", got, want)
	}
}

func TestWorkflowsRootDirPrefersSrcCoreWhenBothTreesExist(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmp, "templates", "core"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(tmp, "src", "core", "runtimes"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(tmp, "src", "core", "workflows"), 0o755); err != nil {
		t.Fatal(err)
	}
	if got, want := WorkflowsRootDir(tmp), filepath.Join(tmp, "src", "core", "workflows"); got != want {
		t.Fatalf("WorkflowsRootDir = %q, want %q", got, want)
	}
}

func TestCoreDirMaterializedBundle(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmp, BundledLayoutDir, "core"), 0o755); err != nil {
		t.Fatal(err)
	}
	if !UsesBundledAssetLayout(tmp) {
		t.Fatal("UsesBundledAssetLayout should be true when bundle/core exists")
	}
	if got, want := CoreDir(tmp), filepath.Join(tmp, BundledLayoutDir, "core"); got != want {
		t.Fatalf("CoreDir = %q, want %q", got, want)
	}
	if got, want := WorkflowsRootDir(tmp), filepath.Join(tmp, BundledLayoutDir, "workflows"); got != want {
		t.Fatalf("WorkflowsRootDir = %q, want %q", got, want)
	}
}

func TestEmbeddedTemplatesPrefixMatchesEmbedGo(t *testing.T) {
	// Keep in sync with repo-root embed.go //go:embed src/core
	if EmbeddedTemplatesPrefix != "src/core" {
		t.Fatalf("EmbeddedTemplatesPrefix = %q", EmbeddedTemplatesPrefix)
	}
}
