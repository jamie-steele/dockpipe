package application

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dockpipe/src/lib/domain"
	"dockpipe/src/lib/infrastructure"
	"dockpipe/src/lib/infrastructure/packagebuild"
)

func TestCmdCloneCopiesWhenAllowCloneTrue(t *testing.T) {
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
	pw := filepath.Join(dir, infrastructure.DockpipeDirRel, "internal", "packages", "workflows")
	matches, err := filepath.Glob(filepath.Join(pw, "dockpipe-workflow-mywf-*.tar.gz"))
	if err != nil || len(matches) != 1 {
		t.Fatalf("expected one compiled tarball under %s: matches=%v err=%v", pw, matches, err)
	}
	pyml, err := packagebuild.ReadFileFromTarGz(matches[0], "workflows/mywf/package.yml")
	if err != nil {
		t.Fatal(err)
	}
	pmPath := filepath.Join(t.TempDir(), "package.yml")
	if err := os.WriteFile(pmPath, pyml, 0o644); err != nil {
		t.Fatal(err)
	}
	m, err := domain.ParsePackageManifest(pmPath)
	if err != nil {
		t.Fatal(err)
	}
	if !m.AllowClone {
		t.Fatal("expected compile to set allow_clone true")
	}
	if err := cmdClone([]string{"mywf", "--workdir", dir, "--to", filepath.Join(dir, "out", "mywf")}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "out", "mywf", "config.yml")); err != nil {
		t.Fatal(err)
	}
}

func TestCmdCloneDeniedWhenAllowCloneFalse(t *testing.T) {
	dir := t.TempDir()
	st := t.TempDir()
	if err := os.WriteFile(filepath.Join(st, "config.yml"), []byte("name: paid\nsteps: []\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	manifest := `schema: 1
name: paid
version: 1.0.0
kind: workflow
requires_capabilities: [workflow.paid]
allow_clone: false
distribution: binary
`
	if err := os.WriteFile(filepath.Join(st, "package.yml"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	pw := filepath.Join(dir, infrastructure.DockpipeDirRel, "internal", "packages", "workflows")
	if err := os.MkdirAll(pw, 0o755); err != nil {
		t.Fatal(err)
	}
	tgz := filepath.Join(pw, "dockpipe-workflow-paid-1.0.0.tar.gz")
	if _, err := packagebuild.WriteDirTarGzWithPrefix(st, tgz, "workflows/paid"); err != nil {
		t.Fatal(err)
	}
	err := cmdClone([]string{"paid", "--workdir", dir, "--to", filepath.Join(dir, "workflows", "paid")})
	if err == nil || !strings.Contains(err.Error(), "does not allow cloning") {
		t.Fatalf("expected clone denied, got %v", err)
	}
}
