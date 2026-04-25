package infrastructure

import (
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
