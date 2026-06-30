package application

import (
	"os"
	"path/filepath"
	"testing"

	"dockpipe/src/lib/infrastructure/packagebuild"
)

func TestCompileWorkflowHooksRunInStagingCopy(t *testing.T) {
	workdir := t.TempDir()
	src := filepath.Join(t.TempDir(), "workflow")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "config.yml"), []byte(`name: staged-workflow
compile_hooks:
  - printf staged > built.txt
steps: []
`), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := compileWorkflowOne(workdir, src, "", true); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(src, "built.txt")); !os.IsNotExist(err) {
		t.Fatalf("expected source tree to stay clean, got err=%v", err)
	}

	tarPath := filepath.Join(workdir, "bin", ".dockpipe", "internal", "packages", "workflows", "dockpipe-workflow-staged-workflow-0.0.0.tar.gz")
	got, err := packagebuild.ReadFileFromTarGz(tarPath, "workflows/staged-workflow/built.txt")
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "staged" {
		t.Fatalf("built.txt = %q", string(got))
	}
}

func TestCompileResolverHooksRunInStagingCopy(t *testing.T) {
	workdir := t.TempDir()
	destRoot := filepath.Join(workdir, "bin", ".dockpipe", "internal", "packages", "resolvers")
	if err := os.MkdirAll(destRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(t.TempDir(), "resolver")
	if err := os.MkdirAll(filepath.Join(src, "profile"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "profile", "env"), []byte("X=1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "config.yml"), []byte(`name: staged-resolver
compile_hooks:
  - printf staged-resolver > built.txt
steps: []
`), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := compileSingleResolverDir(workdir, destRoot, src, "staged-resolver", "dockpipeproject", "0.0.0", true); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(src, "built.txt")); !os.IsNotExist(err) {
		t.Fatalf("expected source tree to stay clean, got err=%v", err)
	}

	tarPath := filepath.Join(destRoot, "dockpipe-resolver-staged-resolver-0.0.0.tar.gz")
	got, err := packagebuild.ReadFileFromTarGz(tarPath, "resolvers/staged-resolver/built.txt")
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "staged-resolver" {
		t.Fatalf("built.txt = %q", string(got))
	}
}
