package infrastructure

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"
)

// dockerPreflightHook, if non-nil, replaces the default Docker check (tests).
var dockerPreflightHook func(*os.File) error

var dockerPreflightOnce sync.Once
var dockerPreflightErr error

// EnsureDockerReachable runs a one-time lightweight check before the first container use in this process.
// It is skipped when DOCKPIPE_SKIP_DOCKER_PREFLIGHT=1 (application tests).
func EnsureDockerReachable(stderr *os.File) error {
	dockerPreflightOnce.Do(func() {
		dockerPreflightErr = checkDockerReachable(stderr, false)
	})
	return dockerPreflightErr
}

// DockerDoctorCheck runs the same logic as EnsureDockerReachable but not memoized (for dockpipe doctor).
// It ignores DOCKPIPE_SKIP_DOCKER_PREFLIGHT so diagnostics always exercise Docker.
func DockerDoctorCheck(stderr *os.File) error {
	return checkDockerReachable(stderr, true)
}

func checkDockerReachable(stderr *os.File, ignoreSkipEnv bool) error {
	if dockerPreflightHook != nil {
		return dockerPreflightHook(stderr)
	}
	if !ignoreSkipEnv && os.Getenv("DOCKPIPE_SKIP_DOCKER_PREFLIGHT") == "1" {
		return nil
	}
	if _, err := exec.LookPath("docker"); err != nil {
		return fmt.Errorf(
			"docker not found in PATH — install the Docker CLI and ensure the daemon is running\n" +
				"  https://docs.docker.com/get-docker/",
		)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	cmd := execCommandContextFn(ctx, "docker", "version")
	out, err := cmd.CombinedOutput()
	msg := strings.TrimSpace(string(out))
	if err == nil {
		return nil
	}

	var hint strings.Builder
	hint.WriteString("docker is not reachable")
	if ctx.Err() == context.DeadlineExceeded {
		hint.WriteString(" (timed out after 15s talking to the daemon)")
	}
	hint.WriteString(".\n")
	if msg != "" {
		hint.WriteString(msg)
		hint.WriteString("\n")
	}
	hint.WriteString(dockerDaemonHints())
	return fmt.Errorf("%s", strings.TrimSpace(hint.String()))
}

// execCommandContextFn is exec.CommandContext for tests.
var execCommandContextFn = func(ctx context.Context, name string, arg ...string) *exec.Cmd {
	return exec.CommandContext(ctx, name, arg...)
}

func dockerDaemonHints() string {
	var b strings.Builder
	switch runtime.GOOS {
	case "darwin":
		b.WriteString("  • Start Docker Desktop (whale icon in the menu bar).\n")
	case "windows":
		b.WriteString("  • Start Docker Desktop. For WSL2: enable Docker Desktop → Settings → Resources → WSL integration.\n")
	default:
		b.WriteString("  • Linux: try `sudo systemctl start docker` (or your distro’s service name).\n")
		b.WriteString("  • Add your user to the `docker` group if you see permission errors on the socket: `sudo usermod -aG docker $USER` (log out/in).\n")
	}
	b.WriteString("  • Docs: https://docs.docker.com/config/daemon/start/\n")
	return b.String()
}
