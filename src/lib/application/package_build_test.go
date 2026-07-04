package application

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dockpipe/src/lib/infrastructure"
)

func TestCmdPackageBuildCoreEmitsOperationResults(t *testing.T) {
	dir := t.TempDir()
	coreRoot := filepath.Join(dir, "src", "core", "runtimes")
	if err := os.MkdirAll(coreRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(coreRoot, ".keep"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "VERSION"), []byte("1.2.3\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	stderr, err := captureResultStderr(t, func() error {
		return cmdPackageBuild([]string{"core", "--repo-root", dir})
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"unit=package.build.core",
		"status=start",
		"status=done",
		"package=dockpipe.core",
		"result=built",
		"version=1.2.3",
		"templates-core-1.2.3.tar.gz",
		"install-manifest.json",
	} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("expected core build stderr to contain %q, got:\n%s", want, stderr)
		}
	}
}

func TestRunPackageBuildStoreFromFlagsEmitsOperationResults(t *testing.T) {
	dir := t.TempDir()
	coreDir := filepath.Join(dir, infrastructure.DockpipeDirRel, "internal", "packages", "core")
	if err := os.MkdirAll(filepath.Join(coreDir, "runtimes"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(coreDir, "runtimes", ".keep"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	manifest := `schema: 1
name: dockpipe.core
version: 2.3.4
title: Core
description: d
author: a
website: https://example.com
license: Apache-2.0
kind: core
`
	if err := os.WriteFile(filepath.Join(coreDir, "package.yml"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}

	stderr, err := captureResultStderr(t, func() error {
		return RunPackageBuildStoreFromFlags(dir, "", "core", "9.9.9")
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"unit=package.build.store",
		"status=start",
		"status=done",
		"slice=core",
		"count=1",
		"result=built",
		"packages-store-manifest.json",
	} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("expected store build stderr to contain %q, got:\n%s", want, stderr)
		}
	}
}
