package infrastructure

import (
	"os"
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
