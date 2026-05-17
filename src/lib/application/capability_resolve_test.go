package application

import (
	"os"
	"path/filepath"
	"testing"

	"dockpipe/src/lib/domain"
)

func TestWorkflowNeedsDockerReachableResolved_VMHostDelegateSkipsDockerPreflight(t *testing.T) {
	repo := t.TempDir()
	profileDir := filepath.Join(repo, "src", "core", "runtimes", "vmimage")
	if err := os.MkdirAll(profileDir, 0o755); err != nil {
		t.Fatal(err)
	}
	profile := "DOCKPIPE_RUNTIME_SUBSTRATE=vmimage\nDOCKPIPE_RUNTIME_HOST_SCRIPT=scripts/core.assets.scripts.vmimage-run.sh\nDOCKPIPE_RUNTIME_HOST_REQUIRES_DOCKER=0\n"
	if err := os.WriteFile(filepath.Join(profileDir, "profile"), []byte(profile), 0o644); err != nil {
		t.Fatal(err)
	}
	wf := &domain.Workflow{
		Runtime: "vmimage",
		Steps: []domain.Step{{
			Cmd: "echo hi",
		}},
	}
	if WorkflowNeedsDockerReachableResolved(wf, repo, repo) {
		t.Fatal("expected vmimage host-delegate runtime to skip docker preflight")
	}
}

func TestWorkflowNeedsDockerReachableResolved_DockerimageStillNeedsDocker(t *testing.T) {
	repo := t.TempDir()
	profileDir := filepath.Join(repo, "src", "core", "runtimes", "dockerimage")
	if err := os.MkdirAll(profileDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(profileDir, "profile"), []byte("DOCKPIPE_RUNTIME_SUBSTRATE=dockerimage\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	wf := &domain.Workflow{
		Runtime: "dockerimage",
		Steps: []domain.Step{{
			Cmd: "echo hi",
		}},
	}
	if !WorkflowNeedsDockerReachableResolved(wf, repo, repo) {
		t.Fatal("expected dockerimage runtime to keep docker preflight")
	}
}
