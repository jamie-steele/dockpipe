package application

import "testing"

func TestBashSingleQuote(t *testing.T) {
	if got := bashSingleQuote(`hello`); got != `'hello'` {
		t.Fatalf("got %q", got)
	}
	if got := bashSingleQuote(`it's`); got != `'it'\''s'` {
		t.Fatalf("got %q", got)
	}
}

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

func TestBuildBashForwardScript(t *testing.T) {
	got := buildBashForwardScript("/mnt/c/proj", []string{"--", "echo", "a b"})
	want := "cd '/mnt/c/proj' && exec dockpipe '--' 'echo' 'a b'"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestBuildBashForwardScript_translatedPathsQuoted(t *testing.T) {
	// Simulates argv after translateBridgeArgv (spaces + special chars safe for bash -lc).
	argv := []string{"--workdir", "/mnt/c/Program Files/repo", "--", "echo", "ok"}
	got := buildBashForwardScript("/mnt/c/proj", argv)
	want := "cd '/mnt/c/proj' && exec dockpipe '--workdir' '/mnt/c/Program Files/repo' '--' 'echo' 'ok'"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}
