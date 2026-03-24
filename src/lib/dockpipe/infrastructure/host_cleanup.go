package infrastructure

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Host cleanup after skip_container host scripts (RunHostScript):
// - workdir/.dockpipe/cleanup/docker-* — each file is one line: a Docker container name to stop
// - legacy: workdir/.dockpipe/cursor-dev/session_container (same one-line name)
//
// Templates register resources by writing those files; the Go runner applies cleanup when the bash
// child exits (trap, normal return, or defer if the process died without removing markers).

const (
	hostCleanupDirRel         = ".dockpipe/cleanup"
	cursorDevSessionLegacyRel = ".dockpipe/cursor-dev/session_container"
)

func envGet(env []string, key string) string {
	prefix := key + "="
	for _, e := range env {
		if strings.HasPrefix(e, prefix) {
			return strings.TrimPrefix(e, prefix)
		}
	}
	return ""
}

func hostCleanupSkip(env []string) bool {
	v := strings.TrimSpace(strings.ToLower(envGet(env, "DOCKPIPE_SKIP_HOST_CLEANUP")))
	return v == "1" || v == "true" || v == "yes"
}

// ApplyHostCleanup stops Docker containers registered under workdir/.dockpipe/cleanup/docker-*
// and removes legacy cursor-dev session_container when still present.
// Call from RunHostScript defer so dockpipe tears down host-started resources the script tracked.
func ApplyHostCleanup(env []string) {
	if hostCleanupSkip(env) {
		return
	}
	wd := strings.TrimSpace(envGet(env, "DOCKPIPE_WORKDIR"))
	if wd == "" {
		return
	}
	wdAbs, err := absHostWorkdir(wd)
	if err != nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	dir := filepath.Join(wdAbs, hostCleanupDirRel)
	ents, err := os.ReadDir(dir)
	if err == nil {
		for _, e := range ents {
			if e.IsDir() {
				continue
			}
			name := e.Name()
			if !strings.HasPrefix(name, "docker-") {
				continue
			}
			p := filepath.Join(dir, name)
			b, err := os.ReadFile(p)
			if err != nil {
				continue
			}
			cn := strings.TrimSpace(string(b))
			if stopped := tryStopDockerAndRemoveMarker(ctx, cn, p); stopped {
				fmt.Fprintf(os.Stderr, "[dockpipe] host cleanup: stopped Docker container %s\n", cn)
			}
		}
	}

	// Legacy single-file marker (cursor-dev and older templates).
	leg := filepath.Join(wdAbs, cursorDevSessionLegacyRel)
	b, err := os.ReadFile(leg)
	if err != nil {
		return
	}
	cn := strings.TrimSpace(string(b))
	if stopped := tryStopDockerAndRemoveMarker(ctx, cn, leg); stopped {
		fmt.Fprintf(os.Stderr, "[dockpipe] host cleanup: stopped Docker container %s (legacy marker)\n", cn)
	}
}

func tryStopDockerAndRemoveMarker(ctx context.Context, name, markerPath string) bool {
	if name == "" || !isSafeDockerContainerName(name) {
		_ = os.Remove(markerPath)
		return false
	}
	run := func(args ...string) error {
		cmd := exec.CommandContext(ctx, "docker", args...)
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}
	if err := run("stop", "-t", "10", name); err == nil {
		_ = os.Remove(markerPath)
		return true
	}
	if err := run("inspect", name); err != nil {
		_ = os.Remove(markerPath)
		return false
	}
	_ = run("kill", name)
	_ = run("stop", "-t", "2", name)
	if err := run("inspect", name); err != nil {
		_ = os.Remove(markerPath)
		return true
	}
	fmt.Fprintf(os.Stderr, "[dockpipe] warning: host cleanup could not stop container %q (try: docker stop %s)\n", name, name)
	_ = os.Remove(markerPath)
	return false
}

func isSafeDockerContainerName(s string) bool {
	if s == "" || len(s) > 253 {
		return false
	}
	for i, r := range s {
		if i == 0 {
			if !isDockerNameCharFirst(r) {
				return false
			}
			continue
		}
		if isDockerNameCharFirst(r) || r == '.' || r == '-' {
			continue
		}
		return false
	}
	return true
}

func isDockerNameCharFirst(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_'
}
