package application

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

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

	if err := cmdPackage([]string{"compile", "core", "--workdir", dir, "--from", src}); err != nil {
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

func TestCmdPackageCompileCoreRunsSourceBuildScript(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src", "core")
	if err := os.MkdirAll(filepath.Join(src, "runtimes"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(src, "assets"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "runtimes", ".keep"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	manifestScript := "assets/build-source.sh"
	scriptPath := filepath.Join(src, "assets", "build-source.sh")
	scriptMode := os.FileMode(0o755)
	scriptBody := "#!/usr/bin/env bash\nset -e\necho built > assets/generated.txt\n"
	if runtime.GOOS == "windows" {
		manifestScript = "assets/build-source.cmd"
		scriptPath = filepath.Join(src, "assets", "build-source.cmd")
		scriptMode = 0o644
		scriptBody = "@echo off\r\necho built>assets\\generated.txt\r\n"
	}
	manifest := "schema: 1\nname: dockpipe.core\nkind: core\nbuild:\n  source:\n    script: " + manifestScript + "\n"
	if err := os.WriteFile(filepath.Join(src, "package.yml"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(scriptPath, []byte(scriptBody), scriptMode); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "VERSION"), []byte("1.2.3\n"), 0o644); err != nil {
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

	if err := cmdPackage([]string{"compile", "core", "--workdir", dir, "--from", src}); err != nil {
		t.Fatal(err)
	}
	coreDir := filepath.Join(dir, infrastructure.DockpipeDirRel, "internal", "packages", "core")
	matches, err := filepath.Glob(filepath.Join(coreDir, "dockpipe-core-*.tar.gz"))
	if err != nil || len(matches) != 1 {
		t.Fatalf("expected one core tarball under %s: matches=%v err=%v", coreDir, matches, err)
	}
	generated, err := packagebuild.ReadFileFromTarGz(matches[0], "core/assets/generated.txt")
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(generated)) != "built" {
		t.Fatalf("expected generated asset from build.source.script, got %q", string(generated))
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

	if err := cmdPackage([]string{"compile", "resolvers", "--workdir", dir, "--from", pack}); err != nil {
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

func TestCmdPackageCompileResolversEmitsOperationResults(t *testing.T) {
	dir := t.TempDir()
	pack := filepath.Join(dir, "vendor")
	resRoot := filepath.Join(pack, "resolvers", "alpha")
	if err := os.MkdirAll(resRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(resRoot, "profile"), []byte("DOCKPIPE_RESOLVER_CMD=test\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	stderr, err := captureResultStderr(t, func() error {
		return cmdPackage([]string{"compile", "resolvers", "--workdir", dir, "--from", pack})
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"unit=package.compile.resolvers",
		"unit=package.compile.resolver",
		"package=alpha",
		"result=compiled",
		"status=done",
	} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("expected resolver compile stderr to contain %q, got:\n%s", want, stderr)
		}
	}
}

func TestCmdPackageCompileResolverMaterializesAuthoredAptPackages(t *testing.T) {
	dir := t.TempDir()
	pack := filepath.Join(dir, "my-vendor")
	resRoot := filepath.Join(pack, "resolvers", "codex")
	img := filepath.Join(dir, "src", "core", "assets", "images", "codex")
	if err := os.MkdirAll(resRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(img, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "src", "core", "runtimes"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(img, "Dockerfile"), []byte("FROM debian:bookworm-slim\nUSER node\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(resRoot, "profile"), []byte("DOCKPIPE_RESOLVER_WORKFLOW=codex\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := `name: codex
image:
  packages:
    apt:
      - python3
      - golang-go
steps:
  - id: codex
    isolate: codex
`
	if err := os.WriteFile(filepath.Join(resRoot, "config.yml"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := cmdPackage([]string{"compile", "resolvers", "--workdir", dir, "--from", pack, "--force"}); err != nil {
		t.Fatal(err)
	}
	tgz := filepath.Join(dir, infrastructure.DockpipeDirRel, "internal", "packages", "resolvers", "dockpipe-resolver-codex-0.0.0.tar.gz")
	imf, err := packagebuild.ReadFileFromTarGz(tgz, "resolvers/codex/.dockpipe/steps/codex.image-artifact.json")
	if err != nil {
		t.Fatal(err)
	}
	var im domain.ImageArtifactManifest
	if err := json.Unmarshal(imf, &im); err != nil {
		t.Fatal(err)
	}
	if im.Source != "build" || !strings.Contains(im.ImageRef, "-tools:") || im.Build == nil {
		t.Fatalf("expected derived tools image artifact, got %+v", im)
	}
	df, err := packagebuild.ReadFileFromTarGz(tgz, filepath.ToSlash(filepath.Join("resolvers/codex", im.Build.Dockerfile)))
	if err != nil {
		t.Fatal(err)
	}
	gotDockerfile := string(df)
	if !strings.Contains(gotDockerfile, "--mount=type=cache,target=/var/cache/apt") ||
		!strings.Contains(gotDockerfile, "apt-get install -y --no-install-recommends golang-go python3") {
		t.Fatalf("generated Dockerfile missing apt install:\n%s", string(df))
	}
	if strings.Contains(gotDockerfile, "RUN npm install") && strings.Index(gotDockerfile, "apt-get install") > strings.Index(gotDockerfile, "RUN npm install") {
		t.Fatalf("expected workflow-authored apt install before npm install:\n%s", gotDockerfile)
	}
	if strings.Index(gotDockerfile, "apt-get install") > strings.LastIndex(gotDockerfile, "USER node") {
		t.Fatalf("expected workflow-authored apt install before final USER:\n%s", gotDockerfile)
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

func TestCmdPackageCompileWorkflowEmitsOperationResults(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src", "mywf")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "config.yml"), []byte("name: mywf\nsteps: []\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	stderr, err := captureResultStderr(t, func() error {
		return cmdPackage([]string{"compile", "workflow", "--workdir", dir, "--from", src})
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"unit=package.compile.workflow",
		"status=start",
		"status=done",
		"package=mywf",
		"result=compiled",
		"output=",
	} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("expected workflow compile stderr to contain %q, got:\n%s", want, stderr)
		}
	}
}

func TestCmdPackageCompileWorkflowRunsCompileHooksInStaging(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src", "mywf")
	if err := os.MkdirAll(filepath.Join(src, "assets"), 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := `name: mywf
compile_hooks:
  - printf staged > assets/generated.txt
steps: []
`
	if err := os.WriteFile(filepath.Join(src, "config.yml"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := cmdPackage([]string{"compile", "workflow", "--workdir", dir, "--from", src}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(src, "assets", "generated.txt")); !os.IsNotExist(err) {
		t.Fatalf("expected compile hook output to stay out of source tree, got %v", err)
	}
	tgz := filepath.Join(dir, infrastructure.DockpipeDirRel, "internal", "packages", "workflows", "dockpipe-workflow-mywf-0.0.0.tar.gz")
	got, err := packagebuild.ReadFileFromTarGz(tgz, "workflows/mywf/assets/generated.txt")
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(got)) != "staged" {
		t.Fatalf("expected staged compile hook output, got %q", string(got))
	}
}

func TestCmdPackageCompileWorkflowRebuildsInvalidStoreTarball(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src", "mywf")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "VERSION"), []byte("1.0.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := "name: mywf\nsteps:\n  - kind: host\n    cmd: echo ok\n"
	if err := os.WriteFile(filepath.Join(src, "config.yml"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	pkgDir, err := infrastructure.PackagesWorkflowsDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	oldStage := filepath.Join(dir, "old")
	if err := os.MkdirAll(oldStage, 0o755); err != nil {
		t.Fatal(err)
	}
	oldCfg := "name: mywf\nsteps:\n  - skip_container: true\n    cmd: echo old\n"
	if err := os.WriteFile(filepath.Join(oldStage, "config.yml"), []byte(oldCfg), 0o644); err != nil {
		t.Fatal(err)
	}
	tgz := filepath.Join(pkgDir, "dockpipe-workflow-mywf-1.0.0.tar.gz")
	if _, err := packagebuild.WriteDirTarGzWithPrefix(oldStage, tgz, "workflows/mywf"); err != nil {
		t.Fatal(err)
	}
	future := time.Now().Add(time.Hour)
	if err := os.Chtimes(tgz, future, future); err != nil {
		t.Fatal(err)
	}
	if err := cmdPackage([]string{"compile", "workflow", "--workdir", dir, "--from", src}); err != nil {
		t.Fatal(err)
	}
	got, err := packagebuild.ReadFileFromTarGz(tgz, "workflows/mywf/config.yml")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(got), "skip_container") || !strings.Contains(string(got), "kind: host") {
		t.Fatalf("expected rebuilt workflow config, got:\n%s", string(got))
	}
}

func TestCmdPackageCompileCoreSkipEmitsOperationResults(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src", "core")
	if err := os.MkdirAll(filepath.Join(src, "runtimes"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "runtimes", ".keep"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := cmdPackage([]string{"compile", "core", "--workdir", dir, "--from", src}); err != nil {
		t.Fatal(err)
	}
	stderr, err := captureResultStderr(t, func() error {
		return cmdPackage([]string{"compile", "core", "--workdir", dir, "--from", src})
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"unit=package.compile.core",
		"status=start",
		"status=done",
		"result=skip",
		"skip_reason=existing_tarball",
	} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("expected core compile stderr to contain %q, got:\n%s", want, stderr)
		}
	}
}

func TestCmdPackageCompileWorkflowsBatchPrunesStaleTarballs(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, "workflows")
	current := filepath.Join(root, "current")
	if err := os.MkdirAll(current, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(current, "config.yml"), []byte("name: current\nsteps: []\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	staleStage := filepath.Join(dir, "stale-stage")
	if err := os.MkdirAll(staleStage, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(staleStage, "config.yml"), []byte("name: stale\nsteps: []\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(dir, infrastructure.DockpipeDirRel, "internal", "packages", "workflows")
	if err := os.MkdirAll(dest, 0o755); err != nil {
		t.Fatal(err)
	}
	staleTar := filepath.Join(dest, "dockpipe-workflow-stale-1.0.0.tar.gz")
	if _, err := packagebuild.WriteDirTarGzWithPrefix(staleStage, staleTar, "workflows/stale"); err != nil {
		t.Fatal(err)
	}

	if err := cmdPackageCompileWorkflowsBatch([]string{"--workdir", dir, "--from", root, "--force", "--prune-stale"}); err != nil {
		t.Fatalf("compile workflows: %v", err)
	}
	if _, err := os.Stat(staleTar); !os.IsNotExist(err) {
		t.Fatalf("stale tarball still exists or stat failed unexpectedly: %v", err)
	}
	currentGlob := filepath.Join(dest, "dockpipe-workflow-current-*.tar.gz")
	matches, err := filepath.Glob(currentGlob)
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 1 {
		t.Fatalf("expected current workflow tarball at %s, got %v", currentGlob, matches)
	}
}

func TestCompileSingleResolverRebuildsInvalidStoreTarball(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src", "resolvers", "alpha")
	dest, err := infrastructure.PackagesResolversDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "profile"), []byte("DOCKPIPE_RESOLVER_IMAGE=alpine\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := "name: alpha\nsteps:\n  - kind: host\n    cmd: echo ok\n"
	if err := os.WriteFile(filepath.Join(src, "config.yml"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	oldStage := filepath.Join(dir, "old-resolver")
	if err := os.MkdirAll(oldStage, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(oldStage, "profile"), []byte("DOCKPIPE_RESOLVER_IMAGE=alpine\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	oldCfg := "name: alpha\nsteps:\n  - skip_container: true\n    cmd: echo old\n"
	if err := os.WriteFile(filepath.Join(oldStage, "config.yml"), []byte(oldCfg), 0o644); err != nil {
		t.Fatal(err)
	}
	tgz := filepath.Join(dest, "dockpipe-resolver-alpha-1.0.0.tar.gz")
	if _, err := packagebuild.WriteDirTarGzWithPrefix(oldStage, tgz, "resolvers/alpha"); err != nil {
		t.Fatal(err)
	}
	future := time.Now().Add(time.Hour)
	if err := os.Chtimes(tgz, future, future); err != nil {
		t.Fatal(err)
	}
	if err := compileSingleResolverDir(dir, dest, src, "alpha", "acme", "1.0.0", false); err != nil {
		t.Fatal(err)
	}
	got, err := packagebuild.ReadFileFromTarGz(tgz, "resolvers/alpha/config.yml")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(got), "skip_container") || !strings.Contains(string(got), "kind: host") {
		t.Fatalf("expected rebuilt resolver config, got:\n%s", string(got))
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

func TestCmdPackageCompileWorkflowMaterializesAuthoredAptPackages(t *testing.T) {
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
	if err := os.WriteFile(filepath.Join(img, "Dockerfile"), []byte("FROM debian:bookworm-slim\nUSER node\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := `name: mywf
isolate: codex
image:
  packages:
    apt:
      - golang-go
      - cargo
steps: []
`
	if err := os.WriteFile(filepath.Join(src, "config.yml"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := cmdPackage([]string{"compile", "workflow", "--workdir", dir, "--from", src}); err != nil {
		t.Fatal(err)
	}
	tgz := filepath.Join(dir, infrastructure.DockpipeDirRel, "internal", "packages", "workflows", "dockpipe-workflow-mywf-0.0.0.tar.gz")
	imf, err := packagebuild.ReadFileFromTarGz(tgz, "workflows/mywf/.dockpipe/image-artifact.json")
	if err != nil {
		t.Fatal(err)
	}
	var im domain.ImageArtifactManifest
	if err := json.Unmarshal(imf, &im); err != nil {
		t.Fatal(err)
	}
	if im.Source != "build" || im.Build == nil {
		t.Fatalf("expected build image artifact, got %+v", im)
	}
	if !strings.HasPrefix(im.ImageRef, "dockpipe-dockpipe-codex-") || !strings.Contains(im.ImageRef, "-tools:") {
		t.Fatalf("expected derived tools image ref, got %q", im.ImageRef)
	}
	df, err := packagebuild.ReadFileFromTarGz(tgz, filepath.ToSlash(filepath.Join("workflows/mywf", im.Build.Dockerfile)))
	if err != nil {
		t.Fatal(err)
	}
	got := string(df)
	if !strings.Contains(got, "--mount=type=cache,target=/var/cache/apt") ||
		!strings.Contains(got, "apt-get install -y --no-install-recommends cargo golang-go") {
		t.Fatalf("generated Dockerfile missing apt install:\n%s", got)
	}
	if strings.Index(got, "apt-get install") > strings.LastIndex(got, "USER node") {
		t.Fatalf("expected workflow-authored apt install before final USER:\n%s", got)
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
