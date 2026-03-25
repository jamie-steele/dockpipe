package packagebuild

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestBuildCompiledStore(t *testing.T) {
	root := t.TempDir()
	core := filepath.Join(root, "core")
	if err := os.MkdirAll(filepath.Join(core, "runtimes", "cli"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(core, "package.yml"), []byte(`schema: 1
name: dockpipe.core
version: 1.2.3
title: Core
description: d
author: a
website: https://example.com
license: MIT
kind: core
`), 0o644); err != nil {
		t.Fatal(err)
	}
	wf := filepath.Join(root, "workflows", "demo")
	if err := os.MkdirAll(wf, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wf, "package.yml"), []byte(`schema: 1
name: demo
version: 0.0.1
title: Demo
description: d
author: a
website: https://example.com
license: MIT
kind: workflow
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wf, "config.yml"), []byte("name: demo\nrun: echo hi\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "out")
	m, err := BuildCompiledStore(root, out, "9.9.9", "all")
	if err != nil {
		t.Fatal(err)
	}
	if m.Packages.Core == nil || m.Packages.Core.Version != "1.2.3" {
		t.Fatalf("core: %+v", m.Packages.Core)
	}
	if len(m.Packages.Workflows) != 1 || m.Packages.Workflows[0].Name != "demo" {
		t.Fatalf("workflows: %+v", m.Packages.Workflows)
	}
	b, err := os.ReadFile(filepath.Join(out, "packages-store-manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var dec StoreBuildManifest
	if err := json.Unmarshal(b, &dec); err != nil {
		t.Fatal(err)
	}
	if dec.Packages.Core.Tarball == "" {
		t.Fatal("empty core tarball in manifest")
	}
}
