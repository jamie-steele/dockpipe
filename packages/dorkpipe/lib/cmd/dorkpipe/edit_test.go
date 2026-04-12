package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLooksLikePackageCreation_TryYourHandPrompt(t *testing.T) {
	t.Parallel()
	prompt := "try your hand at fibonacci number sequence dockpipe package in .staging/packages"
	if !looksLikePackageCreation(prompt) {
		t.Fatalf("expected prompt to route through package creation heuristic")
	}
}

func TestInferRequestedPackagePurpose_TryYourHandPrompt(t *testing.T) {
	t.Parallel()
	got := inferRequestedPackagePurpose("Try your hand at fibonacci number sequence dockpipe package in .staging/packages make sure it is new")
	want := "fibonacci number sequence"
	if got != want {
		t.Fatalf("inferRequestedPackagePurpose() = %q, want %q", got, want)
	}
}

func TestInferPackageScaffoldSpec_StagingFibonacciPrompt(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	seedDir := filepath.Join(root, ".staging", "packages", "existing")
	if err := os.MkdirAll(seedDir, 0o755); err != nil {
		t.Fatalf("mkdir seed dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(seedDir, "package.yml"), []byte("schema: 1\nkind: package\nname: existing\n"), 0o644); err != nil {
		t.Fatalf("write seed package: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "VERSION"), []byte("0.6.0\n"), 0o644); err != nil {
		t.Fatalf("write version: %v", err)
	}

	spec, ok := inferPackageScaffoldSpec(root, "Try your hand at fibonacci number sequence dockpipe package in .staging/packages make sure you respect AGENTS.md ensure its a new package for this")
	if !ok {
		t.Fatalf("expected scaffold spec")
	}
	if spec.PackageRoot != ".staging/packages" {
		t.Fatalf("PackageRoot = %q, want %q", spec.PackageRoot, ".staging/packages")
	}
	if spec.PackageName != "fibonacci" {
		t.Fatalf("PackageName = %q, want %q", spec.PackageName, "fibonacci")
	}
	if spec.ManifestPath != ".staging/packages/fibonacci/package.yml" {
		t.Fatalf("ManifestPath = %q", spec.ManifestPath)
	}
	if spec.ReadmePath != ".staging/packages/fibonacci/README.md" {
		t.Fatalf("ReadmePath = %q", spec.ReadmePath)
	}
	if spec.Confidence < 0.85 {
		t.Fatalf("Confidence = %f, want >= 0.85", spec.Confidence)
	}
}
