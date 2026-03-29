package infrastructure

import (
	"os"
	"path/filepath"
	"testing"

	"dockpipe/src/lib/infrastructure/packagebuild"
)

func TestTryResolveWorkflowTarballURI(t *testing.T) {
	root := t.TempDir()
	art := filepath.Join(root, "release", "artifacts")
	if err := os.MkdirAll(art, 0o755); err != nil {
		t.Fatal(err)
	}
	wf := filepath.Join(t.TempDir(), "demo")
	if err := os.MkdirAll(wf, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wf, "config.yml"), []byte("name: demo\nnamespace: acme\nrun: echo hi\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	tgz := filepath.Join(art, "dockpipe-workflow-demo-1.0.0.tar.gz")
	if _, err := packagebuild.WriteDirTarGzWithPrefix(wf, tgz, "workflows/demo"); err != nil {
		t.Fatal(err)
	}
	got, err := tryResolveWorkflowTarballURI(root, "", "demo")
	if err != nil {
		t.Fatal(err)
	}
	if got == "" {
		t.Fatal("expected tar URI")
	}
	w, err := LoadWorkflow(got)
	if err != nil {
		t.Fatal(err)
	}
	if w.Namespace != "acme" {
		t.Fatalf("namespace %q", w.Namespace)
	}
}
