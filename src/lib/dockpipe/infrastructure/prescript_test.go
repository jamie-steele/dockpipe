package infrastructure

import (
	"path/filepath"
	"runtime"
	"testing"
)

// TestBashIsWSL distinguishes System32 bash (WSL) from Git Bash paths on Windows.
func TestBashIsWSL(t *testing.T) {
	tests := []struct {
		exe  string
		want bool
	}{
		{`C:\Windows\System32\bash.exe`, true},
		{`c:/windows/system32/bash.exe`, true},
		{`C:\Program Files\Git\bin\bash.exe`, false},
		{`/usr/bin/bash`, false},
	}
	for _, tc := range tests {
		if got := bashIsWSL(tc.exe); got != tc.want {
			t.Errorf("bashIsWSL(%q) = %v, want %v", tc.exe, got, tc.want)
		}
	}
}

// TestPathForWSLNonWindowsPath passes Unix paths through on non-Windows (no wslpath).
func TestPathForWSLNonWindowsPath(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("path layout differs on Windows")
	}
	got, err := pathForWSL("/tmp/foo/bar")
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.ToSlash("/tmp/foo/bar")
	if got != want {
		t.Fatalf("pathForWSL: got %q want %q", got, want)
	}
}
