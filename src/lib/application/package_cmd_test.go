package application

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dockpipe/src/lib/infrastructure/packagebuild"
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
	coreDir := filepath.Join(dir, ".dockpipe", "internal", "packages", "core")
	matches, err := filepath.Glob(filepath.Join(coreDir, "dockpipe-core-*.tar.gz"))
	if err != nil || len(matches) != 1 {
		t.Fatalf("expected one core tarball under %s: matches=%v err=%v", coreDir, matches, err)
	}
	if _, err := packagebuild.ReadFileFromTarGz(matches[0], "core/package.yml"); err != nil {
		t.Fatal(err)
	}
}

func TestCmdPackageCompileResolversVendorResolversSubdir(t *testing.T) {
	dir := t.TempDir()
	pack := filepath.Join(dir, "my-vendor")
	resRoot := filepath.Join(pack, "resolvers", "alpha")
	if err := os.MkdirAll(resRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(resRoot, "profile"), []byte("DOCKPIPE_RESOLVER_CMD=test\n"), 0o644); err != nil {
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

	if err := cmdPackage([]string{"compile", "resolvers", "--from", pack}); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(dir, ".dockpipe", "internal", "packages", "resolvers")
	matches, err := filepath.Glob(filepath.Join(dest, "dockpipe-resolver-alpha-*.tar.gz"))
	if err != nil || len(matches) != 1 {
		t.Fatalf("expected one resolver tarball: matches=%v err=%v", matches, err)
	}
	if _, err := packagebuild.ReadFileFromTarGz(matches[0], "resolvers/alpha/profile"); err != nil {
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
	tgz := filepath.Join(dir, ".dockpipe", "internal", "packages", "workflows", "dockpipe-workflow-mywf-0.1.0.tar.gz")
	if _, err := os.Stat(tgz); err != nil {
		t.Fatal(err)
	}
	if _, err := packagebuild.ReadFileFromTarGz(tgz, "workflows/mywf/config.yml"); err != nil {
		t.Fatal(err)
	}
	if _, err := packagebuild.ReadFileFromTarGz(tgz, "workflows/mywf/package.yml"); err != nil {
		t.Fatal(err)
	}
}
