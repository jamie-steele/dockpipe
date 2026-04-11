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

func TestWorkflowCompileStartDirPrefersOnDiskOverStoreTarball(t *testing.T) {
	bundle := t.TempDir()
	if err := os.MkdirAll(filepath.Join(bundle, BundledLayoutDir, "workflows", "demo"), 0o755); err != nil {
		t.Fatal(err)
	}
	project := t.TempDir()
	onDiskDir := filepath.Join(project, "workflows", "demo")
	if err := os.MkdirAll(onDiskDir, 0o755); err != nil {
		t.Fatal(err)
	}
	onDisk := filepath.Join(onDiskDir, "config.yml")
	if err := os.WriteFile(onDisk, []byte("name: demo\nsteps: []\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	st := t.TempDir()
	if err := os.WriteFile(filepath.Join(st, "config.yml"), []byte("name: demo\nsteps: []\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(st, "package.yml"), []byte("schema: 1\nname: demo\nversion: 0.1.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	pw := filepath.Join(project, DockpipeDirRel, "internal", "packages", "workflows")
	if err := os.MkdirAll(pw, 0o755); err != nil {
		t.Fatal(err)
	}
	tgz := filepath.Join(pw, "dockpipe-workflow-demo-0.1.0.tar.gz")
	if _, err := packagebuild.WriteDirTarGzWithPrefix(st, tgz, "workflows/demo"); err != nil {
		t.Fatal(err)
	}
	start, err := WorkflowCompileStartDir(bundle, project, "demo")
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Clean(start) != filepath.Clean(onDiskDir) {
		t.Fatalf("compile start dir: got %q want %q", start, onDiskDir)
	}
}

func TestWorkflowCompileStartDirExtractsTarball(t *testing.T) {
	// Single project root: tarball search uses projectRoot for release/artifacts and packages store.
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
	start, err := WorkflowCompileStartDir(root, root, "demo")
	if err != nil {
		t.Fatal(err)
	}
	cfg := filepath.Join(start, "config.yml")
	if _, err := os.Stat(cfg); err != nil {
		t.Fatalf("expected config at %s: %v", cfg, err)
	}
}
