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
	if err := os.MkdirAll(filepath.Join(core, "runtimes", "dockerimage"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(core, "package.yml"), []byte(`schema: 1
name: dockpipe.core
version: 1.2.3
title: Core
description: d
author: a
website: https://example.com
license: Apache-2.0
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
license: Apache-2.0
kind: workflow
provider: cloudflare
requires_capabilities: [cli.codex]
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
	if len(m.Packages.Workflows) != 1 || m.Packages.Workflows[0].Name != "demo" || m.Packages.Workflows[0].Provider != "cloudflare" {
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
	if len(dec.Packages.Workflows) != 1 || dec.Packages.Workflows[0].Provider != "cloudflare" {
		t.Fatalf("workflow provider in JSON: %+v", dec.Packages.Workflows)
	}
	if len(dec.Packages.Workflows[0].RequiresCapabilities) != 1 || dec.Packages.Workflows[0].RequiresCapabilities[0] != "cli.codex" {
		t.Fatalf("requires_capabilities in JSON: %+v", dec.Packages.Workflows)
	}
}

func TestBuildCompiledStoreFromTarballLayout(t *testing.T) {
	root := t.TempDir()
	coreDir := filepath.Join(root, "core")
	wfDir := filepath.Join(root, "workflows")
	resDir := filepath.Join(root, "resolvers")
	for _, dir := range []string{coreDir, wfDir, resDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	coreSrc := filepath.Join(t.TempDir(), "core-src")
	if err := os.MkdirAll(filepath.Join(coreSrc, "runtimes"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(coreSrc, "package.yml"), []byte(`schema: 1
name: dockpipe.core
version: 0.6.0
title: Core
description: d
author: a
website: https://example.com
license: Apache-2.0
kind: core
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := WriteDirTarGzWithPrefix(coreSrc, filepath.Join(coreDir, "dockpipe-core-0.6.0.tar.gz"), "core"); err != nil {
		t.Fatal(err)
	}

	wfSrc := filepath.Join(t.TempDir(), "wf-src")
	if err := os.MkdirAll(wfSrc, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wfSrc, "package.yml"), []byte(`schema: 1
name: demo
version: 0.6.0
title: Demo
description: d
author: a
website: https://example.com
license: Apache-2.0
kind: workflow
provider: cloudflare
requires_capabilities: [cli.codex]
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wfSrc, "config.yml"), []byte("name: demo\nnamespace: acme\nrun: echo hi\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := WriteDirTarGzWithPrefix(wfSrc, filepath.Join(wfDir, "dockpipe-workflow-demo-0.6.0.tar.gz"), "workflows/demo"); err != nil {
		t.Fatal(err)
	}

	resSrc := filepath.Join(t.TempDir(), "res-src")
	if err := os.MkdirAll(filepath.Join(resSrc, "profile"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(resSrc, "package.yml"), []byte(`schema: 1
name: codex
version: 0.6.0
title: Codex
description: d
author: a
website: https://example.com
license: Apache-2.0
kind: resolver
capability: cli.codex
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := WriteDirTarGzWithPrefix(resSrc, filepath.Join(resDir, "dockpipe-resolver-codex-0.6.0.tar.gz"), "resolvers/codex"); err != nil {
		t.Fatal(err)
	}

	out := filepath.Join(t.TempDir(), "out")
	m, err := BuildCompiledStore(root, out, "9.9.9", "all")
	if err != nil {
		t.Fatal(err)
	}
	if m.Packages.Core == nil || m.Packages.Core.Version != "0.6.0" {
		t.Fatalf("core: %+v", m.Packages.Core)
	}
	if len(m.Packages.Workflows) != 1 || m.Packages.Workflows[0].Name != "demo" {
		t.Fatalf("workflows: %+v", m.Packages.Workflows)
	}
	if len(m.Packages.Resolvers) != 1 || m.Packages.Resolvers[0].Name != "codex" {
		t.Fatalf("resolvers: %+v", m.Packages.Resolvers)
	}
	if _, err := os.Stat(filepath.Join(out, "dockpipe-workflow-demo-0.6.0.tar.gz")); err != nil {
		t.Fatalf("expected workflow tarball copy: %v", err)
	}
	if _, err := os.Stat(filepath.Join(out, "dockpipe-resolver-codex-0.6.0.tar.gz")); err != nil {
		t.Fatalf("expected resolver tarball copy: %v", err)
	}
}
