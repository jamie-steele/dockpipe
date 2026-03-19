package infrastructure

import (
	"os"
	"path/filepath"
)

var (
	executableFn    = os.Executable
	evalSymlinksFn  = filepath.EvalSymlinks
	filepathAbsFn   = filepath.Abs
)

// RepoRoot returns DOCKPIPE_REPO_ROOT layout: from executable parent, or /usr/lib/dockpipe when running from /usr/bin.
func RepoRoot() (string, error) {
	if v := os.Getenv("DOCKPIPE_REPO_ROOT"); v != "" {
		return filepathAbsFn(v)
	}
	exe, err := executableFn()
	if err != nil {
		return "", err
	}
	exe, err = evalSymlinksFn(exe)
	if err != nil {
		return "", err
	}
	dir := filepath.Dir(exe)
	if dir == "/usr/bin" {
		return "/usr/lib/dockpipe", nil
	}
	return filepathAbsFn(filepath.Join(dir, ".."))
}
