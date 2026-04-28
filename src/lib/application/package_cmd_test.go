package application

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"dockpipe/src/lib/domain"
	"dockpipe/src/lib/infrastructure"
	"dockpipe/src/lib/infrastructure/packagebuild"
)

func TestCmdPackageListFindsPackageYml(t *testing.T) {
	dir := t.TempDir()
	pkgRoot := filepath.Join(dir, infrastructure.DockpipeDirRel, "internal", "packages", "workflows", "demo")
	if err := os.MkdirAll(pkgRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := `schema: 1
name: demo
version: 1.0.0
description: hello
`
	if err := os.WriteFile(filepath.Join(pkgRoot, "package.yml"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldWd) })

	if err := cmdPackage([]string{"list"}); err != nil {
		t.Fatal(err)
	}
	// stderr printed to os.Stderr; we only assert command succeeds.
}

func TestCmdPackageImagesMergesPlannedAndMaterializedArtifacts(t *testing.T) {
	dir := t.TempDir()
	buildDir := filepath.Join(dir, "src", "core", "assets", "images", "codex")
	if err := os.MkdirAll(buildDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(buildDir, "Dockerfile"), []byte("FROM alpine\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	artifact, err := buildImageArtifactManifest(dir, "mywf", "mywf", "codex", "dockpipe-codex:test", buildDir, dir, "sha256:policy", domain.ImageArtifactProvenance{Resolver: "codex"})
	if err != nil {
		t.Fatal(err)
	}
	stage := filepath.Join(dir, "stage")
	manifestDir := filepath.Join(stage, domain.RuntimeManifestDirName)
	if err := os.MkdirAll(manifestDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := writeJSONFile(filepath.Join(manifestDir, domain.ImageArtifactFileName), artifact); err != nil {
		t.Fatal(err)
	}
	pkgDir, err := infrastructure.PackagesWorkflowsDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(pkgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	tgz := filepath.Join(pkgDir, "dockpipe-workflow-mywf-1.2.3.tar.gz")
	if _, err := packagebuild.WriteDirTarGzWithPrefix(stage, tgz, "workflows/mywf"); err != nil {
		t.Fatal(err)
	}
	indexed := *artifact
	indexed.ArtifactState = "materialized"
	if err := persistImageArtifactIndexRecord(dir, &indexed); err != nil {
		t.Fatal(err)
	}
	records, err := collectPackageImageArtifacts(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 {
		t.Fatalf("expected one merged image artifact, got %d", len(records))
	}
	if records[0].ArtifactState != "materialized" || records[0].ImageRef != "dockpipe-codex:test" {
		t.Fatalf("unexpected merged image artifact: %+v", records[0])
	}
	rows, err := collectPackageImageArtifactRows(dir, func(image string) (bool, error) {
		if image != "dockpipe-codex:test" {
			t.Fatalf("unexpected image check %q", image)
		}
		return true, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Status != "ready" || rows[0].Artifact.ArtifactState != "materialized" {
		t.Fatalf("unexpected image artifact rows: %+v", rows)
	}
	oldExists := dockerImageExistsAppFn
	t.Cleanup(func() { dockerImageExistsAppFn = oldExists })
	dockerImageExistsAppFn = func(string) (bool, error) { return true, nil }
	if err := cmdPackage([]string{"images", "--workdir", dir}); err != nil {
		t.Fatal(err)
	}
}

func TestCollectPackageImageArtifactRowsDetectsStaleIndex(t *testing.T) {
	dir := t.TempDir()
	buildDir := filepath.Join(dir, "src", "core", "assets", "images", "codex")
	if err := os.MkdirAll(buildDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(buildDir, "Dockerfile"), []byte("FROM alpine\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	planned, err := buildImageArtifactManifest(dir, "mywf", "mywf", "codex", "dockpipe-codex:test", buildDir, dir, "sha256:policy", domain.ImageArtifactProvenance{Resolver: "codex"})
	if err != nil {
		t.Fatal(err)
	}
	stage := filepath.Join(dir, "stage")
	manifestDir := filepath.Join(stage, domain.RuntimeManifestDirName)
	if err := os.MkdirAll(manifestDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := writeJSONFile(filepath.Join(manifestDir, domain.ImageArtifactFileName), planned); err != nil {
		t.Fatal(err)
	}
	pkgDir, err := infrastructure.PackagesWorkflowsDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(pkgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	tgz := filepath.Join(pkgDir, "dockpipe-workflow-mywf-1.2.3.tar.gz")
	if _, err := packagebuild.WriteDirTarGzWithPrefix(stage, tgz, "workflows/mywf"); err != nil {
		t.Fatal(err)
	}
	stale := *planned
	stale.ArtifactState = "materialized"
	stale.Fingerprint = "sha256:stale"
	if err := persistImageArtifactIndexRecord(dir, &stale); err != nil {
		t.Fatal(err)
	}
	rows, err := collectPackageImageArtifactRows(dir, func(string) (bool, error) { return true, nil })
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Status != "stale" {
		t.Fatalf("expected stale row, got %+v", rows)
	}
}

func TestCmdPackageManifest(t *testing.T) {
	if err := cmdPackage([]string{"manifest"}); err != nil {
		t.Fatal(err)
	}
}

func TestCmdPackageUnknown(t *testing.T) {
	err := cmdPackage([]string{"nope"})
	if err == nil || !strings.Contains(err.Error(), "unknown") {
		t.Fatalf("got %v", err)
	}
}

func TestCmdPackageCompileCore(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src", "core")
	if err := os.MkdirAll(filepath.Join(src, "runtimes"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "runtimes", ".keep"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "VERSION"), []byte("9.8.7\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldWd) })

	if err := cmdPackage([]string{"compile", "core", "--from", src}); err != nil {
		t.Fatal(err)
	}
	coreDir := filepath.Join(dir, infrastructure.DockpipeDirRel, "internal", "packages", "core")
	matches, err := filepath.Glob(filepath.Join(coreDir, "dockpipe-core-*.tar.gz"))
	if err != nil || len(matches) != 1 {
		t.Fatalf("expected one core tarball under %s: matches=%v err=%v", coreDir, matches, err)
	}
	if _, err := packagebuild.ReadFileFromTarGz(matches[0], "core/package.yml"); err != nil {
		t.Fatal(err)
	}
	if filepath.Base(matches[0]) != "dockpipe-core-9.8.7.tar.gz" {
		t.Fatalf("expected repo VERSION in core tarball name, got %s", filepath.Base(matches[0]))
	}
}

func TestCmdPackageCompileResolversVendorResolversSubdir(t *testing.T) {
	dir := t.TempDir()
	pack := filepath.Join(dir, "my-vendor")
	resRoot := filepath.Join(pack, "resolvers", "alpha")
	if err := os.MkdirAll(resRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(resRoot, "profile"), []byte("DOCKPIPE_RESOLVER_CMD=test\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "VERSION"), []byte("3.4.5\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldWd) })

	if err := cmdPackage([]string{"compile", "resolvers", "--from", pack}); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(dir, infrastructure.DockpipeDirRel, "internal", "packages", "resolvers")
	matches, err := filepath.Glob(filepath.Join(dest, "dockpipe-resolver-alpha-*.tar.gz"))
	if err != nil || len(matches) != 1 {
		t.Fatalf("expected one resolver tarball: matches=%v err=%v", matches, err)
	}
	if filepath.Base(matches[0]) != "dockpipe-resolver-alpha-3.4.5.tar.gz" {
		t.Fatalf("expected repo VERSION in resolver tarball name, got %s", filepath.Base(matches[0]))
	}
	if _, err := packagebuild.ReadFileFromTarGz(matches[0], "resolvers/alpha/profile"); err != nil {
		t.Fatal(err)
	}
}

func TestRunCompileAliasHelp(t *testing.T) {
	if err := Run([]string{"compile", "core", "--help"}, nil); err != nil {
		t.Fatal(err)
	}
}

func TestCmdPackageCompileWorkflow(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src", "mywf")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "VERSION"), []byte("2.3.4\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := `name: mywf
description: test
steps: []
`
	if err := os.WriteFile(filepath.Join(src, "config.yml"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := cmdPackage([]string{"compile", "workflow", "--workdir", dir, "--from", src}); err != nil {
		t.Fatal(err)
	}
	tgz := filepath.Join(dir, infrastructure.DockpipeDirRel, "internal", "packages", "workflows", "dockpipe-workflow-mywf-2.3.4.tar.gz")
	if _, err := os.Stat(tgz); err != nil {
		t.Fatal(err)
	}
	if _, err := packagebuild.ReadFileFromTarGz(tgz, "workflows/mywf/config.yml"); err != nil {
		t.Fatal(err)
	}
	pyml, err := packagebuild.ReadFileFromTarGz(tgz, "workflows/mywf/package.yml")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(pyml), "version: 2.3.4") {
		t.Fatalf("expected generated manifest version 2.3.4, got:\n%s", string(pyml))
	}
	rmf, err := packagebuild.ReadFileFromTarGz(tgz, "workflows/mywf/.dockpipe/runtime.effective.json")
	if err != nil {
		t.Fatal(err)
	}
	var rm domain.CompiledRuntimeManifest
	if err := json.Unmarshal(rmf, &rm); err != nil {
		t.Fatal(err)
	}
	if rm.Kind != domain.RuntimeManifestKind || rm.Security.Preset != "secure-default" {
		t.Fatalf("unexpected runtime manifest: %+v", rm)
	}
	if rm.Security.Network.Mode != "offline" || rm.Security.Network.Enforcement != "native" {
		t.Fatalf("expected offline native enforcement, got %+v", rm.Security.Network)
	}
}

func TestCmdPackageCompileWorkflowWritesImageArtifactForTemplateBuild(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src", "mywf")
	img := filepath.Join(dir, "src", "core", "assets", "images", "codex")
	runtimes := filepath.Join(dir, "src", "core", "runtimes")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(img, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(runtimes, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(runtimes, ".keep"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(img, "Dockerfile"), []byte("FROM alpine:3.20\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "VERSION"), []byte("5.6.7\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "version"), []byte("5.6.7\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := `name: mywf
isolate: codex
steps: []
`
	if err := os.WriteFile(filepath.Join(src, "config.yml"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := cmdPackage([]string{"compile", "workflow", "--workdir", dir, "--from", src}); err != nil {
		t.Fatal(err)
	}
	tgz := filepath.Join(dir, infrastructure.DockpipeDirRel, "internal", "packages", "workflows", "dockpipe-workflow-mywf-5.6.7.tar.gz")
	imf, err := packagebuild.ReadFileFromTarGz(tgz, "workflows/mywf/.dockpipe/image-artifact.json")
	if err != nil {
		t.Fatal(err)
	}
	var im domain.ImageArtifactManifest
	if err := json.Unmarshal(imf, &im); err != nil {
		t.Fatal(err)
	}
	if im.Kind != domain.ImageArtifactManifestKind || im.Source != "build" {
		t.Fatalf("unexpected image artifact: %+v", im)
	}
	if im.Build == nil || im.Build.Dockerfile != "src/core/assets/images/codex/Dockerfile" {
		t.Fatalf("unexpected build spec: %+v", im.Build)
	}
	if im.ImageRef != "dockpipe-codex:5.6.7" {
		t.Fatalf("unexpected image ref: %q", im.ImageRef)
	}
}

func TestCmdPackageCompileWorkflowWritesPerStepRuntimeArtifacts(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src", "mywf")
	img := filepath.Join(dir, "src", "core", "assets", "images", "codex")
	runtimes := filepath.Join(dir, "src", "core", "runtimes")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(img, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(runtimes, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(runtimes, ".keep"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(img, "Dockerfile"), []byte("FROM alpine:3.20\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := `name: mywf
resolver: codex
steps:
  - id: fetch
    cmd: echo hi
    security:
      profile: sidecar-client
      network:
        mode: allowlist
        allow: [api.openai.com]
`
	if err := os.WriteFile(filepath.Join(src, "config.yml"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := cmdPackage([]string{"compile", "workflow", "--workdir", dir, "--from", src}); err != nil {
		t.Fatal(err)
	}
	tgz := filepath.Join(dir, infrastructure.DockpipeDirRel, "internal", "packages", "workflows", "dockpipe-workflow-mywf-0.0.0.tar.gz")
	rmf, err := packagebuild.ReadFileFromTarGz(tgz, "workflows/mywf/.dockpipe/steps/fetch.runtime.effective.json")
	if err != nil {
		t.Fatal(err)
	}
	var rm domain.CompiledRuntimeManifest
	if err := json.Unmarshal(rmf, &rm); err != nil {
		t.Fatal(err)
	}
	if rm.StepID != "fetch" || rm.PolicyProfile != "sidecar-client" {
		t.Fatalf("unexpected step manifest: %+v", rm)
	}
	if rm.Security.Network.Mode != "allowlist" || rm.Security.Network.Enforcement != "proxy" {
		t.Fatalf("expected proxy allowlist step policy, got %+v", rm.Security.Network)
	}
	if !rm.PolicySources.StepOverride {
		t.Fatalf("expected step override provenance, got %+v", rm.PolicySources)
	}
	imf, err := packagebuild.ReadFileFromTarGz(tgz, "workflows/mywf/.dockpipe/steps/fetch.image-artifact.json")
	if err != nil {
		t.Fatal(err)
	}
	var im domain.ImageArtifactManifest
	if err := json.Unmarshal(imf, &im); err != nil {
		t.Fatal(err)
	}
	if im.ImageKey != "fetch" || im.Source != "build" {
		t.Fatalf("unexpected step image artifact: %+v", im)
	}
}

func TestCmdPackageCompileWorkflowUsesPackageImageRegistryMetadata(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src", "mywf")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := `name: mywf
steps: []
`
	if err := os.WriteFile(filepath.Join(src, "config.yml"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	manifest := `schema: 1
name: mywf
version: 1.2.3
title: Mywf
description: d
author: a
website: https://example.com
license: Apache-2.0
kind: workflow
image:
  source: registry
  ref: ghcr.io/acme/mywf@sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef
  pull_policy: if-missing
`
	if err := os.WriteFile(filepath.Join(src, "package.yml"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := cmdPackage([]string{"compile", "workflow", "--workdir", dir, "--from", src}); err != nil {
		t.Fatal(err)
	}
	tgz := filepath.Join(dir, infrastructure.DockpipeDirRel, "internal", "packages", "workflows", "dockpipe-workflow-mywf-1.2.3.tar.gz")
	rmf, err := packagebuild.ReadFileFromTarGz(tgz, "workflows/mywf/.dockpipe/runtime.effective.json")
	if err != nil {
		t.Fatal(err)
	}
	var rm domain.CompiledRuntimeManifest
	if err := json.Unmarshal(rmf, &rm); err != nil {
		t.Fatal(err)
	}
	if rm.Image.Source != "registry" || rm.Image.Ref == "" || rm.Image.PullPolicy != "if-missing" {
		t.Fatalf("unexpected compiled image selection: %+v", rm.Image)
	}
	imf, err := packagebuild.ReadFileFromTarGz(tgz, "workflows/mywf/.dockpipe/image-artifact.json")
	if err != nil {
		t.Fatal(err)
	}
	var im domain.ImageArtifactManifest
	if err := json.Unmarshal(imf, &im); err != nil {
		t.Fatal(err)
	}
	if im.Source != "registry" || im.ArtifactState != "referenced" || im.ExpectedDigest == "" {
		t.Fatalf("unexpected registry image artifact: %+v", im)
	}
}

func TestCmdPackageCompileWorkflowStepRuntimeOverridesPackageImageRegistryMetadata(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src", "mywf")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}
	img := filepath.Join(dir, "src", "core", "assets", "images", "codex")
	if err := os.MkdirAll(img, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "src", "core", "runtimes"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(img, "Dockerfile"), []byte("FROM alpine:3.20\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := `name: mywf
steps:
  - id: custom
    runtime: codex
    cmd: echo custom
`
	if err := os.WriteFile(filepath.Join(src, "config.yml"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	manifest := `schema: 1
name: mywf
version: 1.2.3
title: Mywf
description: d
author: a
website: https://example.com
license: Apache-2.0
kind: workflow
image:
  source: registry
  ref: ghcr.io/acme/mywf@sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef
  pull_policy: if-missing
`
	if err := os.WriteFile(filepath.Join(src, "package.yml"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := cmdPackage([]string{"compile", "workflow", "--workdir", dir, "--from", src}); err != nil {
		t.Fatal(err)
	}
	tgz := filepath.Join(dir, infrastructure.DockpipeDirRel, "internal", "packages", "workflows", "dockpipe-workflow-mywf-1.2.3.tar.gz")
	rmf, err := packagebuild.ReadFileFromTarGz(tgz, "workflows/mywf/.dockpipe/steps/custom.runtime.effective.json")
	if err != nil {
		t.Fatal(err)
	}
	var rm domain.CompiledRuntimeManifest
	if err := json.Unmarshal(rmf, &rm); err != nil {
		t.Fatal(err)
	}
	if rm.Image.Source != "build" || rm.Image.PullPolicy != "" {
		t.Fatalf("expected step runtime to override package registry image, got %+v", rm.Image)
	}
	imf, err := packagebuild.ReadFileFromTarGz(tgz, "workflows/mywf/.dockpipe/steps/custom.image-artifact.json")
	if err != nil {
		t.Fatal(err)
	}
	var im domain.ImageArtifactManifest
	if err := json.Unmarshal(imf, &im); err != nil {
		t.Fatal(err)
	}
	if im.ImageKey != "custom" || im.Source != "build" || im.ImageRef == "" {
		t.Fatalf("unexpected step image artifact: %+v", im)
	}
}

func TestCmdPackageCompileWorkflowUsesWorkflowSecurityNetworkMode(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src", "mywf")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := `name: mywf
security:
  network:
    mode: offline
steps: []
`
	if err := os.WriteFile(filepath.Join(src, "config.yml"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := cmdPackage([]string{"compile", "workflow", "--workdir", dir, "--from", src}); err != nil {
		t.Fatal(err)
	}
	tgz := filepath.Join(dir, infrastructure.DockpipeDirRel, "internal", "packages", "workflows", "dockpipe-workflow-mywf-0.0.0.tar.gz")
	rmf, err := packagebuild.ReadFileFromTarGz(tgz, "workflows/mywf/.dockpipe/runtime.effective.json")
	if err != nil {
		t.Fatal(err)
	}
	var rm domain.CompiledRuntimeManifest
	if err := json.Unmarshal(rmf, &rm); err != nil {
		t.Fatal(err)
	}
	if rm.Security.Network.Mode != "offline" || rm.Security.Network.Enforcement != "native" {
		t.Fatalf("expected native offline policy, got %+v", rm.Security.Network)
	}
	if !slices.Contains(rm.RuleIDs, "network.mode.offline") {
		t.Fatalf("expected network.mode.offline rule id, got %+v", rm.RuleIDs)
	}
}

func TestCmdPackageCompileWorkflowPreservesAllowlistRules(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src", "mywf")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := `name: mywf
security:
  network:
    mode: allowlist
    allow:
      - api.openai.com
      - "*.anthropic.com"
    block:
      - "*.facebook.com"
steps: []
`
	if err := os.WriteFile(filepath.Join(src, "config.yml"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := cmdPackage([]string{"compile", "workflow", "--workdir", dir, "--from", src}); err != nil {
		t.Fatal(err)
	}
	tgz := filepath.Join(dir, infrastructure.DockpipeDirRel, "internal", "packages", "workflows", "dockpipe-workflow-mywf-0.0.0.tar.gz")
	rmf, err := packagebuild.ReadFileFromTarGz(tgz, "workflows/mywf/.dockpipe/runtime.effective.json")
	if err != nil {
		t.Fatal(err)
	}
	var rm domain.CompiledRuntimeManifest
	if err := json.Unmarshal(rmf, &rm); err != nil {
		t.Fatal(err)
	}
	if rm.Security.Network.Mode != "allowlist" || rm.Security.Network.Enforcement != "advisory" {
		t.Fatalf("expected advisory allowlist policy, got %+v", rm.Security.Network)
	}
	if !slices.Equal(rm.Security.Network.Allow, []string{"api.openai.com", "*.anthropic.com"}) {
		t.Fatalf("unexpected allowlist rules: %+v", rm.Security.Network.Allow)
	}
	if !slices.Equal(rm.Security.Network.Block, []string{"*.facebook.com"}) {
		t.Fatalf("unexpected block rules: %+v", rm.Security.Network.Block)
	}
}

func TestCmdPackageCompileWorkflowSupportsProxyNetworkEnforcement(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src", "mywf")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := `name: mywf
security:
  profile: sidecar-client
  network:
    mode: restricted
steps: []
`
	if err := os.WriteFile(filepath.Join(src, "config.yml"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := cmdPackage([]string{"compile", "workflow", "--workdir", dir, "--from", src}); err != nil {
		t.Fatal(err)
	}
	tgz := filepath.Join(dir, infrastructure.DockpipeDirRel, "internal", "packages", "workflows", "dockpipe-workflow-mywf-0.0.0.tar.gz")
	rmf, err := packagebuild.ReadFileFromTarGz(tgz, "workflows/mywf/.dockpipe/runtime.effective.json")
	if err != nil {
		t.Fatal(err)
	}
	var rm domain.CompiledRuntimeManifest
	if err := json.Unmarshal(rmf, &rm); err != nil {
		t.Fatal(err)
	}
	if rm.Security.Network.Mode != "restricted" || rm.Security.Network.Enforcement != "proxy" {
		t.Fatalf("expected proxy restricted policy, got %+v", rm.Security.Network)
	}
	if rm.PolicyProfile != "sidecar-client" {
		t.Fatalf("expected sidecar-client policy profile, got %+v", rm)
	}
	if !slices.Contains(rm.EnforcementSummaries, "network policy requires a proxy-backed egress layer when this workflow runs") {
		t.Fatalf("unexpected enforcement summaries: %+v", rm.EnforcementSummaries)
	}
}
