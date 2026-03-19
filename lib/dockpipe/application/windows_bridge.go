package application

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

var windowsGetwdFn = os.Getwd

// TryWindowsWSLBridge runs dockpipe inside WSL when invoked from Windows.
// It returns handled=false for subcommands that must stay on the host (e.g. "windows").
// The current Windows working directory is mapped with wslpath and used as cwd in WSL.
func TryWindowsWSLBridge(argv []string, stdin io.Reader, stdout, stderr io.Writer) (handled bool, exitCode int) {
	if windowsGoosFn() != "windows" {
		return false, 0
	}
	if len(argv) > 0 && argv[0] == "windows" {
		return false, 0
	}
	distro, err := resolveBridgeDistro()
	if err != nil {
		fmt.Fprintf(stderr, "[dockpipe] %v\n", err)
		return true, 1
	}
	winWd, err := windowsGetwdFn()
	if err != nil {
		fmt.Fprintf(stderr, "[dockpipe] get working directory: %v\n", err)
		return true, 1
	}
	winWd, err = filepath.Abs(winWd)
	if err != nil {
		fmt.Fprintf(stderr, "[dockpipe] abs working directory: %v\n", err)
		return true, 1
	}
	wslWd := winPathToWSL(distro, winWd)
	fmt.Fprintf(stderr, "[dockpipe] Windows bridge: distro=%q cwd=%s -> %s\n", distro, winWd, wslWd)

	translated := translateBridgeArgv(distro, argv)
	script := buildBashForwardScript(wslWd, translated)
	cmd := windowsExecCommandFn("wsl.exe", "-d", distro, "--", "bash", "-lc", script)
	cmd.Stdin = stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	cmd.Env = append(os.Environ(), "DOCKPIPE_WINDOWS_BRIDGE=1")
	err = cmd.Run()
	if err == nil {
		return true, 0
	}
	if x, ok := err.(*exec.ExitError); ok {
		return true, x.ExitCode()
	}
	fmt.Fprintf(stderr, "[dockpipe] wsl: %v\n", err)
	return true, 1
}

func resolveBridgeDistro() (string, error) {
	cfg, err := loadWindowsConfig()
	if err != nil {
		return "", err
	}
	if cfg != "" {
		return cfg, nil
	}
	distros, err := listWSLDistros()
	if err != nil {
		return "", fmt.Errorf("WSL: %w (run `dockpipe windows setup` to pick a distro)", err)
	}
	if len(distros) == 0 {
		return "", fmt.Errorf("no WSL distros found; install one with `wsl --install -d Ubuntu` then `dockpipe windows setup`")
	}
	d := distros[0]
	fmt.Fprintf(windowsStderr, "[dockpipe] No %%APPDATA%%\\dockpipe\\windows-config.env; using first distro %q (run `dockpipe windows setup` to pin)\n", d)
	return d, nil
}

func winPathToWSL(distro, winPath string) string {
	cmd := windowsExecCommandFn("wsl.exe", "-d", distro, "wslpath", "-u", winPath)
	out, err := cmd.Output()
	if err == nil {
		return strings.TrimSpace(string(out))
	}
	return windowsPathToWSLFallback(winPath)
}

// windowsPathToWSLFallback maps C:\foo\bar -> /mnt/c/foo/bar when wslpath fails.
// Uses drive-letter parsing so it behaves the same on Windows and on Linux (CI).
func isUNCPathNormalized(s string) bool {
	// After \ -> /, UNC is //server/share (not a triple slash).
	return len(s) >= 3 && strings.HasPrefix(s, "//") && s[2] != '/'
}

func windowsPathToWSLFallback(winPath string) string {
	// Normalize Windows separators without filepath.ToSlash: on Unix GOOS, ToSlash is a no-op.
	s := strings.TrimSpace(winPath)
	s = strings.ReplaceAll(s, `\`, `/`)
	// filepath.Clean turns "//server/share" into "/server/share" on Unix — breaks UNC.
	if !isUNCPathNormalized(s) {
		s = filepath.Clean(s)
	}
	if len(s) >= 2 && s[1] == ':' && s[0] != '/' {
		drive := strings.ToLower(string(s[0]))
		rest := strings.TrimPrefix(s[2:], "/")
		if rest == "" {
			return "/mnt/" + drive
		}
		return "/mnt/" + drive + "/" + rest
	}
	return s
}

func bashSingleQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

func buildBashForwardScript(wslWd string, argv []string) string {
	var sb strings.Builder
	sb.WriteString("cd ")
	sb.WriteString(bashSingleQuote(wslWd))
	sb.WriteString(" && exec dockpipe")
	for _, a := range argv {
		sb.WriteString(" ")
		sb.WriteString(bashSingleQuote(a))
	}
	return sb.String()
}
