package application

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCmdPackageListFindsPackageYml(t *testing.T) {
	dir := t.TempDir()
	pkgRoot := filepath.Join(dir, ".dockpipe", "internal", "packages", "workflows", "demo")
	if err := os.MkdirAll(pkgRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := `schema: 1
name: demo
version: 1.0.0
description: hello
`
	if err := os.WriteFile(filepath.Join(pkgRoot, "package.yml"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldWd) })

	if err := cmdPackage([]string{"list"}); err != nil {
		t.Fatal(err)
	}
	// stderr printed to os.Stderr; we only assert command succeeds.
}

func TestCmdPackageManifest(t *testing.T) {
	if err := cmdPackage([]string{"manifest"}); err != nil {
		t.Fatal(err)
	}
}

func TestCmdPackageUnknown(t *testing.T) {
	err := cmdPackage([]string{"nope"})
	if err == nil || !strings.Contains(err.Error(), "unknown") {
		t.Fatalf("got %v", err)
	}
}

func TestCmdPackageCompileCore(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src", "core")
	if err := os.MkdirAll(filepath.Join(src, "runtimes"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "runtimes", ".keep"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldWd) })

	if err := cmdPackage([]string{"compile", "core", "--from", src}); err != nil {
		t.Fatal(err)
	}
	corePkg := filepath.Join(dir, ".dockpipe", "internal", "packages", "core", "package.yml")
	if _, err := os.Stat(corePkg); err != nil {
		t.Fatal(err)
	}
}

func TestRunCompileAliasHelp(t *testing.T) {
	if err := Run([]string{"compile", "core", "--help"}, nil); err != nil {
		t.Fatal(err)
	}
}

func TestCmdPackageCompileWorkflow(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src", "mywf")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := `name: mywf
description: test
steps: []
`
	if err := os.WriteFile(filepath.Join(src, "config.yml"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := cmdPackage([]string{"compile", "workflow", "--workdir", dir, "--from", src}); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(dir, ".dockpipe", "internal", "packages", "workflows", "mywf", "config.yml")
	if _, err := os.Stat(dest); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".dockpipe", "internal", "packages", "workflows", "mywf", "package.yml")); err != nil {
		t.Fatal(err)
	}
}
