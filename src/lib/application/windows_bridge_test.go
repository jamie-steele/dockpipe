package application

import (
	"os"
	"testing"
)

// TestUseWSLBridge reads DOCKPIPE_USE_WSL_BRIDGE for 0/1/unset.
func TestUseWSLBridge(t *testing.T) {
	t.Setenv(EnvUseWSLBridge, "")
	if UseWSLBridge() {
		t.Fatal("expected false when unset")
	}
	t.Setenv(EnvUseWSLBridge, "0")
	if UseWSLBridge() {
		t.Fatal("expected false for 0")
	}
	t.Setenv(EnvUseWSLBridge, "1")
	if !UseWSLBridge() {
		t.Fatal("expected true for 1")
	}
	_ = os.Unsetenv(EnvUseWSLBridge)
}

// TestBashSingleQuote escapes strings for safe bash single-quoted literals.
func TestBashSingleQuote(t *testing.T) {
	if got := bashSingleQuote(`hello`); got != `'hello'` {
		t.Fatalf("got %q", got)
	}
	if got := bashSingleQuote(`it's`); got != `'it'\''s'` {
		t.Fatalf("got %q", got)
	}
}

// TestWindowsPathToWSLFallback maps Windows paths to /mnt/c/... and normalizes UNC for WSL.
func TestWindowsPathToWSLFallback(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"C:/Users/me/repo", "/mnt/c/Users/me/repo"},
		{"c:/tmp/x", "/mnt/c/tmp/x"},
		{`C:\Users\me\repo`, "/mnt/c/Users/me/repo"},
		{`C:\`, "/mnt/c"},
		{`d:\`, "/mnt/d"},
		// UNC: must not run through filepath.Clean on Unix (would become "/server/share").
		{`\\srv\share\path`, "//srv/share/path"},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			if got := windowsPathToWSLFallback(tc.in); got != tc.want {
				t.Fatalf("windowsPathToWSLFallback(%q) = %q want %q", tc.in, got, tc.want)
			}
		})
	}
}

// TestIsUNCPathNormalized detects //server/share style paths used after UNC normalization.
func TestIsUNCPathNormalized(t *testing.T) {
	if !isUNCPathNormalized("//srv/share") {
		t.Fatal("expected UNC")
	}
	if isUNCPathNormalized("/mnt/c/x") {
		t.Fatal("linux path is not UNC")
	}
	if isUNCPathNormalized("//") {
		t.Fatal("too short")
	}
}

// TestBuildBashForwardScript builds the inner bash -lc script for WSL bridge (cd + exec dockpipe).
func TestBuildBashForwardScript(t *testing.T) {
	got := buildBashForwardScript("/mnt/c/proj", []string{"--", "echo", "a b"})
	want := "cd '/mnt/c/proj' && exec dockpipe '--' 'echo' 'a b'"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

// TestBuildBashForwardScript_translatedPathsQuoted ensures argv with spaces survives quoting after translation.
func TestBuildBashForwardScript_translatedPathsQuoted(t *testing.T) {
	// Simulates argv after translateBridgeArgv (spaces + special chars safe for bash -lc).
	argv := []string{"--workdir", "/mnt/c/Program Files/repo", "--", "echo", "ok"}
	got := buildBashForwardScript("/mnt/c/proj", argv)
	want := "cd '/mnt/c/proj' && exec dockpipe '--workdir' '/mnt/c/Program Files/repo' '--' 'echo' 'ok'"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}
