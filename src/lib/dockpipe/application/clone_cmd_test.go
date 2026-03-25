package application

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dockpipe/src/lib/dockpipe/domain"
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
	mPath := filepath.Join(dir, ".dockpipe", "internal", "packages", "workflows", "mywf", "package.yml")
	m, err := domain.ParsePackageManifest(mPath)
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
	pkgRoot := filepath.Join(dir, ".dockpipe", "internal", "packages", "workflows", "paid")
	if err := os.MkdirAll(pkgRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkgRoot, "config.yml"), []byte("name: paid\nsteps: []\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	manifest := `schema: 1
name: paid
version: 1.0.0
kind: workflow
allow_clone: false
distribution: binary
`
	if err := os.WriteFile(filepath.Join(pkgRoot, "package.yml"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	err := cmdClone([]string{"paid", "--workdir", dir, "--to", filepath.Join(dir, "workflows", "paid")})
	if err == nil || !strings.Contains(err.Error(), "does not allow cloning") {
		t.Fatalf("expected clone denied, got %v", err)
	}
}
