package infrastructure

import (
	"path/filepath"
	"runtime"
	"strings"
)

// HostPathForGit returns a directory path suitable for invoking git and for Docker Desktop
// bind mounts on the host. Bash pre-scripts export MSYS paths (/c/Users/...) or WSL-style (/mnt/c/...)
// that native Windows APIs and docker.exe do not resolve correctly; convert those to Windows paths.
func HostPathForGit(dir string) string {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return ""
	}
	if runtime.GOOS == "windows" {
		dir = rewriteMsysOrMntToWindows(dir)
	}
	return filepath.Clean(dir)
}

func rewriteMsysOrMntToWindows(p string) string {
	if runtime.GOOS != "windows" {
		return p
	}
	// Git Bash / MSYS: /c/Users/foo -> C:\Users\foo
	if len(p) >= 3 && p[0] == '/' {
		c := p[1]
		if (c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z') && p[2] == '/' {
			drive := strings.ToUpper(string(c)) + ":"
			rest := filepath.FromSlash(p[3:])
			return filepath.Clean(drive + string(filepath.Separator) + rest)
		}
	}
	// WSL-style: /mnt/c/Users/foo -> C:\Users\foo
	if strings.HasPrefix(strings.ToLower(p), "/mnt/") && len(p) > 5 {
		after := p[5:] // after "/mnt/"
		if len(after) >= 2 && after[1] == '/' {
			drive := strings.ToUpper(string(after[0])) + ":"
			tail := filepath.FromSlash(after[2:])
			return filepath.Clean(drive + string(filepath.Separator) + tail)
		}
	}
	return p
}

// normalizeDockerBindMountWindows converts MSYS host paths in Docker -v values "host:container[:opts]".
// On non-Windows it returns m unchanged.
func normalizeDockerBindMountWindows(m string) string {
	if runtime.GOOS != "windows" {
		return m
	}
	m = strings.TrimSpace(m)
	if m == "" {
		return m
	}
	idx := strings.Index(m, ":/")
	if idx < 0 {
		return HostPathForGit(m)
	}
	host := strings.TrimSpace(m[:idx])
	rest := m[idx:]
	return HostPathForGit(host) + rest
}
