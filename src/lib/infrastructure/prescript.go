package infrastructure

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"golang.org/x/term"
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
	cmd.Env = prepareHostBashEnv(env, bashExe)
	if cwd := strings.TrimSpace(envGet(cmd.Env, "DOCKPIPE_STEP_CWD")); cwd != "" {
		cmd.Dir = cwd
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("pre-script %s: %w\n%s", scriptPath, err, out)
	}
	m := parseEnv0(out)
	return m, nil
}

// RunHostScript runs a bash script as a subprocess with stdin/stdout/stderr attached to this
// process (not sourced). Use for workflow steps with kind: host that print messages or
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
	runID, runFile, env, err := BeginHostRun(envGet(env, "DOCKPIPE_WORKDIR"), env)
	if err != nil {
		return fmt.Errorf("host run registry: %w", err)
	}
	env = applyInteractivePromptMode(env)
	env = prepareHostBashEnv(env, bashExe)
	cmd := exec.Command(bashExe, bashPath)
	cmd.Env = env
	if cwd := strings.TrimSpace(envGet(cmd.Env, "DOCKPIPE_STEP_CWD")); cwd != "" {
		cmd.Dir = cwd
	}
	cmd.Stdin = os.Stdin
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("host script stdout pipe %s: %w", scriptPath, err)
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("host script stderr pipe %s: %w", scriptPath, err)
	}
	setRunHostProcAttrs(cmd)
	defer func() {
		ApplyHostCleanup(env)
		RemoveHostRunArtifacts(runFile)
	}()
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("host script start %s: %w", scriptPath, err)
	}
	stopSpin := StartLineSpinner(os.Stderr, hostScriptSpinnerLabel(scriptPath))
	stopSpinnerOnce := sync.OnceFunc(stopSpin)
	var copyWG sync.WaitGroup
	copyWG.Add(2)
	go streamHostScriptPipe(stdoutPipe, os.Stdout, stopSpinnerOnce, &copyWG)
	go streamHostScriptPipe(stderrPipe, os.Stderr, stopSpinnerOnce, &copyWG)
	if err := WriteHostRunRecord(runFile, runID, cmd.Process.Pid, envGet(env, "DOCKPIPE_WORKDIR"), scriptPath); err != nil {
		_ = cmd.Process.Kill()
		stopSpinnerOnce()
		copyWG.Wait()
		return fmt.Errorf("host run registry write: %w", err)
	}
	err = waitHostScriptWithSignalForward(cmd)
	stopSpinnerOnce()
	copyWG.Wait()
	if err != nil {
		return fmt.Errorf("host script %s: %w", scriptPath, err)
	}
	return nil
}

func applyInteractivePromptMode(env []string) []string {
	if strings.TrimSpace(envGet(env, "DOCKPIPE_SDK_PROMPT_MODE")) != "" {
		return env
	}
	inFd, inOK := fdInt(os.Stdin)
	errFd, errOK := fdInt(os.Stderr)
	if !inOK || !errOK {
		return env
	}
	if !term.IsTerminal(inFd) || !term.IsTerminal(errFd) {
		return env
	}
	return append(env, "DOCKPIPE_SDK_PROMPT_MODE=terminal")
}

// RunHostCommand runs an inline bash command on the host with inherited stdio.
// It uses the same host-run registry and cleanup behavior as RunHostScript.
func RunHostCommand(command string, env []string) error {
	command = strings.TrimSpace(command)
	if command == "" {
		return nil
	}
	f, err := os.CreateTemp("", "dockpipe-hostcmd-*.sh")
	if err != nil {
		return fmt.Errorf("host command temp file: %w", err)
	}
	scriptPath := f.Name()
	_ = f.Close()
	defer func() { _ = os.Remove(scriptPath) }()
	body := command
	if !strings.HasSuffix(body, "\n") {
		body += "\n"
	}
	if err := os.WriteFile(scriptPath, []byte(body), 0o700); err != nil {
		return fmt.Errorf("host command temp write: %w", err)
	}
	return RunHostScript(scriptPath, env)
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

func upsertEnv(env []string, key, value string) []string {
	prefix := key + "="
	out := make([]string, 0, len(env)+1)
	replaced := false
	for _, entry := range env {
		if strings.HasPrefix(entry, prefix) {
			if !replaced {
				out = append(out, prefix+value)
				replaced = true
			}
			continue
		}
		out = append(out, entry)
	}
	if !replaced {
		out = append(out, prefix+value)
	}
	return out
}

func hostScriptSpinnerLabel(scriptPath string) string {
	name := strings.TrimSpace(filepath.Base(scriptPath))
	switch {
	case strings.Contains(name, "clone-worktree"):
		return "Preparing worktree..."
	case strings.Contains(name, "dev-stack"):
		return "Running host setup (dev stack)..."
	default:
		return "Running host setup..."
	}
}

type hostScriptOutputWriter struct {
	dest    io.Writer
	onFirst func()
}

func (w *hostScriptOutputWriter) Write(p []byte) (int, error) {
	if len(bytes.TrimSpace(p)) > 0 && w.onFirst != nil {
		w.onFirst()
		w.onFirst = nil
	}
	return w.dest.Write(p)
}

func streamHostScriptPipe(src io.Reader, dest io.Writer, stopSpinner func(), wg *sync.WaitGroup) {
	defer wg.Done()
	_, _ = io.Copy(&hostScriptOutputWriter{dest: dest, onFirst: stopSpinner}, src)
}

func prepareHostBashEnv(env []string, bashExe string) []string {
	env = upsertEnv(env, "DOCKPIPE_HOST_BASH_BIN", bashExe)
	mergedPath := mergeHostExecutablePATH(envGet(env, "PATH"), os.Getenv("PATH"), bashExe)
	if strings.TrimSpace(mergedPath) != "" {
		env = upsertEnv(env, "PATH", mergedPath)
	}
	return env
}

func mergeHostExecutablePATH(currentPath, hostPath, bashExe string) string {
	currentPath = strings.TrimSpace(currentPath)
	hostPath = strings.TrimSpace(hostPath)
	if currentPath == "" {
		return hostPath
	}
	if hostPath == "" {
		return currentPath
	}

	targetDelim := detectPathListDelimiter(currentPath)
	currentParts := splitPathList(currentPath, targetDelim)
	hostParts := splitPathList(hostPath, detectPathListDelimiter(hostPath))
	if runtime.GOOS == "windows" && targetDelim == ":" {
		hostParts = convertWindowsPathListForBash(hostParts, bashExe)
	}

	seen := make(map[string]bool)
	out := make([]string, 0, len(currentParts))
	for _, part := range currentParts {
		addPathListPart(&out, seen, part)
	}
	for _, part := range hostParts {
		addPathListPart(&out, seen, part)
	}
	return strings.Join(out, targetDelim)
}

func detectPathListDelimiter(path string) string {
	if strings.Contains(path, ";") {
		return ";"
	}
	return ":"
}

func splitPathList(path, delim string) []string {
	if strings.TrimSpace(path) == "" {
		return nil
	}
	raw := strings.Split(path, delim)
	out := make([]string, 0, len(raw))
	for _, part := range raw {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		out = append(out, part)
	}
	return out
}

func addPathListPart(out *[]string, seen map[string]bool, part string) {
	part = strings.TrimSpace(part)
	if part == "" {
		return
	}
	key := part
	if runtime.GOOS == "windows" {
		key = strings.ToLower(key)
	}
	if seen[key] {
		return
	}
	seen[key] = true
	*out = append(*out, part)
}

func convertWindowsPathListForBash(parts []string, bashExe string) []string {
	if len(parts) == 0 {
		return nil
	}
	mapper := pathForBashSource
	if bashIsWSL(bashExe) {
		mapper = pathForWSL
	}
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if looksLikeWindowsAbsolutePath(part) {
			if mapped, err := mapper(part); err == nil && strings.TrimSpace(mapped) != "" {
				out = append(out, mapped)
				continue
			}
		}
		out = append(out, part)
	}
	return out
}

func looksLikeWindowsAbsolutePath(path string) bool {
	if len(path) >= 3 && ((path[1] == ':' && (path[2] == '\\' || path[2] == '/')) ||
		(path[0] == '\\' && path[1] == '\\')) {
		return true
	}
	return false
}
