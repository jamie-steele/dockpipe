package infrastructure

import (
	"os"
	"path/filepath"
	"strings"
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
	// Debian .deb: /usr/bin/dockpipe → /usr/lib/dockpipe. On Windows, filepath.Dir("/usr/bin/dockpipe")
	// does not equal "/usr/bin", so match on normalized exe path.
	exeSlash := filepath.ToSlash(filepath.Clean(exe))
	if strings.HasSuffix(exeSlash, "/usr/bin/dockpipe") || strings.HasSuffix(exeSlash, "/usr/bin/dockpipe.exe") {
		return "/usr/lib/dockpipe", nil
	}
	dir := filepath.Dir(exe)
	return filepathAbsFn(filepath.Join(dir, ".."))
}
