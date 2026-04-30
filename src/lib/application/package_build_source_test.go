package application

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunPackageBuildSourceFromFlagsRunsDeclaredScript(t *testing.T) {
	dir := t.TempDir()
	pkgDir := filepath.Join(dir, "packages", "demo")
	cfg := `{
  "schema": 1,
  "compile": {
    "workflows": ["packages"]
  }
}`
	if err := os.WriteFile(filepath.Join(dir, "dockpipe.config.json"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(pkgDir, "assets", "scripts"), 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := `schema: 1
name: demo
version: 1.0.0
title: Demo
description: d
author: a
website: https://example.com
license: Apache-2.0
kind: package
build:
  source:
    script: assets/scripts/build-source.sh
`
	if err := os.WriteFile(filepath.Join(pkgDir, "package.yml"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	script := "#!/usr/bin/env bash\nset -euo pipefail\nprintf '%s\\n' \"$DOCKPIPE_PACKAGE_ROOT\" > built.txt\n"
	if err := os.WriteFile(filepath.Join(pkgDir, "assets", "scripts", "build-source.sh"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := RunPackageBuildSourceFromFlags(dir, ""); err != nil {
		t.Fatal(err)
	}
	built, err := os.ReadFile(filepath.Join(pkgDir, "built.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(string(built)); got != pkgDir {
		t.Fatalf("built.txt = %q, want %q", got, pkgDir)
	}
}

func TestRunPackageBuildSourceFromFlagsOnlyMatchesOnePackage(t *testing.T) {
	dir := t.TempDir()
	cfg := `{
  "schema": 1,
  "compile": {
    "workflows": ["packages"]
  }
}`
	if err := os.WriteFile(filepath.Join(dir, "dockpipe.config.json"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"demo", "other"} {
		pkgDir := filepath.Join(dir, "packages", name)
		if err := os.MkdirAll(filepath.Join(pkgDir, "assets", "scripts"), 0o755); err != nil {
			t.Fatal(err)
		}
		manifest := "schema: 1\nname: " + name + "\nversion: 1.0.0\ntitle: T\ndescription: d\nauthor: a\nwebsite: https://example.com\nlicense: Apache-2.0\nkind: package\nbuild:\n  source:\n    script: assets/scripts/build-source.sh\n"
		if err := os.WriteFile(filepath.Join(pkgDir, "package.yml"), []byte(manifest), 0o644); err != nil {
			t.Fatal(err)
		}
		script := "#!/usr/bin/env bash\nset -euo pipefail\ntouch built-" + name + "\n"
		if err := os.WriteFile(filepath.Join(pkgDir, "assets", "scripts", "build-source.sh"), []byte(script), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := RunPackageBuildSourceFromFlags(dir, "demo"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "packages", "demo", "built-demo")); err != nil {
		t.Fatalf("demo script did not run: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "packages", "other", "built-other")); !os.IsNotExist(err) {
		t.Fatalf("other script should not have run, got err=%v", err)
	}
}

func TestRunPackageBuildSourceFromFlagsUsesCompileWorkflowRoots(t *testing.T) {
	dir := t.TempDir()
	cfg := `{
  "schema": 1,
  "compile": {
    "workflows": ["vendor"]
  }
}`
	if err := os.WriteFile(filepath.Join(dir, "dockpipe.config.json"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	pkgDir := filepath.Join(dir, "vendor", "acme", "demo")
	if err := os.MkdirAll(filepath.Join(pkgDir, "assets", "scripts"), 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := `schema: 1
name: demo
version: 1.0.0
title: Demo
description: d
author: a
website: https://example.com
license: Apache-2.0
kind: package
build:
  source:
    script: assets/scripts/build-source.sh
`
	if err := os.WriteFile(filepath.Join(pkgDir, "package.yml"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	script := "#!/usr/bin/env bash\nset -euo pipefail\ntouch built-from-vendor\n"
	if err := os.WriteFile(filepath.Join(pkgDir, "assets", "scripts", "build-source.sh"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := RunPackageBuildSourceFromFlags(dir, "demo"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(pkgDir, "built-from-vendor")); err != nil {
		t.Fatalf("expected vendor-root build output: %v", err)
	}
}
