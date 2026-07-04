package infrastructure

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

// TestRewriteMsysOrMntToWindows maps /c/... and /mnt/c/... style paths to Windows paths (Windows only).
func TestRewriteMsysOrMntToWindows(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("MSYS/WSL path conversion is only applied on Windows")
	}
	tests := []struct {
		in   string
		want string
	}{
		{`/c/Users/Jamie/repo`, filepath.Clean(`C:\Users\Jamie\repo`)},
		{`/C/Program Files/Git`, filepath.Clean(`C:\Program Files\Git`)},
		{`/mnt/c/Users/x/wt`, filepath.Clean(`C:\Users\x\wt`)},
	}
	for _, tt := range tests {
		got := rewriteMsysOrMntToWindows(tt.in)
		if got != tt.want {
			t.Errorf("rewriteMsysOrMntToWindows(%q) = %q want %q", tt.in, got, tt.want)
		}
	}
}

// TestHostPathForGitNonWindowsPassthrough ensures non-Windows hosts get filepath-clean paths unchanged.
func TestHostPathForGitNonWindowsPassthrough(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip()
	}
	in := "/tmp/foo/bar"
	if got := HostPathForGit(in); got != filepath.Clean(in) {
		t.Fatalf("HostPathForGit(%q) = %q", in, got)
	}
}

func TestHostPathForDockerWindowsDaemonRewrite(t *testing.T) {
	t.Setenv("DOCKER_HOST", "npipe:////./pipe/docker_engine")
	got := HostPathForDocker(`/mnt/c/Users/Jamie/repo`)
	want := `C:\Users\Jamie\repo`
	if got != want {
		t.Fatalf("HostPathForDocker rewrite = %q want %q", got, want)
	}
}

// TestNormalizeDockerBindMountWindows normalizes MSYS host paths in host:container bind specs (Windows only).
func TestNormalizeDockerBindMountWindows(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("bind-mount MSYS normalization is Windows-only")
	}
	in := `/c/Users/Jamie/wt:/work`
	want := filepath.Clean(`C:\Users\Jamie\wt`) + `:/work`
	if got := normalizeDockerBindMountWindows(in); got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestNormalizeDockerBindMountWindowsDaemonRewrite(t *testing.T) {
	t.Setenv("DOCKER_HOST", "npipe:////./pipe/docker_engine")
	in := `/mnt/c/Users/Jamie/wt:/work`
	want := `C:\Users\Jamie\wt:/work`
	if got := normalizeDockerBindMountWindows(in); got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestStatePathsNormalizeMsysWorkdirOnWindows(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("MSYS workdir normalization is Windows-only")
	}
	workdir := `/c/Source/uh-workflows`
	wantState := filepath.Clean(`C:\Source\uh-workflows\bin\.dockpipe`)
	gotState, err := StateRoot(workdir)
	if err != nil {
		t.Fatalf("StateRoot returned error: %v", err)
	}
	if gotState != wantState {
		t.Fatalf("StateRoot(%q) = %q, want %q", workdir, gotState, wantState)
	}

	wantCache := filepath.Join(wantState, "internal", "cache", "images")
	gotCache, err := ImageArtifactCacheDir(workdir)
	if err != nil {
		t.Fatalf("ImageArtifactCacheDir returned error: %v", err)
	}
	if gotCache != wantCache {
		t.Fatalf("ImageArtifactCacheDir(%q) = %q, want %q", workdir, gotCache, wantCache)
	}

	wantIndex := filepath.Join(wantState, "internal", "images")
	gotIndex, err := ImageArtifactIndexDir(workdir)
	if err != nil {
		t.Fatalf("ImageArtifactIndexDir returned error: %v", err)
	}
	if gotIndex != wantIndex {
		t.Fatalf("ImageArtifactIndexDir(%q) = %q, want %q", workdir, gotIndex, wantIndex)
	}
}

func TestRewriteViaCygpathFallbackOnWindows(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("cygpath fallback is Windows-only")
	}
	oldLookPath := lookPathHostPathFn
	oldExec := execCommandHostPathFn
	t.Cleanup(func() {
		lookPathHostPathFn = oldLookPath
		execCommandHostPathFn = oldExec
	})
	lookPathHostPathFn = func(file string) (string, error) {
		if file != "cygpath" {
			return "", errors.New("unexpected lookup")
		}
		return `C:\Program Files\Git\usr\bin\cygpath.exe`, nil
	}
	execCommandHostPathFn = func(name string, args ...string) *exec.Cmd {
		cs := []string{"-test.run=TestHostPathGitHelperProcess", "--", name}
		cs = append(cs, args...)
		cmd := exec.Command(os.Args[0], cs...)
		cmd.Env = append(os.Environ(), "GO_WANT_HOSTPATH_HELPER_PROCESS=1")
		return cmd
	}
	got := rewriteMsysOrMntToWindows(`/tmp/mounted.txt`)
	want := filepath.Clean(`C:\Users\Jamie\AppData\Local\Temp\mounted.txt`)
	if got != want {
		t.Fatalf("rewriteMsysOrMntToWindows cygpath fallback = %q want %q", got, want)
	}
}

func TestHostPathGitHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HOSTPATH_HELPER_PROCESS") != "1" {
		return
	}
	args := os.Args
	sep := -1
	for i, arg := range args {
		if arg == "--" {
			sep = i
			break
		}
	}
	if sep < 0 || sep+3 >= len(args) {
		os.Exit(2)
	}
	name := args[sep+1]
	if name != "cygpath" || args[sep+2] != "-aw" || args[sep+3] != "/tmp/mounted.txt" {
		os.Exit(3)
	}
	_, _ = os.Stdout.WriteString("C:\\Users\\Jamie\\AppData\\Local\\Temp\\mounted.txt\r\n")
	os.Exit(0)
}
