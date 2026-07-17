package packagebuild

import (
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strings"
)

func normalizeArchiveMode(name string, mode os.FileMode, isDir bool) os.FileMode {
	if isDir {
		return 0o755
	}
	if shouldForceExecutable(name) {
		return 0o755
	}
	return 0o644
}

func shouldForceExecutable(name string) bool {
	name = strings.TrimSpace(filepathToSlashClean(name))
	if name == "" || strings.HasSuffix(name, "/") {
		return false
	}
	if strings.Contains(name, "/assets/scripts/") {
		return true
	}
	if strings.Contains(name, "/tooling/bin/") {
		return true
	}
	base := strings.ToLower(path.Base(name))
	return strings.HasSuffix(base, ".sh") ||
		strings.HasSuffix(base, ".bash") ||
		strings.HasSuffix(base, ".zsh") ||
		strings.HasSuffix(base, ".ksh")
}

func filepathToSlashClean(name string) string {
	name = strings.ReplaceAll(name, "\\", "/")
	name = path.Clean("/" + name)
	return strings.TrimPrefix(name, "/")
}

var markExecutableOnDisk = bestEffortMarkExecutableOnDisk

func bestEffortMarkExecutableOnDisk(p string) error {
	_ = os.Chmod(p, 0o755)
	if runtime.GOOS != "windows" {
		return nil
	}
	bashExe, err := resolveBashExeForPackagebuild()
	if err != nil {
		return nil
	}
	bashPath, err := pathForDetectedBash(bashExe, p)
	if err != nil {
		return nil
	}
	cmd := exec.Command(bashExe, "-lc", "chmod 755 "+bashSingleQuoted(bashPath))
	if out, err := cmd.CombinedOutput(); err != nil {
		msg := strings.TrimSpace(string(out))
		if msg != "" {
			return fmt.Errorf("chmod via %s failed for %s: %w: %s", bashExe, p, err, msg)
		}
		return fmt.Errorf("chmod via %s failed for %s: %w", bashExe, p, err)
	}
	return nil
}

func bashSingleQuoted(s string) string {
	return `'` + strings.ReplaceAll(s, `'`, `'\''`) + `'`
}

func resolveBashExeForPackagebuild() (string, error) {
	if runtime.GOOS == "windows" {
		if gb := gitBashWindowsForPackagebuild(); gb != "" {
			return gb, nil
		}
	}
	return exec.LookPath("bash")
}

func gitBashWindowsForPackagebuild() string {
	candidates := []string{
		filepath.Join(os.Getenv("ProgramFiles"), "Git", "bin", "bash.exe"),
		filepath.Join(os.Getenv("ProgramFiles(x86)"), "Git", "bin", "bash.exe"),
		filepath.Join(os.Getenv("LocalAppData"), "Programs", "Git", "bin", "bash.exe"),
		`C:\Program Files\Git\bin\bash.exe`,
		`C:\Program Files (x86)\Git\bin\bash.exe`,
	}
	seen := map[string]bool{}
	for _, candidate := range candidates {
		if candidate == "" || seen[candidate] {
			continue
		}
		seen[candidate] = true
		if st, err := os.Stat(candidate); err == nil && !st.IsDir() {
			return candidate
		}
	}
	return ""
}

func pathForDetectedBash(bashExe, p string) (string, error) {
	if runtime.GOOS == "windows" && bashIsWSLForPackagebuild(bashExe) {
		return pathForWSLForPackagebuild(p)
	}
	return pathForGitBashForPackagebuild(p)
}

func bashIsWSLForPackagebuild(bashExe string) bool {
	s := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(bashExe), `\`, `/`))
	return strings.Contains(s, "/system32/bash") ||
		strings.Contains(s, "windowsapps") ||
		strings.Contains(s, "/wsl/")
}

func pathForWSLForPackagebuild(p string) (string, error) {
	abs, err := filepath.Abs(p)
	if err != nil {
		return "", err
	}
	vol := filepath.VolumeName(abs)
	if len(vol) >= 2 && vol[1] == ':' {
		drive := strings.ToLower(string(vol[0]))
		rest := abs[len(vol):]
		for len(rest) > 0 && (rest[0] == '\\' || rest[0] == '/') {
			rest = rest[1:]
		}
		return "/mnt/" + drive + "/" + filepath.ToSlash(rest), nil
	}
	return filepath.ToSlash(abs), nil
}

func pathForGitBashForPackagebuild(p string) (string, error) {
	abs, err := filepath.Abs(p)
	if err != nil {
		return "", err
	}
	if runtime.GOOS != "windows" {
		return abs, nil
	}
	vol := filepath.VolumeName(abs)
	if len(vol) >= 2 && vol[1] == ':' {
		drive := strings.ToLower(string(vol[0]))
		rest := abs[len(vol):]
		for len(rest) > 0 && (rest[0] == '\\' || rest[0] == '/') {
			rest = rest[1:]
		}
		return "/" + drive + "/" + filepath.ToSlash(rest), nil
	}
	return filepath.ToSlash(abs), nil
}
