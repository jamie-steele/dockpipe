package infrastructure

import (
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strings"
)

var (
	lookPathHostPathFn = exec.LookPath
	execCommandHostPathFn = exec.Command
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
	if translated, ok := rewriteViaCygpath(p); ok {
		return translated
	}
	return p
}

// normalizeDockerBindMountWindows converts MSYS host paths in Docker -v values "host:container[:opts]".
// On non-Windows it returns m unchanged.
func normalizeDockerBindMountWindows(m string) string {
	if runtime.GOOS != "windows" && !dockerHostUsesWindowsPaths() {
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
	return HostPathForDocker(host) + rest
}

// HostPathForDocker returns a host path suitable for docker bind mounts. Unlike HostPathForGit,
// this also rewrites /mnt/c/... paths when a non-Windows dockpipe process is targeting the
// Windows Docker daemon (for example Linux act containers talking to Docker Desktop over npipe).
func HostPathForDocker(p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return ""
	}
	if runtime.GOOS == "windows" || dockerHostUsesWindowsPaths() {
		if rewritten, ok := rewriteUnixLikeToWindowsPath(p); ok {
			return rewritten
		}
		if runtime.GOOS == "windows" {
			if rewritten := rewriteMsysOrMntToWindows(p); rewritten != p {
				return rewritten
			}
		}
	}
	return filepath.Clean(p)
}

func dockerHostUsesWindowsPaths() bool {
	host := strings.ToLower(strings.TrimSpace(strings.ReplaceAll(os.Getenv("DOCKER_HOST"), "\\", "/")))
	if host == "" {
		return false
	}
	return strings.HasPrefix(host, "npipe:") || strings.Contains(host, "//./pipe/docker_engine")
}

func rewriteUnixLikeToWindowsPath(p string) (string, bool) {
	if len(p) >= 3 && p[1] == ':' && (p[2] == '\\' || p[2] == '/') {
		return cleanWindowsPathString(p[:2], p[2:]), true
	}
	if len(p) >= 3 && p[0] == '/' {
		c := p[1]
		if (c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z') && p[2] == '/' {
			return cleanWindowsPathString(strings.ToUpper(string(c))+":", p[2:]), true
		}
	}
	if strings.HasPrefix(strings.ToLower(p), "/mnt/") && len(p) > 7 {
		after := p[5:]
		if len(after) >= 2 && after[1] == '/' {
			return cleanWindowsPathString(strings.ToUpper(string(after[0]))+":", after[1:]), true
		}
	}
	return "", false
}

func cleanWindowsPathString(drive, tail string) string {
	normalized := strings.ReplaceAll(strings.TrimSpace(tail), "\\", "/")
	cleaned := path.Clean(normalized)
	if cleaned == "." || cleaned == "/" {
		return drive + `\`
	}
	cleaned = strings.TrimPrefix(cleaned, "/")
	return drive + `\` + strings.ReplaceAll(cleaned, "/", `\`)
}

func rewriteViaCygpath(p string) (string, bool) {
	p = strings.TrimSpace(p)
	if p == "" || !strings.HasPrefix(p, "/") {
		return "", false
	}
	if _, err := lookPathHostPathFn("cygpath"); err != nil {
		return "", false
	}
	out, err := execCommandHostPathFn("cygpath", "-aw", p).Output()
	if err != nil {
		return "", false
	}
	translated := strings.TrimSpace(string(out))
	if translated == "" {
		return "", false
	}
	return filepath.Clean(translated), true
}
