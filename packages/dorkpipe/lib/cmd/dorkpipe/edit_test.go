package main

import (
	"encoding/json"
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

func TestParseEditArtifact_AcceptsStructuredEdits(t *testing.T) {
	t.Parallel()
	text := `{
  "summary": "update settings renderer",
  "target_files": ["src/index.ts"],
  "patch": "diff --git a/src/index.ts b/src/index.ts\n--- a/src/index.ts\n+++ b/src/index.ts\n@@ -1 +1 @@\n-old\n+new\n",
  "structured_edits": [
    {
      "id": "replace_range-src-index-ts-1",
      "op": "replace_range",
      "language": "typescript",
      "target_file": "src/index.ts",
      "description": "Update function renderSettings in src/index.ts.",
      "target": {
        "kind": "function",
        "symbol_name": "renderSettings",
        "symbol_kind": "function"
      },
      "range": {
        "start_line": 1,
        "old_line_count": 1,
        "new_line_count": 1
      },
      "old_text": "old\n",
      "new_text": "new\n"
    }
  ]
}`
	artifact, diag, err := parseEditArtifact(text)
	if err != nil {
		t.Fatalf("parseEditArtifact() error = %v", err)
	}
	if diag == nil || diag.StructuredEditsSource != "structured_edits" {
		t.Fatalf("unexpected diagnostics: %#v", diag)
	}
	if len(artifact.StructuredEdits) != 1 {
		t.Fatalf("expected structured edit, got %#v", artifact.StructuredEdits)
	}
	if artifact.StructuredEdits[0].Target.SymbolName != "renderSettings" {
		t.Fatalf("unexpected structured target: %#v", artifact.StructuredEdits[0].Target)
	}
}

func TestApplyStructuredEdits_ReplaceRange(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	target := filepath.Join(root, "src", "index.ts")
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(target, []byte("const value = 1;\nconsole.log(value);\n"), 0o644); err != nil {
		t.Fatalf("write before: %v", err)
	}
	artifact := &editModelArtifact{
		ArtifactVersion: editArtifactVersion,
		Summary:         "update console line",
		TargetFiles:     []string{"src/index.ts"},
		Patch:           "diff --git a/src/index.ts b/src/index.ts\n--- a/src/index.ts\n+++ b/src/index.ts\n@@ -2 +2 @@\n-console.log(value);\n+console.log('done');\n",
		StructuredEdits: []editStructuredEdit{
			{
				ID:         "replace_range-src-index-ts-1",
				Op:         "replace_range",
				Language:   "typescript",
				TargetFile: "src/index.ts",
				Range: &editStructuredRange{
					StartLine:    2,
					OldLineCount: 1,
					NewLineCount: 1,
				},
				OldText: "console.log(value);\n",
				NewText: "console.log('done');\n",
			},
		},
	}
	output, err := applyStructuredEdits(root, artifact)
	if err != nil {
		t.Fatalf("applyStructuredEdits() error = %v", err)
	}
	if output == "" {
		t.Fatalf("expected apply output")
	}
	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read after: %v", err)
	}
	if string(got) != "const value = 1;\nconsole.log('done');\n" {
		t.Fatalf("unexpected file contents: %q", string(got))
	}
}

func TestPrepareArtifactForStorage_DerivesStructuredEditsFromCreatedFiles(t *testing.T) {
	t.Parallel()
	artifact := &editModelArtifact{
		Summary:     "create readme",
		TargetFiles: []string{"README.md"},
		Patch:       "diff --git a/README.md b/README.md\nnew file mode 100644\n--- /dev/null\n+++ b/README.md\n@@ -0,0 +1 @@\n+hello\n",
		CreatedFiles: map[string]string{
			"README.md": "hello\n",
		},
	}
	prepared := prepareArtifactForStorage("", artifact)
	if prepared.ArtifactVersion != editArtifactVersion {
		t.Fatalf("ArtifactVersion = %q, want %q", prepared.ArtifactVersion, editArtifactVersion)
	}
	if len(prepared.StructuredEdits) == 0 {
		t.Fatalf("expected structured edits to be derived")
	}
	body, err := json.Marshal(prepared)
	if err != nil {
		t.Fatalf("marshal prepared artifact: %v", err)
	}
	if !json.Valid(body) {
		t.Fatalf("prepared artifact should marshal as valid JSON")
	}
}
