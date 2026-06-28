package application

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dockpipe/src/lib/domain"
	"dockpipe/src/lib/infrastructure"
	"dockpipe/src/lib/infrastructure/packagebuild"
)

func TestPrebuildCompiledImageArtifactsBuildsPlannedImage(t *testing.T) {
	dir := t.TempDir()
	buildDir := filepath.Join(dir, "src", "core", "assets", "images", "codex")
	if err := os.MkdirAll(buildDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(buildDir, "Dockerfile"), []byte("FROM alpine:3.20\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	artifact, err := buildImageArtifactManifest(dir, "mywf", "mywf", "codex", "dockpipe-codex:test", buildDir, dir, "sha256:policy", domain.ImageArtifactProvenance{
		Resolver:        "codex",
		PackageVersion:  "1.2.3",
		DockpipeVersion: "1.2.3",
	})
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

	oldExists := dockerImageExistsAppFn
	oldBuild := dockerBuildAppFn
	t.Cleanup(func() {
		dockerImageExistsAppFn = oldExists
		dockerBuildAppFn = oldBuild
	})
	dockerImageExistsAppFn = func(image string) (bool, error) {
		if image != "dockpipe-codex:test" {
			t.Fatalf("unexpected image exists check %q", image)
		}
		return false, nil
	}
	var builtImage, builtDir, builtCtx string
	dockerBuildAppFn = func(image, dockerfileDir, contextDir string) error {
		builtImage, builtDir, builtCtx = image, dockerfileDir, contextDir
		return nil
	}

	n, err := prebuildCompiledImageArtifacts(dir)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("built count got %d want 1", n)
	}
	if builtImage != "dockpipe-codex:test" || builtDir != buildDir || builtCtx != dir {
		t.Fatalf("unexpected docker build args image=%q dir=%q ctx=%q", builtImage, builtDir, builtCtx)
	}
	index := filepath.Join(dir, infrastructure.DockpipeDirRel, "internal", "images", "by-fingerprint")
	matches, err := filepath.Glob(filepath.Join(index, "*.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 1 {
		t.Fatalf("expected one image index record under %s, got %d", index, len(matches))
	}
}

func TestPrebuildCompiledImageArtifactsBuildsPackagedGeneratedImage(t *testing.T) {
	dir := t.TempDir()
	stage := filepath.Join(dir, "stage")
	buildDir := filepath.Join(stage, domain.RuntimeManifestDirName, "images", "claude")
	if err := os.MkdirAll(buildDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(buildDir, "Dockerfile"), []byte("FROM alpine:3.20\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	artifact, err := buildImageArtifactManifest(stage, "claude", "claude", "claude", "dockpipe-claude:test", buildDir, stage, "sha256:policy", domain.ImageArtifactProvenance{
		Resolver:        "claude",
		PackageVersion:  "1.2.3",
		DockpipeVersion: "1.2.3",
	})
	if err != nil {
		t.Fatal(err)
	}
	manifestDir := filepath.Join(stage, domain.RuntimeManifestDirName, domain.StepArtifactsDirName)
	if err := os.MkdirAll(manifestDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := writeJSONFile(filepath.Join(manifestDir, "claude.image-artifact.json"), artifact); err != nil {
		t.Fatal(err)
	}
	pkgDir, err := infrastructure.PackagesWorkflowsDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(pkgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	tgz := filepath.Join(pkgDir, "dockpipe-workflow-claude-1.2.3.tar.gz")
	if _, err := packagebuild.WriteDirTarGzWithPrefix(stage, tgz, "workflows/claude"); err != nil {
		t.Fatal(err)
	}

	oldExists := dockerImageExistsAppFn
	oldBuild := dockerBuildAppFn
	t.Cleanup(func() {
		dockerImageExistsAppFn = oldExists
		dockerBuildAppFn = oldBuild
	})
	dockerImageExistsAppFn = func(image string) (bool, error) {
		if image != "dockpipe-claude:test" {
			t.Fatalf("unexpected image exists check %q", image)
		}
		return false, nil
	}
	var builtImage, builtDir, builtCtx string
	dockerBuildAppFn = func(image, dockerfileDir, contextDir string) error {
		builtImage, builtDir, builtCtx = image, dockerfileDir, contextDir
		return nil
	}

	n, err := prebuildCompiledImageArtifacts(dir)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("built count got %d want 1", n)
	}
	if builtImage != "dockpipe-claude:test" {
		t.Fatalf("unexpected docker build image %q", builtImage)
	}
	if !strings.HasSuffix(filepath.ToSlash(builtDir), "/workflows/claude/.dockpipe/images/claude") {
		t.Fatalf("dockerfile dir %q was not resolved from extracted workflow package", builtDir)
	}
	if !strings.HasSuffix(filepath.ToSlash(builtCtx), "/workflows/claude") {
		t.Fatalf("context dir %q was not resolved from extracted workflow package", builtCtx)
	}
	if _, err := os.Stat(filepath.Join(builtDir, "Dockerfile")); err != nil {
		t.Fatalf("resolved dockerfile missing: %v", err)
	}
}

func TestPrebuildCompiledImageArtifactsSkipsExistingImage(t *testing.T) {
	dir := t.TempDir()
	buildDir := filepath.Join(dir, "src", "core", "assets", "images", "codex")
	if err := os.MkdirAll(buildDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(buildDir, "Dockerfile"), []byte("FROM alpine:3.20\n"), 0o644); err != nil {
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

	oldExists := dockerImageExistsAppFn
	oldBuild := dockerBuildAppFn
	t.Cleanup(func() {
		dockerImageExistsAppFn = oldExists
		dockerBuildAppFn = oldBuild
	})
	dockerImageExistsAppFn = func(string) (bool, error) { return true, nil }
	dockerBuildAppFn = func(string, string, string) error {
		t.Fatal("docker build should not be called for an existing image")
		return nil
	}

	n, err := prebuildCompiledImageArtifacts(dir)
	if err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("built count got %d want 0", n)
	}
	index := filepath.Join(dir, infrastructure.DockpipeDirRel, "internal", "images", "by-fingerprint")
	matches, err := filepath.Glob(filepath.Join(index, "*.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 1 {
		t.Fatalf("expected one reused image index record under %s, got %d", index, len(matches))
	}
}
