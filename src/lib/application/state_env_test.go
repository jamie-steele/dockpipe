package application

import (
	"path/filepath"
	"testing"

	"dockpipe/src/lib/infrastructure"
)

func TestApplyDockpipeStateEnv(t *testing.T) {
	wd := t.TempDir()
	envMap := map[string]string{}
	if err := applyDockpipeStateEnv(envMap, wd, "Pipeon Dev/Stack"); err != nil {
		t.Fatal(err)
	}
	if got, want := envMap[infrastructure.EnvStateDir], filepath.Join(wd, "bin", ".dockpipe"); got != want {
		t.Fatalf("state dir = %q want %q", got, want)
	}
	if got, want := envMap[infrastructure.EnvPackageID], "pipeon-dev-stack"; got != want {
		t.Fatalf("package id = %q want %q", got, want)
	}
	if got, want := envMap[infrastructure.EnvPackageStateDir], filepath.Join(wd, "bin", ".dockpipe", "packages", "pipeon-dev-stack"); got != want {
		t.Fatalf("package state dir = %q want %q", got, want)
	}
}

func TestApplyCIArtifactEnvWorkflow(t *testing.T) {
	wd := t.TempDir()
	envMap := map[string]string{"DOCKPIPE_WORKFLOW_NAME": "docs.orchestrate"}
	if err := applyCIArtifactEnv(envMap, wd); err != nil {
		t.Fatal(err)
	}
	if got, want := envMap["DOCKPIPE_CI_RAW_DIR"], filepath.Join(wd, "bin", ".dockpipe", "workflows", "docs.orchestrate", "artifacts", "ci-raw"); got != want {
		t.Fatalf("raw dir = %q want %q", got, want)
	}
	if got, want := envMap["DOCKPIPE_CI_ANALYSIS_DIR"], filepath.Join(wd, "bin", ".dockpipe", "workflows", "docs.orchestrate", "artifacts", "ci-analysis"); got != want {
		t.Fatalf("analysis dir = %q want %q", got, want)
	}
}

func TestApplyCIArtifactEnvPackageDefaultAndPreserveExplicit(t *testing.T) {
	wd := t.TempDir()
	envMap := map[string]string{"DOCKPIPE_CI_ANALYSIS_DIR": "/custom/analysis"}
	if err := applyCIArtifactEnv(envMap, wd); err != nil {
		t.Fatal(err)
	}
	if got, want := envMap["DOCKPIPE_CI_RAW_DIR"], filepath.Join(wd, "bin", ".dockpipe", "packages", "dorkpipe", "ci", "raw"); got != want {
		t.Fatalf("raw dir = %q want %q", got, want)
	}
	if got := envMap["DOCKPIPE_CI_ANALYSIS_DIR"]; got != "/custom/analysis" {
		t.Fatalf("analysis dir should preserve explicit value, got %q", got)
	}
}

func TestApplyWorkflowArtifactEnv(t *testing.T) {
	wd := t.TempDir()
	envMap := map[string]string{}
	if err := applyWorkflowArtifactEnv(envMap, wd, "CI/Test"); err != nil {
		t.Fatal(err)
	}
	if got, want := envMap["DOCKPIPE_SOURCE_ROOT"], wd; got != want {
		t.Fatalf("source root = %q want %q", got, want)
	}
	if got, want := envMap["DOCKPIPE_ARTIFACT_ROOT"], filepath.Join(wd, "bin", ".dockpipe", "workflows", "CI-Test", "artifacts"); got != want {
		t.Fatalf("artifact root = %q want %q", got, want)
	}
}
