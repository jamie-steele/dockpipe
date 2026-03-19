package infrastructure

import (
	"os"
	"path/filepath"
)

// RepoRoot returns DOCKPIPE_REPO_ROOT layout: from executable parent, or /usr/lib/dockpipe when running from /usr/bin.
func RepoRoot() (string, error) {
	if v := os.Getenv("DOCKPIPE_REPO_ROOT"); v != "" {
		return filepath.Abs(v)
	}
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		return "", err
	}
	dir := filepath.Dir(exe)
	if dir == "/usr/bin" {
		return "/usr/lib/dockpipe", nil
	}
	return filepath.Abs(filepath.Join(dir, ".."))
}
