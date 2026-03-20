package infrastructure

import (
	"errors"
	"os"
	"os/exec"
	"runtime"
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

func TestRunContainerDetachBuildsDockerRun(t *testing.T) {
	withDockerSeams(t)
	var mu sync.Mutex
	var calls []call
	execCommandFn = func(name string, args ...string) *exec.Cmd {
		mu.Lock()
		calls = append(calls, call{name: name, args: append([]string(nil), args...)})
		mu.Unlock()
		return exec.Command("bash", "-c", "exit 0")
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
		Image:         "img",
		Detach:        true,
		WorkdirHost:   "/tmp/wd",
		DataDir:       "/tmp/data",
		ExtraMounts:   []string{" /h:/c ", ""},
		ExtraEnv:      []string{" A=B ", ""},
		Stdin:         in,
		Stdout:        out,
		Stderr:        errf,
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
		if ch.name != "docker" || !strings.Contains(chJoined, "chown") {
			t.Fatalf("expected first call to be chown docker run, got %s %v", ch.name, ch.args)
		}
	}
	last := calls[len(calls)-1]
	joined := strings.Join(last.args, " ")
	if last.name != "docker" || !strings.Contains(joined, "-d --rm") || !strings.Contains(joined, "img echo ok") {
		t.Fatalf("unexpected detach docker args: %s %s", last.name, joined)
	}
	if runtime.GOOS == "windows" && strings.Contains(joined, "-u ") {
		t.Fatalf("expected no -u on Windows detach run, got %s", joined)
	}
}

func TestRunContainerAttachedExitCodeTriggersLogsAndRm(t *testing.T) {
	withDockerSeams(t)
	var mu sync.Mutex
	var calls []call
	execCommandFn = func(name string, args ...string) *exec.Cmd {
		mu.Lock()
		calls = append(calls, call{name: name, args: append([]string(nil), args...)})
		mu.Unlock()
		if len(args) > 0 && args[0] == "run" {
			return exec.Command("bash", "-c", "exit 7")
		}
		if len(args) > 0 && args[0] == "logs" {
			return exec.Command("bash", "-c", "echo line1")
		}
		return exec.Command("bash", "-c", "exit 0")
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

func TestRunContainerAttachedCallsCommitOnHost(t *testing.T) {
	withDockerSeams(t)
	execCommandFn = func(name string, args ...string) *exec.Cmd {
		return exec.Command("bash", "-c", "exit 0")
	}
	getwdDockerFn = func() (string, error) { return "/tmp/wd", nil }
	filepathAbsDocker = func(path string) (string, error) { return path, nil }
	osStatDockerFn = func(name string) (os.FileInfo, error) { return nil, os.ErrNotExist }
	isTerminalDockerFn = func(fd int) bool { return false }
	timeNowDockerFn = func() time.Time { return time.Unix(1000, 0) }
	called := false
	commitOnHostFn = func(workdir, message, bundleOut string, bundleAll bool) error {
		called = true
		if workdir != "/tmp/wd" || message != "m" || bundleOut != "b.bundle" || bundleAll {
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
			return exec.Command("bash", "-c", "exit 1")
		}
		return exec.Command("bash", "-c", "exit 0")
	}
	if err := DockerBuild("dockpipe-dev:0.6.0", "/repo/images/dev", "/repo"); err != nil {
		t.Fatalf("DockerBuild dockpipe-dev failed: %v", err)
	}
	if err := DockerBuild("ubuntu:latest", "/repo/images/dev", "/repo"); err != nil {
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

func flattenCalls(calls []call) []string {
	out := make([]string, 0, len(calls))
	for _, c := range calls {
		out = append(out, c.name+" "+strings.Join(c.args, " "))
	}
	return out
}
