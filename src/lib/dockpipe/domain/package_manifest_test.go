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
