package infrastructure

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// SourceHostScript runs a bash script with set -a (export all) and returns the resulting environment as a map.
func SourceHostScript(scriptPath string, env []string) (map[string]string, error) {
	bashExe, err := resolveBashExe()
	if err != nil {
		hint := ""
		if runtime.GOOS == "windows" {
			hint = " (dockpipe requires bash on the host — e.g. Git for Windows, or DOCKPIPE_USE_WSL_BRIDGE=1 with dockpipe in WSL; see docs/install.md)"
		}
		return nil, fmt.Errorf("bash not found for pre-script%s: %w", hint, err)
	}
	pathFor := pathMapperFor(bashExe)
	bashPath, err := pathFor(scriptPath)
	if err != nil {
		return nil, fmt.Errorf("pre-script path: %w", err)
	}
	// Git Bash on Windows is unreliable for bash -c with extra argv ($1 empty) and for passing
	// paths only via env (some setups drop or empty vars). Run a tiny temp script that embeds
	// the path so the shell only sees: bash /path/to/wrapper.sh
	wrapper, err := os.CreateTemp("", "dockpipe-pre-*.sh")
	if err != nil {
		return nil, fmt.Errorf("pre-script wrapper temp file: %w", err)
	}
	wrapperPath := wrapper.Name()
	_ = wrapper.Close()

	defer func() { _ = os.Remove(wrapperPath) }()

	body := fmt.Sprintf("set -euo pipefail\nset -a\n. %s\nenv -0\n", bashSingleQuoted(bashPath))
	if err := os.WriteFile(wrapperPath, []byte(body), 0o600); err != nil {
		return nil, fmt.Errorf("pre-script wrapper write: %w", err)
	}

	// Same path style as bashPath (MSYS /c/... for Git Bash, /mnt/c/... for WSL bash).
	wrapperForBash, err := pathFor(wrapperPath)
	if err != nil {
		return nil, fmt.Errorf("pre-script wrapper path: %w", err)
	}
	cmd := exec.Command(bashExe, wrapperForBash)
	cmd.Env = env
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("pre-script %s: %w\n%s", scriptPath, err, out)
	}
	m := parseEnv0(out)
	return m, nil
}

// RunHostScript runs a bash script as a subprocess with stdin/stdout/stderr attached to this
// process (not sourced). Use for workflow steps with skip_container: true that print messages or
// launch programs — SourceHostScript captures all output and hides it from the user.
func RunHostScript(scriptPath string, env []string) error {
	bashExe, err := resolveBashExe()
	if err != nil {
		hint := ""
		if runtime.GOOS == "windows" {
			hint = " (dockpipe requires bash on the host — e.g. Git for Windows, or DOCKPIPE_USE_WSL_BRIDGE=1 with dockpipe in WSL; see docs/install.md)"
		}
		return fmt.Errorf("bash not found for host script%s: %w", hint, err)
	}
	pathFor := pathMapperFor(bashExe)
	bashPath, err := pathFor(scriptPath)
	if err != nil {
		return fmt.Errorf("host script path: %w", err)
	}
	cmd := exec.Command(bashExe, bashPath)
	cmd.Env = env
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("host script %s: %w", scriptPath, err)
	}
	return nil
}

// bashSingleQuoted returns s as a single-quoted bash literal ('...' with ' escaped as '\”).
func bashSingleQuoted(s string) string {
	return `'` + strings.ReplaceAll(s, `'`, `'\''`) + `'`
}

// resolveBashExe prefers Git for Windows when installed: PATH often resolves
// C:\Windows\System32\bash.exe (WSL) first, which does not understand MSYS /c/... paths.
func resolveBashExe() (string, error) {
	if runtime.GOOS == "windows" {
		if gb := gitBashWindows(); gb != "" {
			return gb, nil
		}
	}
	return exec.LookPath("bash")
}

func gitBashWindows() string {
	candidates := []string{
		filepath.Join(os.Getenv("ProgramFiles"), "Git", "bin", "bash.exe"),
		filepath.Join(os.Getenv("ProgramFiles(x86)"), "Git", "bin", "bash.exe"),
		filepath.Join(os.Getenv("LocalAppData"), "Programs", "Git", "bin", "bash.exe"),
		`C:\Program Files\Git\bin\bash.exe`,
		`C:\Program Files (x86)\Git\bin\bash.exe`,
	}
	seen := map[string]bool{}
	for _, p := range candidates {
		if p == "" || seen[p] {
			continue
		}
		seen[p] = true
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p
		}
	}
	return ""
}

func bashIsWSL(bashExe string) bool {
	s := strings.ToLower(strings.ReplaceAll(bashExe, `\`, `/`))
	return strings.Contains(s, "/system32/bash") ||
		strings.Contains(s, "windowsapps") ||
		strings.Contains(s, "/wsl/")
}

// pathMapperFor picks Windows path rules for the bash binary we invoke.
func pathMapperFor(bashExe string) func(string) (string, error) {
	if runtime.GOOS == "windows" && bashIsWSL(bashExe) {
		return pathForWSL
	}
	return pathForBashSource
}

// pathForWSL maps a Windows path to a path inside WSL (e.g. C:\x → /mnt/c/x).
func pathForWSL(p string) (string, error) {
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
		rest = filepath.ToSlash(rest)
		return "/mnt/" + drive + "/" + rest, nil
	}
	return filepath.ToSlash(abs), nil
}

// pathForBashSource returns a path Git Bash / MSYS accepts for sourcing.
// Windows drive paths like C:\x must become /c/x for reliable lookup.
func pathForBashSource(p string) (string, error) {
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
		rest = filepath.ToSlash(rest)
		return "/" + drive + "/" + rest, nil
	}
	return filepath.ToSlash(abs), nil
}

func parseEnv0(data []byte) map[string]string {
	m := make(map[string]string)
	for _, chunk := range bytes.Split(data, []byte{0}) {
		if len(chunk) == 0 {
			continue
		}
		line := string(chunk)
		k, v, ok := strings.Cut(line, "=")
		if ok {
			m[k] = v
		}
	}
	return m
}
