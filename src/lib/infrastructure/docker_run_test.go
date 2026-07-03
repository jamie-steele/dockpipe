package infrastructure

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

type call struct {
	name string
	args []string
}

func withDockerSeams(t *testing.T) {
	t.Helper()
	oldExec := execCommandFn
	oldGetwd := getwdDockerFn
	oldAbs := filepathAbsDocker
	oldStat := osStatDockerFn
	oldMkdir := mkdirAllDockerFn
	oldUID := getuidDockerFn
	oldGID := getgidDockerFn
	oldTTY := isTerminalDockerFn
	oldNow := timeNowDockerFn
	oldCommit := commitOnHostFn
	t.Cleanup(func() {
		execCommandFn = oldExec
		getwdDockerFn = oldGetwd
		filepathAbsDocker = oldAbs
		osStatDockerFn = oldStat
		mkdirAllDockerFn = oldMkdir
		getuidDockerFn = oldUID
		getgidDockerFn = oldGID
		isTerminalDockerFn = oldTTY
		timeNowDockerFn = oldNow
		commitOnHostFn = oldCommit
	})
}

// TestRunContainerRequiresImage fails fast when Image is empty.
func TestRunContainerRequiresImage(t *testing.T) {
	rc, err := RunContainer(RunOpts{}, nil)
	if err == nil || rc != 1 {
		t.Fatalf("expected image required error, got rc=%d err=%v", rc, err)
	}
}

func TestRunContainerGetwdError(t *testing.T) {
	withDockerSeams(t)
	getwdDockerFn = func() (string, error) { return "", errors.New("getwd fail") }
	rc, err := RunContainer(RunOpts{Image: "img"}, nil)
	if err == nil || rc != 1 {
		t.Fatalf("expected getwd error, got rc=%d err=%v", rc, err)
	}
}

// TestRunContainerReinitNoTTYRequiresForce refuses destructive --reinit without a TTY unless --force.
func TestRunContainerReinitNoTTYRequiresForce(t *testing.T) {
	withDockerSeams(t)
	isTerminalDockerFn = func(fd int) bool { return false }
	execCommandFn = func(name string, args ...string) *exec.Cmd {
		t.Fatalf("exec should not be called on no-TTY reinit check")
		return nil
	}
	in, _ := os.CreateTemp(t.TempDir(), "in")
	out, _ := os.CreateTemp(t.TempDir(), "out")
	errf, _ := os.CreateTemp(t.TempDir(), "err")
	defer in.Close()
	defer out.Close()
	defer errf.Close()

	rc, err := RunContainer(RunOpts{
		Image:      "img",
		DataVolume: "dockpipe-data",
		Reinit:     true,
		Force:      false,
		Stdin:      in,
		Stdout:     out,
		Stderr:     errf,
	}, []string{"echo", "x"})
	if err == nil || !strings.Contains(err.Error(), "no TTY") || rc != 1 {
		t.Fatalf("expected no-TTY error, got rc=%d err=%v", rc, err)
	}
}

// TestRunContainerDetachBuildsDockerRun exercises detached mode: chown helper (Unix), docker run -d, mounts and env.
func TestRunContainerDetachBuildsDockerRun(t *testing.T) {
	withDockerSeams(t)
	var mu sync.Mutex
	var calls []call
	execCommandFn = func(name string, args ...string) *exec.Cmd {
		mu.Lock()
		calls = append(calls, call{name: name, args: append([]string(nil), args...)})
		mu.Unlock()
		return helperExitCommand(0)
	}
	getwdDockerFn = func() (string, error) { return "/tmp/wd", nil }
	filepathAbsDocker = func(path string) (string, error) { return path, nil }
	osStatDockerFn = func(name string) (os.FileInfo, error) { return nil, os.ErrNotExist }
	mkdirAllDockerFn = func(path string, perm os.FileMode) error { return nil }
	getuidDockerFn = func() int { return 1000 }
	getgidDockerFn = func() int { return 1000 }
	isTerminalDockerFn = func(fd int) bool { return false }
	timeNowDockerFn = func() time.Time { return time.Unix(1000, 0) }
	in, _ := os.CreateTemp(t.TempDir(), "in")
	out, _ := os.CreateTemp(t.TempDir(), "out")
	errf, _ := os.CreateTemp(t.TempDir(), "err")
	defer in.Close()
	defer out.Close()
	defer errf.Close()

	rc, err := RunContainer(RunOpts{
		Image:       "img",
		Detach:      true,
		WorkdirHost: "/tmp/wd",
		DataDir:     "/tmp/data",
		ExtraMounts: []string{" /h:/c ", ""},
		ExtraEnv:    []string{" A=B ", ""},
		Stdin:       in,
		Stdout:      out,
		Stderr:      errf,
	}, []string{"echo", "ok"})
	if err != nil || rc != 0 {
		t.Fatalf("RunContainer detach failed rc=%d err=%v", rc, err)
	}

	mu.Lock()
	defer mu.Unlock()
	wantMin := 2
	if runtime.GOOS == "windows" {
		// Chown helper docker run is skipped on Windows (no meaningful host uid/gid).
		wantMin = 1
	}
	if len(calls) < wantMin {
		t.Fatalf("expected at least %d docker calls, got %#v", wantMin, calls)
	}
	if runtime.GOOS != "windows" {
		ch := calls[0]
		chJoined := strings.Join(ch.args, " ")
		if !isDockerCommandName(ch.name) || !strings.Contains(chJoined, "chown") {
			t.Fatalf("expected first call to be chown docker run, got %s %v", ch.name, ch.args)
		}
	}
	last := calls[len(calls)-1]
	joined := strings.Join(last.args, " ")
	if !isDockerCommandName(last.name) || !strings.Contains(joined, "-d --rm") || !strings.Contains(joined, "img echo ok") {
		t.Fatalf("unexpected detach docker args: %s %s", last.name, joined)
	}
	if runtime.GOOS == "windows" {
		if strings.Contains(joined, "-u ") {
			t.Fatalf("expected no -u on Windows detach run by default (set DOCKPIPE_WINDOWS_CONTAINER_USER to opt in), got %s", joined)
		}
	}
}

func TestRunContainerAppliesSecurityFlags(t *testing.T) {
	withDockerSeams(t)
	var mu sync.Mutex
	var calls []call
	execCommandFn = func(name string, args ...string) *exec.Cmd {
		mu.Lock()
		calls = append(calls, call{name: name, args: append([]string(nil), args...)})
		mu.Unlock()
		return helperExitCommand(0)
	}
	getwdDockerFn = func() (string, error) { return "/tmp/wd", nil }
	filepathAbsDocker = func(path string) (string, error) { return path, nil }
	osStatDockerFn = func(name string) (os.FileInfo, error) { return nil, os.ErrNotExist }
	mkdirAllDockerFn = func(path string, perm os.FileMode) error { return nil }
	getuidDockerFn = func() int { return 1000 }
	getgidDockerFn = func() int { return 1000 }
	isTerminalDockerFn = func(fd int) bool { return false }
	timeNowDockerFn = func() time.Time { return time.Unix(1000, 0) }
	in, _ := os.CreateTemp(t.TempDir(), "in")
	out, _ := os.CreateTemp(t.TempDir(), "out")
	errf, _ := os.CreateTemp(t.TempDir(), "err")
	defer in.Close()
	defer out.Close()
	defer errf.Close()

	rc, err := RunContainer(RunOpts{
		Image:          "img",
		WorkdirHost:    "/tmp/wd",
		ContainerUser:  "0:0",
		ReadOnlyRootFS: true,
		TmpfsPaths:     []string{"/tmp", "/var/tmp"},
		SecurityOpt:    []string{"no-new-privileges"},
		CapDrop:        []string{"ALL"},
		CapAdd:         []string{"NET_BIND_SERVICE"},
		PIDLimit:       64,
		CPULimit:       "2",
		MemoryLimit:    "1g",
		Stdin:          in,
		Stdout:         out,
		Stderr:         errf,
	}, []string{"echo", "ok"})
	if err != nil || rc != 0 {
		t.Fatalf("RunContainer failed rc=%d err=%v", rc, err)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(calls) == 0 {
		t.Fatal("expected docker run call")
	}
	joined := ""
	for _, c := range calls {
		if isDockerCommandName(c.name) && len(c.args) > 0 && c.args[0] == "run" {
			joined = strings.Join(c.args, " ")
			break
		}
	}
	if joined == "" {
		t.Fatalf("expected docker run call, got %#v", calls)
	}
	for _, want := range []string{
		"-u 0:0",
		"--read-only",
		"--tmpfs /tmp",
		"--tmpfs /var/tmp",
		"--security-opt no-new-privileges",
		"--cap-drop ALL",
		"--cap-add NET_BIND_SERVICE",
		"--pids-limit 64",
		"--cpus 2",
		"--memory 1g",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("expected docker args to contain %q, got %s", want, joined)
		}
	}
}

// TestRunContainerAttachedExitCodeTriggersLogsAndRm on non-zero exit runs docker logs and rm for the container.
func TestRunContainerAttachedExitCodeTriggersLogsAndRm(t *testing.T) {
	withDockerSeams(t)
	var mu sync.Mutex
	var calls []call
	execCommandFn = func(name string, args ...string) *exec.Cmd {
		mu.Lock()
		calls = append(calls, call{name: name, args: append([]string(nil), args...)})
		mu.Unlock()
		if len(args) > 0 && args[0] == "run" {
			return helperExitCommand(7)
		}
		return helperExitCommand(0)
	}
	getwdDockerFn = func() (string, error) { return "/tmp/wd", nil }
	filepathAbsDocker = func(path string) (string, error) { return path, nil }
	osStatDockerFn = func(name string) (os.FileInfo, error) { return nil, os.ErrNotExist }
	isTerminalDockerFn = func(fd int) bool { return false }
	timeNowDockerFn = func() time.Time { return time.Unix(1000, 0) }
	in, _ := os.CreateTemp(t.TempDir(), "in")
	out, _ := os.CreateTemp(t.TempDir(), "out")
	errf, _ := os.CreateTemp(t.TempDir(), "err")
	defer in.Close()
	defer out.Close()
	defer errf.Close()

	rc, err := RunContainer(RunOpts{
		Image:  "img",
		Stdin:  in,
		Stdout: out,
		Stderr: errf,
	}, []string{"false"})
	if err != nil {
		t.Fatalf("expected nil error with non-zero rc, got %v", err)
	}
	if rc != 7 {
		t.Fatalf("expected rc 7, got %d", rc)
	}
	mu.Lock()
	defer mu.Unlock()
	got := strings.Join(flattenCalls(calls), "\n")
	if !strings.Contains(got, "docker logs") || !strings.Contains(got, "docker rm") {
		t.Fatalf("expected logs and rm calls, got:\n%s", got)
	}
}

// TestRunContainerAttachedCallsCommitOnHost invokes commit-on-host after a successful attached run when requested.
func TestRunContainerAttachedCallsCommitOnHost(t *testing.T) {
	withDockerSeams(t)
	execCommandFn = func(name string, args ...string) *exec.Cmd {
		return helperExitCommand(0)
	}
	getwdDockerFn = func() (string, error) { return "/tmp/wd", nil }
	filepathAbsDocker = func(path string) (string, error) { return path, nil }
	osStatDockerFn = func(name string) (os.FileInfo, error) { return nil, os.ErrNotExist }
	isTerminalDockerFn = func(fd int) bool { return false }
	timeNowDockerFn = func() time.Time { return time.Unix(1000, 0) }
	called := false
	wantWorkdir := HostPathForGit("/tmp/wd")
	commitOnHostFn = func(workdir, message, bundleOut string, bundleAll bool) error {
		called = true
		if workdir != wantWorkdir || message != "m" || bundleOut != "b.bundle" || bundleAll {
			t.Fatalf("unexpected commit args: %q %q %q bundleAll=%v", workdir, message, bundleOut, bundleAll)
		}
		return nil
	}
	in, _ := os.CreateTemp(t.TempDir(), "in")
	out, _ := os.CreateTemp(t.TempDir(), "out")
	errf, _ := os.CreateTemp(t.TempDir(), "err")
	defer in.Close()
	defer out.Close()
	defer errf.Close()

	rc, err := RunContainer(RunOpts{
		Image:         "img",
		WorkdirHost:   "/tmp/wd",
		CommitOnHost:  true,
		CommitMessage: "m",
		BundleOut:     "b.bundle",
		Stdin:         in,
		Stdout:        out,
		Stderr:        errf,
	}, []string{"echo", "ok"})
	if err != nil || rc != 0 {
		t.Fatalf("RunContainer attached failed rc=%d err=%v", rc, err)
	}
	if !called {
		t.Fatal("expected CommitOnHost to be called")
	}
}

func TestRunContainerWorkspaceVolumeSyncsAroundRun(t *testing.T) {
	withDockerSeams(t)
	var mu sync.Mutex
	var calls []call
	execCommandFn = func(name string, args ...string) *exec.Cmd {
		mu.Lock()
		calls = append(calls, call{name: name, args: append([]string(nil), args...)})
		mu.Unlock()
		return helperExitCommand(0)
	}
	repo := initGitSessionTestRepo(t)
	getwdDockerFn = func() (string, error) { return repo, nil }
	filepathAbsDocker = func(path string) (string, error) { return path, nil }
	osStatDockerFn = os.Stat
	isTerminalDockerFn = func(fd int) bool { return false }
	timeNowDockerFn = func() time.Time { return time.Unix(1000, 0) }
	in, _ := os.CreateTemp(t.TempDir(), "in")
	out, _ := os.CreateTemp(t.TempDir(), "out")
	errf, _ := os.CreateTemp(t.TempDir(), "err")
	defer in.Close()
	defer out.Close()
	defer errf.Close()

	rc, err := RunContainer(RunOpts{
		Image:         "img",
		WorkdirHost:   repo,
		WorkdirVolume: "dockpipe-ws-demo",
		Stdin:         in,
		Stdout:        out,
		Stderr:        errf,
	}, []string{"echo", "ok"})
	if err != nil || rc != 0 {
		t.Fatalf("RunContainer failed rc=%d err=%v", rc, err)
	}
	if err := errf.Close(); err != nil {
		t.Fatalf("close stderr capture: %v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	got := strings.Join(flattenCalls(calls), "\n")
	if strings.Count(got, "docker run") < 3 {
		t.Fatalf("expected sync-in, main run, and sync-out docker run calls, got:\n%s", got)
	}
	if !strings.Contains(got, "--name dockpipe-helper-session-volume-seed-") {
		t.Fatalf("expected stable seed helper container name, got:\n%s", got)
	}
	if !strings.Contains(got, "--label com.dockpipe.helper=1") || !strings.Contains(got, "--label com.dockpipe.helper.unit=session.volume.seed") {
		t.Fatalf("expected helper labels on workspace sync container, got:\n%s", got)
	}
	if !strings.Contains(got, "dockpipe-ws-demo:/work") {
		t.Fatalf("expected main run to mount workspace volume at /work, got:\n%s", got)
	}
	if !strings.Contains(got, repo+":/dockpipe-sync-src:ro") || !strings.Contains(got, "dockpipe-ws-demo:/dockpipe-sync-dst") {
		t.Fatalf("expected host to volume bootstrap call, got:\n%s", got)
	}
	if strings.Count(got, "--entrypoint sh") < 2 {
		t.Fatalf("expected helper sync containers to force sh entrypoint, got:\n%s", got)
	}
	if !strings.Contains(got, "git clone /dockpipe-sync-src /dockpipe-sync-dst") || !strings.Contains(got, "git -C /dockpipe-sync-dst fetch --prune dockpipe-host") {
		t.Fatalf("expected git-based volume bootstrap script, got:\n%s", got)
	}
	if !strings.Contains(got, "dockpipe-ws-demo:/dockpipe-sync-src") || !strings.Contains(got, repo+":/dockpipe-sync-dst") {
		t.Fatalf("expected volume to host patch apply call, got:\n%s", got)
	}
	if !strings.Contains(got, "git -C /dockpipe-sync-src diff --cached --binary | git -C /dockpipe-sync-dst apply --whitespace=nowarn -") {
		t.Fatalf("expected git patch apply sync-back script, got:\n%s", got)
	}
	stderrBytes, readErr := os.ReadFile(errf.Name())
	if readErr != nil {
		t.Fatalf("read stderr capture: %v", readErr)
	}
	stderrText := string(stderrBytes)
	for _, want := range []string{
		"unit=session.volume.sync_in status=start volume=dockpipe-ws-demo",
		"unit=session.volume.sync_in status=done duration_ms=",
		"unit=session.volume.sync_out status=start volume=dockpipe-ws-demo",
		"unit=session.volume.sync_out status=done duration_ms=",
	} {
		if !strings.Contains(stderrText, want) {
			t.Fatalf("expected stderr to contain %q, got:\n%s", want, stderrText)
		}
	}
}

func TestRunContainerWorkspaceVolumeUsesTarSyncForGitWorktree(t *testing.T) {
	withDockerSeams(t)
	var mu sync.Mutex
	var calls []call
	execCommandFn = func(name string, args ...string) *exec.Cmd {
		mu.Lock()
		calls = append(calls, call{name: name, args: append([]string(nil), args...)})
		mu.Unlock()
		return helperExitCommand(0)
	}
	repo := initGitSessionTestRepo(t)
	session, err := CreateSessionBranch(GitSessionRequest{
		WorkspaceID:  "demo",
		SourceDir:    repo,
		Mode:         "managed",
		Storage:      "worktree",
		BranchPrefix: "ai",
		SessionID:    "volume-worktree-test",
	})
	if err != nil {
		t.Fatalf("CreateSessionBranch: %v", err)
	}
	t.Cleanup(func() {
		gitRemoveWorktree(t, repo, session.Storage.Workspace)
	})
	getwdDockerFn = func() (string, error) { return session.Storage.Workspace, nil }
	filepathAbsDocker = func(path string) (string, error) { return path, nil }
	osStatDockerFn = os.Stat
	isTerminalDockerFn = func(fd int) bool { return false }
	timeNowDockerFn = func() time.Time { return time.Unix(1000, 0) }
	in, _ := os.CreateTemp(t.TempDir(), "in")
	out, _ := os.CreateTemp(t.TempDir(), "out")
	errf, _ := os.CreateTemp(t.TempDir(), "err")
	defer in.Close()
	defer out.Close()
	defer errf.Close()

	rc, err := RunContainer(RunOpts{
		Image:         "img",
		WorkdirHost:   session.Storage.Workspace,
		WorkdirVolume: "dockpipe-ws-worktree-demo",
		Stdin:         in,
		Stdout:        out,
		Stderr:        errf,
	}, []string{"echo", "ok"})
	if err != nil || rc != 0 {
		t.Fatalf("RunContainer failed rc=%d err=%v", rc, err)
	}

	mu.Lock()
	defer mu.Unlock()
	got := strings.Join(flattenCalls(calls), "\n")
	if strings.Contains(got, "git clone /dockpipe-sync-src /dockpipe-sync-dst") || strings.Contains(got, "git -C /dockpipe-sync-src diff --cached --binary | git -C /dockpipe-sync-dst apply --whitespace=nowarn -") {
		t.Fatalf("expected worktree-backed volume sync to avoid git metadata helper flow, got:\n%s", got)
	}
	if strings.Count(got, "tar --exclude=.git --exclude=bin/.dockpipe --exclude=.dorkpipe -cf - . | tar xf - -C /dockpipe-sync-dst") < 2 {
		t.Fatalf("expected tar sync with .git exclusion for worktree-backed volume sync, got:\n%s", got)
	}
}

func TestRunContainerWorkspaceVolumeSkipsSyncWhenAuthoritative(t *testing.T) {
	withDockerSeams(t)
	var mu sync.Mutex
	var calls []call
	execCommandFn = func(name string, args ...string) *exec.Cmd {
		mu.Lock()
		calls = append(calls, call{name: name, args: append([]string(nil), args...)})
		mu.Unlock()
		return helperExitCommand(0)
	}
	getwdDockerFn = func() (string, error) { return "/tmp/wd", nil }
	filepathAbsDocker = func(path string) (string, error) { return path, nil }
	osStatDockerFn = os.Stat
	isTerminalDockerFn = func(fd int) bool { return false }
	timeNowDockerFn = func() time.Time { return time.Unix(1000, 0) }
	in, _ := os.CreateTemp(t.TempDir(), "in")
	out, _ := os.CreateTemp(t.TempDir(), "out")
	errf, _ := os.CreateTemp(t.TempDir(), "err")
	defer in.Close()
	defer out.Close()
	defer errf.Close()

	rc, err := RunContainer(RunOpts{
		Image:                   "img",
		WorkdirHost:             "/tmp/wd",
		WorkdirVolume:           "dockpipe-ws-authoritative",
		SkipVolumeWorkspaceSync: true,
		Stdin:                   in,
		Stdout:                  out,
		Stderr:                  errf,
	}, []string{"echo", "ok"})
	if err != nil || rc != 0 {
		t.Fatalf("RunContainer failed rc=%d err=%v", rc, err)
	}
	mu.Lock()
	defer mu.Unlock()
	got := strings.Join(flattenCalls(calls), "\n")
	if strings.Count(got, "docker run") != 1 {
		t.Fatalf("expected only the main docker run call, got:\n%s", got)
	}
	if !strings.Contains(got, "dockpipe-ws-authoritative:/work") {
		t.Fatalf("expected authoritative volume mount at /work, got:\n%s", got)
	}
	if strings.Contains(got, "/dockpipe-sync-src") || strings.Contains(got, "/dockpipe-sync-dst") {
		t.Fatalf("expected no workspace sync helper calls, got:\n%s", got)
	}
}

// TestRunContainerAttachedSkipsCommitOnNonZero does not invoke CommitOnHost when the container exits with an error.
func TestRunContainerAttachedSkipsCommitOnNonZero(t *testing.T) {
	withDockerSeams(t)
	execCommandFn = func(name string, args ...string) *exec.Cmd {
		// docker run ... → failure; docker logs / rm still run
		if isDockerCommandName(name) && len(args) > 0 && args[0] == "run" {
			return helperExitCommand(3)
		}
		return helperExitCommand(0)
	}
	getwdDockerFn = func() (string, error) { return "/tmp/wd", nil }
	filepathAbsDocker = func(path string) (string, error) { return path, nil }
	osStatDockerFn = func(name string) (os.FileInfo, error) { return nil, os.ErrNotExist }
	isTerminalDockerFn = func(fd int) bool { return false }
	timeNowDockerFn = func() time.Time { return time.Unix(1000, 0) }
	called := false
	commitOnHostFn = func(workdir, message, bundleOut string, bundleAll bool) error {
		called = true
		return nil
	}
	in, _ := os.CreateTemp(t.TempDir(), "in")
	out, _ := os.CreateTemp(t.TempDir(), "out")
	errf, _ := os.CreateTemp(t.TempDir(), "err")
	defer in.Close()
	defer out.Close()
	defer errf.Close()

	rc, err := RunContainer(RunOpts{
		Image:        "img",
		WorkdirHost:  "/tmp/wd",
		CommitOnHost: true,
		Stdin:        in,
		Stdout:       out,
		Stderr:       errf,
	}, []string{"false"})
	if err != nil {
		t.Fatalf("RunContainer: %v", err)
	}
	if rc != 3 {
		t.Fatalf("expected rc 3, got %d", rc)
	}
	if called {
		t.Fatal("expected CommitOnHost to be skipped on container failure")
	}
}

// TestRunContainerActionAbsError fails when the action script path cannot be made absolute.
func TestRunContainerActionAbsError(t *testing.T) {
	withDockerSeams(t)
	filepathAbsDocker = func(path string) (string, error) {
		if path == "bad-action.sh" {
			return "", errors.New("abs fail")
		}
		return path, nil
	}
	getwdDockerFn = func() (string, error) { return "/tmp/wd", nil }
	isTerminalDockerFn = func(fd int) bool { return false }
	in, _ := os.CreateTemp(t.TempDir(), "in")
	out, _ := os.CreateTemp(t.TempDir(), "out")
	errf, _ := os.CreateTemp(t.TempDir(), "err")
	defer in.Close()
	defer out.Close()
	defer errf.Close()

	rc, err := RunContainer(RunOpts{
		Image:      "img",
		ActionPath: "bad-action.sh",
		Stdin:      in,
		Stdout:     out,
		Stderr:     errf,
	}, []string{"echo", "x"})
	if err == nil || rc != 1 {
		t.Fatalf("expected action abs error, got rc=%d err=%v", rc, err)
	}
}

// TestDockerBuildPaths verifies DockerBuild runs inspect/build steps for dockpipe-dev (base chain) and plain image names.
func TestDockerBuildPaths(t *testing.T) {
	withDockerSeams(t)
	var mu sync.Mutex
	var calls []call
	execCommandFn = func(name string, args ...string) *exec.Cmd {
		mu.Lock()
		calls = append(calls, call{name: name, args: append([]string(nil), args...)})
		mu.Unlock()
		// Force inspect miss to trigger base build path.
		if len(args) >= 3 && args[0] == "image" && args[1] == "inspect" {
			return helperExitCommand(1)
		}
		return helperExitCommand(0)
	}
	if err := DockerBuild("dockpipe-dev:0.6.0", "/repo/templates/core/assets/images/dev", "/repo"); err != nil {
		t.Fatalf("DockerBuild dockpipe-dev failed: %v", err)
	}
	if err := DockerBuild("ubuntu:latest", "/repo/templates/core/assets/images/dev", "/repo"); err != nil {
		t.Fatalf("DockerBuild non-dockpipe failed: %v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	got := strings.Join(flattenCalls(calls), "\n")
	if !strings.Contains(got, "docker image inspect dockpipe-base-dev:latest") {
		t.Fatalf("expected base image inspect call, got:\n%s", got)
	}
	if !strings.Contains(got, "docker build -q -t dockpipe-base-dev") {
		t.Fatalf("expected base image build call, got:\n%s", got)
	}
	if !strings.Contains(got, "docker build -q -t ubuntu:latest") {
		t.Fatalf("expected final image build call, got:\n%s", got)
	}
}

func TestDockerBuildUsesConfiguredDockerBinary(t *testing.T) {
	withDockerSeams(t)
	fakeDocker := filepath.Join(t.TempDir(), "docker-test")
	if err := os.WriteFile(fakeDocker, []byte(""), 0o755); err != nil {
		t.Fatalf("write fake docker: %v", err)
	}
	t.Setenv("DOCKPIPE_DOCKER_BIN", fakeDocker)
	var calls []call
	execCommandFn = func(name string, args ...string) *exec.Cmd {
		calls = append(calls, call{name: name, args: append([]string(nil), args...)})
		return helperExitCommand(0)
	}
	if err := DockerBuild("ubuntu:latest", "/repo/templates/core/assets/images/dev", "/repo"); err != nil {
		t.Fatalf("DockerBuild failed: %v", err)
	}
	if len(calls) != 1 {
		t.Fatalf("expected one docker call, got %#v", calls)
	}
	if calls[0].name != fakeDocker {
		t.Fatalf("expected configured docker binary %q, got %q", fakeDocker, calls[0].name)
	}
}

func TestDockerBuildEnvEnablesBuildKitByDefault(t *testing.T) {
	got := dockerBuildEnv([]string{"PATH=/usr/bin"}, "docker")
	if !containsEnv(got, "DOCKER_BUILDKIT=1") {
		t.Fatalf("expected DOCKER_BUILDKIT=1, got %#v", got)
	}
	got = dockerBuildEnv([]string{"DOCKER_BUILDKIT=0"}, "docker")
	if len(got) != 1 || got[0] != "DOCKER_BUILDKIT=0" {
		t.Fatalf("expected existing DOCKER_BUILDKIT to be preserved, got %#v", got)
	}
}

func TestDockerCommandEnvPrependsDockerBinaryDir(t *testing.T) {
	dockerCmd := filepath.Join(t.TempDir(), "docker")
	got := dockerCommandEnv([]string{"PATH=/usr/bin"}, dockerCmd)
	want := filepath.Dir(dockerCmd) + string(os.PathListSeparator) + "/usr/bin"
	if !containsEnv(got, "PATH="+want) {
		t.Fatalf("expected PATH to include docker binary dir, got %#v", got)
	}
}

func containsEnv(env []string, want string) bool {
	for _, entry := range env {
		if entry == want {
			return true
		}
	}
	return false
}

func flattenCalls(calls []call) []string {
	out := make([]string, 0, len(calls))
	for _, c := range calls {
		name := c.name
		if isDockerCommandName(name) {
			name = "docker"
		}
		out = append(out, name+" "+strings.Join(c.args, " "))
	}
	return out
}

func isDockerCommandName(name string) bool {
	base := strings.ToLower(filepath.Base(name))
	return base == "docker" || base == "docker.exe"
}

func helperExitCommand(code int) *exec.Cmd {
	cmd := exec.Command(os.Args[0], "-test.run=TestDockerHelperProcess", "--", strconv.Itoa(code))
	cmd.Env = append(os.Environ(), fmt.Sprintf("GO_WANT_DOCKER_HELPER_PROCESS=%d", code))
	return cmd
}

func TestDockerHelperProcess(t *testing.T) {
	code := strings.TrimSpace(os.Getenv("GO_WANT_DOCKER_HELPER_PROCESS"))
	if code == "" && len(os.Args) > 0 {
		code = strings.TrimSpace(os.Args[len(os.Args)-1])
	}
	if code == "" {
		return
	}
	if n, err := strconv.Atoi(code); err == nil {
		os.Exit(n)
	}
	os.Exit(1)
}
