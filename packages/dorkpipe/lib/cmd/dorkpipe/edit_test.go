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

func TestParseEditArtifact_PatchObjectContent(t *testing.T) {
	t.Parallel()
	text := `{
  "summary": "create files",
  "files": [".staging/packages/fibonacci/package.yml"],
  "patch": {
    "content": "diff --git a/a b/a\n--- /dev/null\n+++ b/a\n@@ -0,0 +1 @@\n+hi\n"
  },
  "validation": "check it"
}`
	artifact, diag, err := parseEditArtifact(text)
	if err != nil {
		t.Fatalf("parseEditArtifact() error = %v", err)
	}
	if diag == nil || diag.PatchSource != "object:content" || diag.TargetFilesSource != "files" || diag.ValidationsSource != "validation" {
		t.Fatalf("unexpected diagnostics: %#v", diag)
	}
	if artifact.Patch == "" {
		t.Fatalf("expected normalized patch content")
	}
	if len(artifact.TargetFiles) != 1 || artifact.TargetFiles[0] != ".staging/packages/fibonacci/package.yml" {
		t.Fatalf("unexpected target files: %#v", artifact.TargetFiles)
	}
	if len(artifact.Validations) != 1 || artifact.Validations[0] != "check it" {
		t.Fatalf("unexpected validations: %#v", artifact.Validations)
	}
}

func TestParseEditArtifact_RepairsMultilinePatchString(t *testing.T) {
	t.Parallel()
	text := "{\n" +
		`"summary":"create files",` + "\n" +
		`"target_files":[".staging/packages/fibonacci/package.yml"],` + "\n" +
		`"patch":"diff --git a/.staging/packages/fibonacci/package.yml b/.staging/packages/fibonacci/package.yml` + "\n" +
		`--- /dev/null` + "\n" +
		`+++ b/.staging/packages/fibonacci/package.yml` + "\n" +
		`@@ -0,0 +1 @@` + "\n" +
		`+schema: 1",` + "\n" +
		`"validations":["check it"]` + "\n" +
		"}"
	artifact, diag, err := parseEditArtifact(text)
	if err != nil {
		t.Fatalf("parseEditArtifact() error = %v", err)
	}
	if diag == nil || len(diag.AppliedRepairs) == 0 {
		t.Fatalf("expected multiline repair diagnostics, got %#v", diag)
	}
	if artifact.Patch == "" || artifact.Summary != "create files" {
		t.Fatalf("unexpected artifact: %#v", artifact)
	}
}

func TestParseEditArtifact_AcceptsStringTargetsAndChecks(t *testing.T) {
	t.Parallel()
	text := `{
  "summary": "update thing",
  "targets": ".staging/packages/fibonacci/package.yml, .staging/packages/fibonacci/README.md",
  "diff": "diff --git a/a b/a\n--- /dev/null\n+++ b/a\n@@ -0,0 +1 @@\n+hi\n",
  "checks": "verify file content\nverify readme"
}`
	artifact, diag, err := parseEditArtifact(text)
	if err != nil {
		t.Fatalf("parseEditArtifact() error = %v", err)
	}
	if diag == nil || diag.TargetFilesSource != "targets" || diag.ValidationsSource != "checks" || diag.PatchSource != "string" {
		t.Fatalf("unexpected diagnostics: %#v", diag)
	}
	if len(artifact.TargetFiles) != 2 {
		t.Fatalf("unexpected target files: %#v", artifact.TargetFiles)
	}
	if len(artifact.Validations) != 2 {
		t.Fatalf("unexpected validations: %#v", artifact.Validations)
	}
}
