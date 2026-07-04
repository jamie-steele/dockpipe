package application

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dockpipe/src/lib/domain"
	"dockpipe/src/lib/infrastructure"
	"dockpipe/src/lib/infrastructure/packagebuild"
)

func captureCloneStderr(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stderr = w
	t.Cleanup(func() { os.Stderr = old })
	fn()
	_ = w.Close()
	b, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	_ = r.Close()
	os.Stderr = old
	return string(b)
}

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
	stderr := captureCloneStderr(t, func() {
		if err := cmdClone([]string{"mywf", "--workdir", dir, "--to", filepath.Join(dir, "out", "mywf")}); err != nil {
			t.Fatal(err)
		}
	})
	if _, err := os.Stat(filepath.Join(dir, "out", "mywf", "config.yml")); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"unit=clone.workflow", "status=start", "status=done", "workflow=mywf"} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("expected clone stderr to contain %q, got:\n%s", want, stderr)
		}
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
