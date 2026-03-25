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
		t.Fatal("UsesBundledAssetLayout should be false without shipyard/core")
	}
}

func TestWorkflowsRootDirAuthoringSrcTemplatesLayout(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmp, "src", "templates", "core"), 0o755); err != nil {
		t.Fatal(err)
	}
	if got, want := WorkflowsRootDir(tmp), filepath.Join(tmp, "src", "templates"); got != want {
		t.Fatalf("WorkflowsRootDir = %q, want %q", got, want)
	}
	if got, want := CoreDir(tmp), filepath.Join(tmp, "src", "templates", "core"); got != want {
		t.Fatalf("CoreDir = %q, want %q", got, want)
	}
}

func TestWorkflowsRootDirPrefersSrcTemplatesWhenBothTreesExist(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmp, "templates", "core"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(tmp, "src", "templates", "core"), 0o755); err != nil {
		t.Fatal(err)
	}
	if got, want := WorkflowsRootDir(tmp), filepath.Join(tmp, "src", "templates"); got != want {
		t.Fatalf("WorkflowsRootDir = %q, want %q", got, want)
	}
}

func TestCoreDirMaterializedBundle(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmp, ShipyardDir, "core"), 0o755); err != nil {
		t.Fatal(err)
	}
	if !UsesBundledAssetLayout(tmp) {
		t.Fatal("UsesBundledAssetLayout should be true when shipyard/core exists")
	}
	if got, want := CoreDir(tmp), filepath.Join(tmp, ShipyardDir, "core"); got != want {
		t.Fatalf("CoreDir = %q, want %q", got, want)
	}
	if got, want := WorkflowsRootDir(tmp), filepath.Join(tmp, ShipyardDir, "workflows"); got != want {
		t.Fatalf("WorkflowsRootDir = %q, want %q", got, want)
	}
}

func TestEmbeddedTemplatesPrefixMatchesEmbedGo(t *testing.T) {
	// Keep in sync with repo-root embed.go //go:embed src/templates
	if EmbeddedTemplatesPrefix != "src/templates" {
		t.Fatalf("EmbeddedTemplatesPrefix = %q", EmbeddedTemplatesPrefix)
	}
}
