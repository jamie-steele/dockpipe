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
)

var (
	windowsGoosFn         = func() string { return runtime.GOOS }
	windowsExecCommandFn  = exec.Command
	windowsStdin          io.Reader = os.Stdin
	windowsStdout         io.Writer = os.Stdout
	windowsStderr         io.Writer = os.Stderr
	windowsUserHomeDirFn            = os.UserHomeDir
)

type windowsSetupOpts struct {
	Distro         string
	InstallCommand string
	NonInteractive bool
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
	fmt.Fprint(windowsStdout, `dockpipe windows — Windows + WSL2 bridge utilities.

Usage:
  dockpipe windows setup [--distro <name>] [--install-command <cmd>] [--non-interactive]
  dockpipe windows doctor

Examples:
  dockpipe windows setup
  dockpipe windows setup --distro Ubuntu --install-command "curl -fsSL https://example/install.sh | sh" --non-interactive

Other dockpipe commands from Windows (dockpipe.exe) forward into WSL automatically; only "windows …" runs on the host.
`)
}

func cmdWindowsDoctor() error {
	if windowsGoosFn() != "windows" {
		return fmt.Errorf("windows commands must run on Windows host")
	}
	distros, err := listWSLDistros()
	if err != nil {
		return err
	}
	if len(distros) == 0 {
		return fmt.Errorf("no WSL distros found. Install one via `wsl --install -d Ubuntu` first")
	}
	fmt.Fprintf(windowsStdout, "[dockpipe] WSL distros found: %s\n", strings.Join(distros, ", "))
	cfg, _ := loadWindowsConfig()
	if cfg != "" {
		fmt.Fprintf(windowsStdout, "[dockpipe] Default configured distro: %s\n", cfg)
	}
	return nil
}

func cmdWindowsSetup(args []string) error {
	if windowsGoosFn() != "windows" {
		return fmt.Errorf("windows setup must run on Windows host")
	}
	opts, err := parseWindowsSetupArgs(args)
	if err != nil {
		return err
	}
	distros, err := listWSLDistros()
	if err != nil {
		return err
	}
	if len(distros) == 0 {
		return fmt.Errorf("no WSL distros found. Install one via `wsl --install -d Ubuntu` first")
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
		return fmt.Errorf("dockpipe not found inside WSL distro %q. Re-run setup with --install-command to install it", chosen)
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
		default:
			return o, fmt.Errorf("unknown option %s", a)
		}
	}
	return o, nil
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
