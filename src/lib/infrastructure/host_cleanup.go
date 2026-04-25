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

// Host cleanup after kind: host scripts (RunHostScript):
//
// When DOCKPIPE_RUN_ID is set (normal host run with a workdir), cleanup is run-scoped only:
// workdir/bin/.dockpipe/runs/<runID>.container holds the Docker container name for this invocation.
// Matching entries under workdir/bin/.dockpipe/cleanup/docker-* and the legacy session_container file
// are removed without stopping other containers.
//
// When DOCKPIPE_RUN_ID is empty (no host run registry), legacy mode scans workdir/bin/.dockpipe/cleanup/docker-*
// and workdir/bin/.dockpipe/cursor-dev/session_container — each file is one line: a container name to stop.
//
// Templates register resources by writing those files; the Go runner applies cleanup when the bash
// child exits (trap, normal return, or defer if the process died without removing markers).

var (
	hostCleanupDirRel         = filepath.Join(DockpipeDirRel, "cleanup")
	cursorDevSessionLegacyRel = filepath.Join(DockpipeDirRel, "cursor-dev", "session_container")
	hostCleanupExecCommandFn  = exec.CommandContext
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

// ApplyHostCleanup stops Docker containers registered for this host run.
// With DOCKPIPE_RUN_ID, only runs/<id>.container is used (not a global cleanup/ sweep).
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

	runID := strings.TrimSpace(envGet(env, "DOCKPIPE_RUN_ID"))
	if runID != "" {
		if !isValidHostRunID(runID) {
			return
		}
		applyRunScopedHostCleanup(ctx, wdAbs, runID)
		return
	}

	applyLegacyHostCleanupSweep(ctx, wdAbs)
}

// isValidHostRunID rejects path segments and spoofed env (expected: 8 hex chars from BeginHostRun).
func isValidHostRunID(runID string) bool {
	if len(runID) != 8 {
		return false
	}
	for _, c := range runID {
		switch {
		case c >= '0' && c <= '9':
		case c >= 'a' && c <= 'f':
		case c >= 'A' && c <= 'F':
		default:
			return false
		}
	}
	return true
}

func applyRunScopedHostCleanup(ctx context.Context, wdAbs, runID string) {
	sidecar := filepath.Join(HostRunsDir(wdAbs), runID+".container")
	b, err := os.ReadFile(sidecar)
	if err != nil {
		return
	}
	cn := strings.TrimSpace(string(b))
	if cn == "" {
		_ = os.Remove(sidecar)
		return
	}
	marker := sidecar
	if stopped := tryStopDockerAndRemoveMarker(ctx, cn, marker); stopped {
		fmt.Fprintf(os.Stderr, "[dockpipe] host cleanup: stopped Docker container %s\n", cn)
	}
	removeCleanupMarkersForContainerName(wdAbs, cn)
}

// removeCleanupMarkersForContainerName deletes docker-* and legacy session_container files whose
// single-line content matches cn (after the container was stopped via the run sidecar).
func removeCleanupMarkersForContainerName(wdAbs, cn string) {
	if cn == "" {
		return
	}
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
			if strings.TrimSpace(string(b)) == cn {
				_ = os.Remove(p)
			}
		}
	}
	leg := filepath.Join(wdAbs, cursorDevSessionLegacyRel)
	if b, err := os.ReadFile(leg); err == nil && strings.TrimSpace(string(b)) == cn {
		_ = os.Remove(leg)
	}
}

func applyLegacyHostCleanupSweep(ctx context.Context, wdAbs string) {
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
		cmd := hostCleanupExecCommandFn(ctx, "docker", args...)
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
