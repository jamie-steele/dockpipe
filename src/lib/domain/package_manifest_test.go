package domain

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParsePackageManifestKindPackageIncludesResolvers(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	p := filepath.Join(dir, "package.yml")
	body := `schema: 1
name: agent
version: 0.1.0
title: Agent package
description: Umbrella for agent resolvers
author: DockPipe
license: Apache-2.0
kind: package
includes_resolvers: [codex, claude]
`
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	m, err := ParsePackageManifest(p)
	if err != nil {
		t.Fatal(err)
	}
	if m.Kind != "package" || len(m.IncludesResolvers) != 2 {
		t.Fatalf("got kind=%q includes=%v", m.Kind, m.IncludesResolvers)
	}
}

func TestParsePackageManifest(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	p := filepath.Join(dir, "package.yml")
	body := `schema: 1
name: demo
version: 1.0.0
title: Demo
description: A demo package
author: ACME
website: https://example.com
license: Apache-2.0
kind: workflow
requires_capabilities: [workflow.demo]
`
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	m, err := ParsePackageManifest(p)
	if err != nil {
		t.Fatal(err)
	}
	if m.Name != "demo" || m.Version != "1.0.0" || m.Author != "ACME" || m.Website != "https://example.com" {
		t.Fatalf("got %+v", m)
	}
}

func TestParsePackageManifestRichMetadata(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	p := filepath.Join(dir, "package.yml")
	body := `schema: 1
name: acme-resolver
version: 2.0.0
title: ACME Resolver
description: Tool adapter
author: ACME
website: https://example.com
license: Apache-2.0
kind: resolver
capability: cli.codex
tags: [resolver, codex]
keywords: [ai]
min_dockpipe_version: "1.0.0"
repository: https://github.com/acme/resolver
provides: [codex]
requires_resolvers: []
depends: [base-pack]
icon: assets/images/icon.png
artwork:
  vscode: assets/images/vscode.png
`
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	m, err := ParsePackageManifest(p)
	if err != nil {
		t.Fatal(err)
	}
	if m.Kind != "resolver" || len(m.Tags) != 2 || m.Tags[0] != "resolver" {
		t.Fatalf("tags/kind: %+v", m)
	}
	if m.MinDockpipeVersion != "1.0.0" || m.Repository != "https://github.com/acme/resolver" {
		t.Fatalf("version/repo: %+v", m)
	}
	if len(m.Provides) != 1 || m.Provides[0] != "codex" {
		t.Fatalf("provides: %+v", m)
	}
	if len(m.Depends) != 1 || m.Depends[0] != "base-pack" {
		t.Fatalf("depends: %+v", m)
	}
	if m.Icon != "assets/images/icon.png" {
		t.Fatalf("icon: %+v", m)
	}
	if got := m.Artwork["vscode"]; got != "assets/images/vscode.png" {
		t.Fatalf("artwork: %+v", m.Artwork)
	}
}

func TestParsePackageManifestRejectReservedNamespace(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	p := filepath.Join(dir, "package.yml")
	body := `schema: 1
name: x
version: 1.0.0
title: X
description: d
author: a
website: https://example.com
license: Apache-2.0
kind: workflow
requires_capabilities: [workflow.x]
namespace: dockpipe
`
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := ParsePackageManifest(p)
	if err == nil {
		t.Fatal("expected error for reserved namespace")
	}
}

func TestParsePackageManifestNamespaceOK(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	p := filepath.Join(dir, "package.yml")
	body := `schema: 1
name: x
version: 1.0.0
title: X
description: d
author: a
website: https://example.com
license: Apache-2.0
kind: workflow
requires_capabilities: [workflow.x]
namespace: acme-labs
`
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	m, err := ParsePackageManifest(p)
	if err != nil {
		t.Fatal(err)
	}
	if m.Namespace != "acme-labs" {
		t.Fatalf("namespace: %q", m.Namespace)
	}
}

func TestParsePackageManifestProvider(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	p := filepath.Join(dir, "package.yml")
	body := `schema: 1
name: x
version: 1.0.0
title: X
description: d
author: a
website: https://example.com
license: Apache-2.0
kind: workflow
requires_capabilities: [workflow.x]
provider: cloudflare
`
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	m, err := ParsePackageManifest(p)
	if err != nil {
		t.Fatal(err)
	}
	if m.Provider != "cloudflare" {
		t.Fatalf("provider: %q", m.Provider)
	}
}

func TestParsePackageManifestCapability(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	p := filepath.Join(dir, "package.yml")
	body := `schema: 1
name: codex-pack
version: 1.0.0
title: Codex
description: d
author: a
website: https://example.com
license: Apache-2.0
kind: resolver
capability: cli.codex
`
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	m, err := ParsePackageManifest(p)
	if err != nil {
		t.Fatal(err)
	}
	if m.Capability != "cli.codex" {
		t.Fatalf("capability: %q", m.Capability)
	}
}

func TestParsePackageManifestRequiresCapabilities(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	p := filepath.Join(dir, "package.yml")
	body := `schema: 1
name: wf
version: 1.0.0
title: W
description: d
author: a
website: https://example.com
license: Apache-2.0
kind: workflow
requires_capabilities: [cli.codex, app.vscode]
`
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	m, err := ParsePackageManifest(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(m.RequiresCapabilities) != 2 || m.RequiresCapabilities[0] != "cli.codex" {
		t.Fatalf("requires_capabilities: %+v", m.RequiresCapabilities)
	}
}

func TestParsePackageManifestProviderRejectsTooLong(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	p := filepath.Join(dir, "package.yml")
	long := strings.Repeat("a", 257)
	body := `schema: 1
name: x
version: 1.0.0
title: X
description: d
author: a
website: https://example.com
license: Apache-2.0
kind: workflow
requires_capabilities: [workflow.x]
provider: ` + long + "\n"
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := ParsePackageManifest(p)
	if err == nil {
		t.Fatal("expected error for provider longer than 256")
	}
}

func TestParsePackageManifestAllowClone(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	p := filepath.Join(dir, "package.yml")
	body := `schema: 1
name: x
version: 1.0.0
title: X
description: d
author: a
website: https://example.com
license: Apache-2.0
kind: workflow
requires_capabilities: [workflow.x]
allow_clone: true
distribution: binary
`
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	m, err := ParsePackageManifest(p)
	if err != nil {
		t.Fatal(err)
	}
	if !m.AllowClone || m.Distribution != "binary" {
		t.Fatalf("got %+v", m)
	}
}

func TestParsePackageManifestImageRegistry(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	p := filepath.Join(dir, "package.yml")
	body := `schema: 1
name: x
version: 1.0.0
title: X
description: d
author: a
website: https://example.com
license: Apache-2.0
kind: workflow
image:
  source: registry
  ref: ghcr.io/acme/tool@sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef
  pull_policy: if-missing
`
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	m, err := ParsePackageManifest(p)
	if err != nil {
		t.Fatal(err)
	}
	if m.Image.Ref == "" || m.Image.PullPolicy != "if-missing" {
		t.Fatalf("unexpected image metadata: %+v", m.Image)
	}
}

func TestParsePackageManifestBuildSourceScript(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	p := filepath.Join(dir, "package.yml")
	body := `schema: 1
name: x
version: 1.0.0
title: X
description: d
author: a
website: https://example.com
license: Apache-2.0
kind: package
build:
  source:
    script: assets/scripts/build-source.sh
`
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	m, err := ParsePackageManifest(p)
	if err != nil {
		t.Fatal(err)
	}
	if m.Build.Source == nil || m.Build.Source.Script != "assets/scripts/build-source.sh" {
		t.Fatalf("unexpected build source: %+v", m.Build)
	}
}

func TestParsePackageManifestRejectsEscapingBuildSourceScript(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	p := filepath.Join(dir, "package.yml")
	body := `schema: 1
name: x
version: 1.0.0
title: X
description: d
author: a
website: https://example.com
license: Apache-2.0
kind: package
build:
  source:
    script: ../build.sh
`
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := ParsePackageManifest(p)
	if err == nil || !strings.Contains(err.Error(), "build.source.script") {
		t.Fatalf("expected build.source.script validation error, got %v", err)
	}
}

func TestParsePackageManifestTestScript(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	p := filepath.Join(dir, "package.yml")
	body := `schema: 1
name: x
version: 1.0.0
title: X
description: d
author: a
website: https://example.com
license: Apache-2.0
kind: package
test:
  script: tests/run.sh
`
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	m, err := ParsePackageManifest(p)
	if err != nil {
		t.Fatal(err)
	}
	if m.Test.Script != "tests/run.sh" {
		t.Fatalf("unexpected test script: %+v", m.Test)
	}
}

func TestParsePackageManifestRejectsEscapingTestScript(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	p := filepath.Join(dir, "package.yml")
	body := `schema: 1
name: x
version: 1.0.0
title: X
description: d
author: a
website: https://example.com
license: Apache-2.0
kind: package
test:
  script: ../tests/run.sh
`
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := ParsePackageManifest(p)
	if err == nil || !strings.Contains(err.Error(), "test.script") {
		t.Fatalf("expected test.script validation error, got %v", err)
	}
}

func TestParsePackageManifestRejectsInvalidImagePullPolicy(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	p := filepath.Join(dir, "package.yml")
	body := `schema: 1
name: x
version: 1.0.0
title: X
description: d
author: a
website: https://example.com
license: Apache-2.0
kind: workflow
image:
  ref: ghcr.io/acme/tool:latest
  pull_policy: always
`
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := ParsePackageManifest(p)
	if err == nil || !strings.Contains(err.Error(), "pull_policy") {
		t.Fatalf("expected pull_policy validation error, got %v", err)
	}
}

func TestParsePackageManifestRejectsInvalidVersion(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	p := filepath.Join(dir, "package.yml")
	body := `schema: 1
name: demo
version: latest
title: Demo
description: A demo package
author: ACME
website: https://example.com
license: Apache-2.0
kind: workflow
`
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := ParsePackageManifest(p)
	if err == nil || !strings.Contains(err.Error(), "semver-like") {
		t.Fatalf("expected semver-like version error, got %v", err)
	}
}
