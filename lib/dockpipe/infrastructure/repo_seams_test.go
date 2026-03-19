package infrastructure

import (
	"errors"
	"path/filepath"
	"testing"
)

func TestRepoRootDefaultFromExecutableParent(t *testing.T) {
	oldExe, oldEval, oldAbs := executableFn, evalSymlinksFn, filepathAbsFn
	t.Cleanup(func() {
		executableFn = oldExe
		evalSymlinksFn = oldEval
		filepathAbsFn = oldAbs
	})

	executableFn = func() (string, error) { return "/tmp/bin/dockpipe", nil }
	evalSymlinksFn = func(path string) (string, error) { return path, nil }
	filepathAbsFn = filepath.Abs

	got, err := RepoRoot()
	if err != nil {
		t.Fatalf("RepoRoot error: %v", err)
	}
	want, _ := filepath.Abs("/tmp")
	if got != want {
		t.Fatalf("RepoRoot() = %q, want %q", got, want)
	}
}

func TestRepoRootUsrBinSpecialCase(t *testing.T) {
	oldExe, oldEval, oldAbs := executableFn, evalSymlinksFn, filepathAbsFn
	t.Cleanup(func() {
		executableFn = oldExe
		evalSymlinksFn = oldEval
		filepathAbsFn = oldAbs
	})

	executableFn = func() (string, error) { return "/usr/bin/dockpipe", nil }
	evalSymlinksFn = func(path string) (string, error) { return path, nil }
	filepathAbsFn = filepath.Abs

	got, err := RepoRoot()
	if err != nil {
		t.Fatalf("RepoRoot error: %v", err)
	}
	if got != "/usr/lib/dockpipe" {
		t.Fatalf("RepoRoot() = %q, want /usr/lib/dockpipe", got)
	}
}

func TestRepoRootExecutableError(t *testing.T) {
	oldExe := executableFn
	t.Cleanup(func() { executableFn = oldExe })
	executableFn = func() (string, error) { return "", errors.New("boom") }

	if _, err := RepoRoot(); err == nil {
		t.Fatal("expected error from executableFn")
	}
}

func TestRepoRootEvalSymlinkError(t *testing.T) {
	oldExe, oldEval := executableFn, evalSymlinksFn
	t.Cleanup(func() {
		executableFn = oldExe
		evalSymlinksFn = oldEval
	})
	executableFn = func() (string, error) { return "/x/y", nil }
	evalSymlinksFn = func(path string) (string, error) { return "", errors.New("nope") }

	if _, err := RepoRoot(); err == nil {
		t.Fatal("expected error from evalSymlinksFn")
	}
}
