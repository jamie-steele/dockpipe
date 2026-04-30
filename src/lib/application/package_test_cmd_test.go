package application

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRunPackageTestFromFlagsRunsDeclaredScripts(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	cfg := `{
  "schema": 1,
  "compile": {
    "workflows": ["packages"]
  }
}`
	if err := os.WriteFile(filepath.Join(repo, "dockpipe.config.json"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	pkgDir := filepath.Join(repo, "packages", "alpha")
	if err := os.MkdirAll(filepath.Join(pkgDir, "tests"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkgDir, "package.yml"), []byte(`schema: 1
name: alpha
version: 1.0.0
title: Alpha
description: d
author: a
license: Apache-2.0
kind: package
test:
  script: tests/run.sh
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkgDir, "tests", "run.sh"), []byte("#!/usr/bin/env bash\nset -euo pipefail\nprintf ok > \"$DOCKPIPE_WORKDIR/alpha.test\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := RunPackageTestFromFlags(repo, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(repo, "alpha.test")); err != nil {
		t.Fatalf("expected package test output: %v", err)
	}
}

func TestRunPackageTestFromFlagsOnlyFilter(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	cfg := `{
  "schema": 1,
  "compile": {
    "workflows": ["packages"]
  }
}`
	if err := os.WriteFile(filepath.Join(repo, "dockpipe.config.json"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"alpha", "beta"} {
		pkgDir := filepath.Join(repo, "packages", name)
		if err := os.MkdirAll(filepath.Join(pkgDir, "tests"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(pkgDir, "package.yml"), []byte("schema: 1\nname: "+name+"\nversion: 1.0.0\ntitle: "+name+"\ndescription: d\nauthor: a\nlicense: Apache-2.0\nkind: package\ntest:\n  script: tests/run.sh\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		script := "#!/usr/bin/env bash\nset -euo pipefail\nprintf " + name + " > \"$DOCKPIPE_WORKDIR/" + name + ".test\"\n"
		if err := os.WriteFile(filepath.Join(pkgDir, "tests", "run.sh"), []byte(script), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := RunPackageTestFromFlags(repo, "beta"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(repo, "alpha.test")); !os.IsNotExist(err) {
		t.Fatalf("expected alpha to be skipped, got err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(repo, "beta.test")); err != nil {
		t.Fatalf("expected beta test output: %v", err)
	}
}

func TestRunPackageTestFromFlagsFindsNestedPackageManifests(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	cfg := `{
  "schema": 1,
  "compile": {
    "workflows": ["packages"]
  }
}`
	if err := os.WriteFile(filepath.Join(repo, "dockpipe.config.json"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	pkgDir := filepath.Join(repo, "packages", "dorkpipe", "mcp")
	if err := os.MkdirAll(filepath.Join(pkgDir, "tests"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkgDir, "package.yml"), []byte(`schema: 1
name: dorkpipe.mcp
version: 1.0.0
title: MCP
description: d
author: a
license: Apache-2.0
kind: package
test:
  script: tests/run.sh
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkgDir, "tests", "run.sh"), []byte("#!/usr/bin/env bash\nset -euo pipefail\nprintf ok > \"$DOCKPIPE_WORKDIR/dorkpipe-mcp.test\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := RunPackageTestFromFlags(repo, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(repo, "dorkpipe-mcp.test")); err != nil {
		t.Fatalf("expected nested package test output: %v", err)
	}
}

func TestRunPackageTestFromFlagsUsesCompileWorkflowRoots(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	cfg := `{
  "schema": 1,
  "compile": {
    "workflows": ["vendor"]
  }
}`
	if err := os.WriteFile(filepath.Join(repo, "dockpipe.config.json"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	pkgDir := filepath.Join(repo, "vendor", "suite", "gamma")
	if err := os.MkdirAll(filepath.Join(pkgDir, "tests"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkgDir, "package.yml"), []byte(`schema: 1
name: gamma
version: 1.0.0
title: Gamma
description: d
author: a
license: Apache-2.0
kind: package
test:
  script: tests/run.sh
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkgDir, "tests", "run.sh"), []byte("#!/usr/bin/env bash\nset -euo pipefail\nprintf ok > \"$DOCKPIPE_WORKDIR/gamma.test\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := RunPackageTestFromFlags(repo, "gamma"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(repo, "gamma.test")); err != nil {
		t.Fatalf("expected vendor-root package test output: %v", err)
	}
}
