package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMustWorkdirPrefersDockPipeContext(t *testing.T) {
	sourceRoot := t.TempDir()
	workdir := t.TempDir()
	t.Setenv("DOCKPIPE_SOURCE_ROOT", sourceRoot)
	t.Setenv("DOCKPIPE_WORKDIR", workdir)

	if got := mustWorkdir(""); got != sourceRoot {
		t.Fatalf("mustWorkdir default = %q want source root %q", got, sourceRoot)
	}
	if got := mustWorkdir(workdir); got != workdir {
		t.Fatalf("mustWorkdir flag = %q want %q", got, workdir)
	}
}

func TestMustWorkdirFallsBackToDockPipeWorkdir(t *testing.T) {
	workdir := t.TempDir()
	t.Setenv("DOCKPIPE_SOURCE_ROOT", "")
	t.Setenv("DOCKPIPE_WORKDIR", workdir)

	if got := mustWorkdir(""); got != workdir {
		t.Fatalf("mustWorkdir default = %q want workdir %q", got, workdir)
	}
}

func TestMustWorkdirFallsBackToCWD(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("DOCKPIPE_SOURCE_ROOT", "")
	t.Setenv("DOCKPIPE_WORKDIR", "")

	if got := mustWorkdir(""); got != wd {
		t.Fatalf("mustWorkdir default = %q want cwd %q", got, wd)
	}
}

func TestMustWorkdirCanonicalizesRelativeFlags(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("DOCKPIPE_SOURCE_ROOT", "")
	t.Setenv("DOCKPIPE_WORKDIR", "")

	got := mustWorkdir(".")
	if got != wd {
		t.Fatalf("mustWorkdir relative flag = %q want %q", got, wd)
	}

	got = providerPoolWorkdirHash(".")
	want := providerPoolWorkdirHash(filepath.Clean(wd))
	if got != want {
		t.Fatalf("providerPoolWorkdirHash relative = %q want %q", got, want)
	}
}
