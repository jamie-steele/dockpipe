package application

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"golang.org/x/term"
)

const (
	gitForWindowsURL = "https://git-scm.com/download/win"
	wslInstallURL    = "https://learn.microsoft.com/windows/wsl/install"
)

// ensureHostBash verifies `bash` is on PATH before any container run.
// dockpipe requires bash on the host (pre-scripts, tooling); on Windows without bash,
// interactive TTY may offer WSL re-exec.
func ensureHostBash() error {
	if _, err := exec.LookPath("bash"); err == nil {
		return nil
	}
	if windowsGoosFn() != "windows" {
		return fmt.Errorf(
			"bash not found in PATH — dockpipe requires bash on the host. Install bash (e.g. `bash` package on Linux; `/bin/bash` on macOS is usually present)",
		)
	}

	distros, err := listWSLDistros()
	if err != nil || len(distros) == 0 {
		return fmt.Errorf(
			"bash not found on PATH — dockpipe requires bash.\n"+
				"  • Install Git for Windows (includes bash): %s\n"+
				"  • Or install WSL2: %s — then set %s=1 and run `dockpipe windows setup`, or run dockpipe from a Linux install inside WSL",
			gitForWindowsURL, wslInstallURL, EnvUseWSLBridge,
		)
	}

	fd := int(os.Stdin.Fd())
	if !term.IsTerminal(fd) {
		return fmt.Errorf(
			"bash not found on PATH — dockpipe requires bash. Install Git for Windows: %s — or set %s=1 and use WSL (see `dockpipe windows setup`). WSL distros detected: %s",
			gitForWindowsURL, EnvUseWSLBridge, strings.Join(distros, ", "),
		)
	}

	fmt.Fprintf(os.Stderr, "[dockpipe] bash not found on PATH — dockpipe requires bash.\n\n")
	fmt.Fprintf(os.Stderr, "  • Install Git for Windows: %s\n", gitForWindowsURL)
	fmt.Fprintf(os.Stderr, "  • Or re-run through WSL so the Linux `dockpipe` in your distro runs (bash + git there).\n\n")
	fmt.Fprintf(os.Stderr, "Re-run this command through WSL? [y/N]: ")
	br := bufio.NewReader(os.Stdin)
	line, _ := br.ReadString('\n')
	line = strings.TrimSpace(strings.ToLower(line))
	if line != "y" && line != "yes" {
		return fmt.Errorf("install bash on Windows (e.g. Git for Windows): %s", gitForWindowsURL)
	}

	distro, err := chooseWSLDistro(distros, windowsSetupOpts{})
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "[dockpipe] Re-launching with %s=1 and distro %q (ensure `dockpipe` is installed in that distro — see `dockpipe windows setup`).\n", EnvUseWSLBridge, distro)
	return reexecWithWSLBridge(distro)
}

func reexecWithWSLBridge(distro string) error {
	if err := saveWindowsConfig(distro); err != nil {
		return fmt.Errorf("save WSL distro config: %w", err)
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	cmd := exec.Command(exe, os.Args[1:]...)
	cmd.Env = append(os.Environ(), EnvUseWSLBridge+"=1")
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	if err != nil {
		if x, ok := err.(*exec.ExitError); ok {
			os.Exit(x.ExitCode())
		}
		return err
	}
	os.Exit(0)
	return nil
}
