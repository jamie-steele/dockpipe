package infrastructure

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestDockerDoctorCheckUsesHook(t *testing.T) {
	old := dockerPreflightHook
	defer func() { dockerPreflightHook = old }()

	dockerPreflightHook = func(_ *os.File) error { return nil }
	t.Setenv("DOCKPIPE_SKIP_DOCKER_PREFLIGHT", "")
	if err := DockerDoctorCheck(nil); err != nil {
		t.Fatalf("hook: %v", err)
	}
}

func TestDockerDoctorCheckUsesConfiguredDockerBinary(t *testing.T) {
	oldHook := dockerPreflightHook
	oldExec := execCommandContextFn
	t.Cleanup(func() {
		dockerPreflightHook = oldHook
		execCommandContextFn = oldExec
	})
	dockerPreflightHook = nil
	fakeDocker := filepath.Join(t.TempDir(), "docker-test")
	if err := os.WriteFile(fakeDocker, []byte(""), 0o755); err != nil {
		t.Fatalf("write fake docker: %v", err)
	}
	t.Setenv("DOCKPIPE_DOCKER_BIN", fakeDocker)
	var gotName string
	execCommandContextFn = func(ctx context.Context, name string, arg ...string) *exec.Cmd {
		gotName = name
		return helperExitCommand(0)
	}
	if err := DockerDoctorCheck(nil); err != nil {
		t.Fatalf("DockerDoctorCheck: %v", err)
	}
	if gotName != fakeDocker {
		t.Fatalf("expected configured docker binary %q, got %q", fakeDocker, gotName)
	}
}
