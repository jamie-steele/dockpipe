package domain

import (
	"os"
	"path/filepath"
	"testing"
)

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
license: MIT
kind: workflow
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
tags: [resolver, codex]
keywords: [ai]
min_dockpipe_version: "1.0.0"
repository: https://github.com/acme/resolver
provides: [codex]
requires_resolvers: []
depends: [base-pack]
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
}
