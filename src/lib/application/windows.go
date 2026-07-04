package application

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

var (
	windowsGoosFn                  = func() string { return runtime.GOOS }
	windowsExecCommandFn           = exec.Command
	windowsStdin         io.Reader = os.Stdin
	windowsStdout        io.Writer = os.Stdout
	windowsStderr        io.Writer = os.Stderr
	windowsUserHomeDirFn           = os.UserHomeDir
)

// defaultWSLBootstrapDistro is the default `wsl --install -d …` name for tester bootstrap.
// Alpine is minimal (musl); release Linux binaries are static (CGO_ENABLED=0) and work on Alpine.
// If `wsl --install -d Alpine` is unavailable on your Windows build, use --distro Ubuntu.
const defaultWSLBootstrapDistro = "Alpine"

type windowsSetupOpts struct {
	Distro            string
	InstallCommand    string
	NonInteractive    bool
	BootstrapWSL      bool
	InstallDockpipe   bool
	NoInstallDockpipe bool
}

func cmdWindows(args []string) error {
	if len(args) == 0 || args[0] == "help" || args[0] == "-h" || args[0] == "--help" {
		printWindowsUsage()
		return nil
	}
	switch args[0] {
	case "setup":
		return cmdWindowsSetup(args[1:])
	case "doctor":
		return cmdWindowsDoctor()
	default:
		return fmt.Errorf("unknown windows subcommand %q", args[0])
	}
}

func printWindowsUsage() {
	fmt.Fprint(windowsStdout, `dockpipe windows — WSL setup and diagnostics (Windows only).

Usage:
  dockpipe windows setup [--distro <name>] [--install-command <cmd>] [--non-interactive]
  dockpipe windows doctor

Examples:
  dockpipe windows setup
  dockpipe windows setup --distro Ubuntu --install-command "curl -fsSL https://example/install.sh | sh" --non-interactive

Details: docs/install.md (Windows section).
`)
}

func cmdWindowsDoctor() error {
	if windowsGoosFn() != "windows" {
		return fmt.Errorf("windows commands must run on Windows host")
	}
	if out, err := wslVersionOutput(); err == nil && strings.TrimSpace(out) != "" {
		fmt.Fprintf(windowsStdout, "[dockpipe] %s\n", strings.TrimSpace(out))
	}
	distros, err := listWSLDistros()
	if err != nil {
		fmt.Fprintf(windowsStdout, "[dockpipe] WSL list failed: %v\n", err)
		fmt.Fprintf(windowsStdout, "[dockpipe] If WSL is not installed: run `dockpipe windows setup --bootstrap-wsl --distro Alpine --non-interactive` (may need Administrator / reboot). If Alpine is not offered, use --distro Ubuntu.\n")
		return nil
	}
	if len(distros) == 0 {
		fmt.Fprintf(windowsStdout, "[dockpipe] No WSL distros registered. Install with: `wsl --install -d Alpine` (or Ubuntu) or `dockpipe windows setup --bootstrap-wsl --non-interactive`\n")
		return nil
	}
	fmt.Fprintf(windowsStdout, "[dockpipe] WSL distros found: %s\n", strings.Join(distros, ", "))
	cfg, _ := loadWindowsConfig()
	if cfg != "" {
		fmt.Fprintf(windowsStdout, "[dockpipe] Default configured distro: %s\n", cfg)
	}
	return nil
}

func wslVersionOutput() (string, error) {
	cmd := windowsExecCommandFn("wsl.exe", "--version")
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func cmdWindowsSetup(args []string) error {
	if windowsGoosFn() != "windows" {
		return fmt.Errorf("windows setup must run on Windows host")
	}
	opts, err := parseWindowsSetupArgs(args)
	if err != nil {
		return err
	}
	applyDefaultWSLInstallCommand(&opts)

	distros, err := listWSLDistros()
	needBootstrap := opts.BootstrapWSL && (err != nil || len(distros) == 0)
	if err != nil && !opts.BootstrapWSL {
		return fmt.Errorf("WSL not available (%w). Install WSL or re-run with --bootstrap-wsl --distro Alpine (or Ubuntu)", err)
	}
	if len(distros) == 0 && !opts.BootstrapWSL && err == nil {
		return fmt.Errorf("no WSL distros found. Run `wsl --install -d Alpine` or: dockpipe windows setup --bootstrap-wsl --distro Alpine")
	}
	if needBootstrap {
		distro := opts.Distro
		if distro == "" {
			distro = defaultWSLBootstrapDistro
		}
		fmt.Fprintf(windowsStderr, "[dockpipe] Bootstrapping WSL (distro %q). This may open an Administrator prompt or require a reboot...\n", distro)
		if err := runWSLBootstrap(distro); err != nil {
			return fmt.Errorf("WSL bootstrap: %w", err)
		}
		for attempt := 0; attempt < 5; attempt++ {
			time.Sleep(2 * time.Second)
			distros, err = listWSLDistros()
			if err == nil && len(distros) > 0 {
				break
			}
		}
	}
	if err != nil {
		return fmt.Errorf("WSL not available after bootstrap (%w). If Windows asked for a reboot, restart and run `dockpipe windows setup` again with the same flags", err)
	}
	if len(distros) == 0 {
		return fmt.Errorf("no WSL distros registered yet. If you just installed WSL, reboot Windows, then run `dockpipe windows setup` again (same flags as before)")
	}
	chosen, err := chooseWSLDistro(distros, opts)
	if err != nil {
		return err
	}
	if err := saveWindowsConfig(chosen); err != nil {
		return err
	}
	bootstrap := strings.ReplaceAll(windowsBootstrapScript, "__DISTRO__", shellEscape(chosen))
	if err := runInWSL(chosen, bootstrap); err != nil {
		return fmt.Errorf("bootstrap WSL environment: %w", err)
	}
	if strings.TrimSpace(opts.InstallCommand) != "" {
		fmt.Fprintf(windowsStderr, "[dockpipe] Running install command in WSL distro %q...\n", chosen)
		if err := runInWSL(chosen, opts.InstallCommand); err != nil {
			return fmt.Errorf("install command failed: %w", err)
		}
	}
	if err := runInWSL(chosen, "command -v dockpipe >/dev/null 2>&1"); err != nil {
		return fmt.Errorf("dockpipe not found inside WSL distro %q. Re-run with --install-dockpipe or --install-command …", chosen)
	}
	fmt.Fprintf(windowsStdout, "[dockpipe] Windows setup complete. Default WSL distro: %s\n", chosen)
	return nil
}

func parseWindowsSetupArgs(args []string) (windowsSetupOpts, error) {
	var o windowsSetupOpts
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch a {
		case "--distro":
			if i+1 >= len(args) {
				return o, fmt.Errorf("--distro requires a value")
			}
			o.Distro = args[i+1]
			i++
		case "--install-command":
			if i+1 >= len(args) {
				return o, fmt.Errorf("--install-command requires a value")
			}
			o.InstallCommand = args[i+1]
			i++
		case "--non-interactive":
			o.NonInteractive = true
		case "--bootstrap-wsl":
			o.BootstrapWSL = true
		case "--install-dockpipe":
			o.InstallDockpipe = true
		case "--no-install-dockpipe":
			o.NoInstallDockpipe = true
		default:
			return o, fmt.Errorf("unknown option %s", a)
		}
	}
	return o, nil
}

// applyDefaultWSLInstallCommand sets the GitHub-latest install script when flags ask for it and the user did not pass a custom command.
func applyDefaultWSLInstallCommand(o *windowsSetupOpts) {
	if o.NoInstallDockpipe || strings.TrimSpace(o.InstallCommand) != "" {
		return
	}
	if o.BootstrapWSL || o.InstallDockpipe {
		o.InstallCommand = defaultWSLInstallDockpipeScript()
	}
}

func runWSLBootstrap(distroname string) error {
	cmd := windowsExecCommandFn("wsl.exe", "--install", "-d", distroname)
	cmd.Stdout = windowsStdout
	cmd.Stderr = windowsStderr
	if err := cmd.Run(); err == nil {
		return nil
	}
	fmt.Fprintf(windowsStderr, "[dockpipe] Retrying WSL install with Administrator elevation (UAC prompt)...\n")
	return runWSLBootstrapElevated(distroname)
}

func runWSLBootstrapElevated(distroname string) error {
	// Single-quoted distro name for PowerShell; avoid injection (distro is validated in choose/bootstrap path).
	esc := strings.ReplaceAll(distroname, "'", "''")
	ps := fmt.Sprintf("Start-Process -FilePath 'wsl.exe' -ArgumentList '--install','-d','%s' -Verb RunAs -Wait", esc)
	cmd := windowsExecCommandFn("powershell.exe", "-NoProfile", "-Command", ps)
	cmd.Stdout = windowsStdout
	cmd.Stderr = windowsStderr
	return cmd.Run()
}

// defaultWSLInstallDockpipeScript installs the latest release linux tar.gz into ~/.local/bin (for DOCKPIPE_USE_WSL_BRIDGE=1).
// Minimal deps: Alpine gets a tiny apk set; Debian/Ubuntu/apt a small apt-get line; static dockpipe binary works on musl.
func defaultWSLInstallDockpipeScript() string {
	return `set -eu
REPO="${DOCKPIPE_GITHUB_REPO:-jamie-steele/dockpipe}"
BIN_DIR="${HOME}/.local/bin"
mkdir -p "$BIN_DIR"
if [ -f /etc/alpine-release ]; then
  apk add --no-cache bash curl ca-certificates git tar gzip docker-cli
elif command -v apt-get >/dev/null 2>&1 && [ "$(id -u)" = 0 ]; then
  export DEBIAN_FRONTEND=noninteractive
  apt-get update -qq && apt-get install -y --no-install-recommends curl ca-certificates git
fi
ARCH=$(uname -m)
JSON="$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest")"
case "$ARCH" in
  x86_64) URL=$(printf '%s\n' "$JSON" | tr -d '\r' | grep -oE 'https://[^"]+_linux_amd64\.tar\.gz' | head -1) ;;
  aarch64) URL=$(printf '%s\n' "$JSON" | tr -d '\r' | grep -oE 'https://[^"]+_linux_arm64\.tar\.gz' | head -1) ;;
  *) echo "[dockpipe] Unsupported machine ${ARCH}; need x86_64 or aarch64 WSL" >&2; exit 1 ;;
esac
if [ -z "$URL" ]; then
  echo "[dockpipe] Could not find linux tarball in latest GitHub release for ${REPO}" >&2
  exit 1
fi
TMP="$(mktemp)"
trap 'rm -f "$TMP"' EXIT
curl -fsSL -o "$TMP" "$URL"
tar -xzf "$TMP" -C "$BIN_DIR" dockpipe
chmod +x "$BIN_DIR/dockpipe"
if ! echo ":${PATH:-}:" | grep -Fq ":$BIN_DIR:"; then
  printf '\nexport PATH="$HOME/.local/bin:$PATH"\n' >> "${HOME}/.bashrc"
fi
export PATH="$BIN_DIR:$PATH"
command -v dockpipe
dockpipe version || true
`
}

func listWSLDistros() ([]string, error) {
	cmd := windowsExecCommandFn("wsl.exe", "-l", "-q")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("list WSL distros: %w\n%s", err, out)
	}
	return parseWSLDistroList(string(out)), nil
}

func parseWSLDistroList(s string) []string {
	var out []string
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		out = append(out, strings.TrimPrefix(line, "* "))
	}
	return out
}

func chooseWSLDistro(distros []string, o windowsSetupOpts) (string, error) {
	if o.Distro != "" {
		for _, d := range distros {
			if strings.EqualFold(d, o.Distro) {
				return d, nil
			}
		}
		return "", fmt.Errorf("WSL distro %q not found (available: %s)", o.Distro, strings.Join(distros, ", "))
	}
	if o.NonInteractive {
		return "", fmt.Errorf("--non-interactive requires --distro")
	}
	fmt.Fprintln(windowsStdout, "[dockpipe] Select WSL distro for dockpipe:")
	for i, d := range distros {
		fmt.Fprintf(windowsStdout, "  %d) %s\n", i+1, d)
	}
	fmt.Fprint(windowsStdout, "Enter number (default 1): ")
	br := bufio.NewReader(windowsStdin)
	line, _ := br.ReadString('\n')
	line = strings.TrimSpace(line)
	if line == "" {
		return distros[0], nil
	}
	n, err := strconv.Atoi(line)
	if err != nil || n < 1 || n > len(distros) {
		return "", errors.New("invalid selection")
	}
	return distros[n-1], nil
}

func runInWSL(distro, script string) error {
	cmd := windowsExecCommandFn("wsl.exe", "-d", distro, "--", "sh", "-lc", script)
	cmd.Stdout = windowsStdout
	cmd.Stderr = windowsStderr
	return cmd.Run()
}

func windowsConfigPath() (string, error) {
	appData := os.Getenv("APPDATA")
	if appData == "" {
		home, err := windowsUserHomeDirFn()
		if err != nil {
			return "", err
		}
		appData = filepath.Join(home, "AppData", "Roaming")
	}
	return filepath.Join(appData, "dockpipe", "windows-config.env"), nil
}

func saveWindowsConfig(distro string) error {
	p, err := windowsConfigPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	content := "DOCKPIPE_WSL_DISTRO=" + distro + "\n"
	return os.WriteFile(p, []byte(content), 0o644)
}

func loadWindowsConfig() (string, error) {
	p, err := windowsConfigPath()
	if err != nil {
		return "", err
	}
	b, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	for _, line := range strings.Split(string(b), "\n") {
		if strings.HasPrefix(line, "DOCKPIPE_WSL_DISTRO=") {
			return strings.TrimSpace(strings.TrimPrefix(line, "DOCKPIPE_WSL_DISTRO=")), nil
		}
	}
	return "", nil
}

func shellEscape(s string) string {
	return strings.ReplaceAll(s, "'", `'"'"'`)
}

const windowsBootstrapScript = `
set -eu
mkdir -p "$HOME/.dockpipe"
cat > "$HOME/.dockpipe/windows-host.env" <<'EOF'
export DOCKPIPE_WINDOWS_HOST=1
export DOCKPIPE_WINDOWS_FETCH_BACK=1
export DOCKPIPE_WSL_DISTRO='__DISTRO__'
EOF
if [ -f "$HOME/.bashrc" ] && ! grep -Fq '.dockpipe/windows-host.env' "$HOME/.bashrc"; then
  printf '\n[ -f "$HOME/.dockpipe/windows-host.env" ] && . "$HOME/.dockpipe/windows-host.env"\n' >> "$HOME/.bashrc"
fi
`
