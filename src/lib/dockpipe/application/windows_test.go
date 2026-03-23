package application

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func withWindowsSeams(t *testing.T) {
	t.Helper()
	oldGoos := windowsGoosFn
	oldStdin := windowsStdin
	oldStdout := windowsStdout
	oldStderr := windowsStderr
	oldHome := windowsUserHomeDirFn
	t.Cleanup(func() {
		windowsGoosFn = oldGoos
		windowsStdin = oldStdin
		windowsStdout = oldStdout
		windowsStderr = oldStderr
		windowsUserHomeDirFn = oldHome
	})
}

// TestParseWindowsSetupArgs parses dockpipe windows setup flags (distro, install command, non-interactive).
func TestParseWindowsSetupArgs(t *testing.T) {
	o, err := parseWindowsSetupArgs([]string{
		"--distro", "Ubuntu",
		"--install-command", "echo hi",
		"--non-interactive",
	})
	if err != nil {
		t.Fatalf("parseWindowsSetupArgs error: %v", err)
	}
	if o.Distro != "Ubuntu" || o.InstallCommand != "echo hi" || !o.NonInteractive {
		t.Fatalf("unexpected opts: %#v", o)
	}
}

// TestParseWindowsSetupArgsErrors on missing --distro value and unknown options.
func TestParseWindowsSetupArgsErrors(t *testing.T) {
	_, err := parseWindowsSetupArgs([]string{"--distro"})
	if err == nil || !strings.Contains(err.Error(), "--distro requires a value") {
		t.Fatalf("expected distro error, got %v", err)
	}
	_, err = parseWindowsSetupArgs([]string{"--unknown"})
	if err == nil || !strings.Contains(err.Error(), "unknown option") {
		t.Fatalf("expected unknown option error, got %v", err)
	}
}

// TestParseWSLDistroList parses wsl -l -q output into distro name lines.
func TestParseWSLDistroList(t *testing.T) {
	got := parseWSLDistroList("Ubuntu\n* Debian\r\n\n")
	if len(got) != 2 || got[0] != "Ubuntu" || got[1] != "Debian" {
		t.Fatalf("unexpected distros: %#v", got)
	}
}

// TestChooseWSLDistro selects distro by --distro, errors if non-interactive without distro, or reads stdin.
func TestChooseWSLDistro(t *testing.T) {
	withWindowsSeams(t)
	distros := []string{"Ubuntu", "Debian"}
	d, err := chooseWSLDistro(distros, windowsSetupOpts{Distro: "debian"})
	if err != nil || d != "Debian" {
		t.Fatalf("expected Debian selection, got distro=%q err=%v", d, err)
	}
	_, err = chooseWSLDistro(distros, windowsSetupOpts{NonInteractive: true})
	if err == nil || !strings.Contains(err.Error(), "requires --distro") {
		t.Fatalf("expected non-interactive error, got %v", err)
	}

	windowsStdin = strings.NewReader("2\n")
	windowsStdout = &bytes.Buffer{}
	d, err = chooseWSLDistro(distros, windowsSetupOpts{})
	if err != nil || d != "Debian" {
		t.Fatalf("expected interactive selection Debian, got distro=%q err=%v", d, err)
	}
}

// TestWindowsConfigSaveLoad round-trips default WSL distro under APPDATA.
func TestWindowsConfigSaveLoad(t *testing.T) {
	withWindowsSeams(t)
	appData := t.TempDir()
	t.Setenv("APPDATA", appData)
	if err := saveWindowsConfig("Ubuntu"); err != nil {
		t.Fatalf("saveWindowsConfig error: %v", err)
	}
	got, err := loadWindowsConfig()
	if err != nil {
		t.Fatalf("loadWindowsConfig error: %v", err)
	}
	if got != "Ubuntu" {
		t.Fatalf("expected Ubuntu, got %q", got)
	}
	p, err := windowsConfigPath()
	if err != nil {
		t.Fatalf("windowsConfigPath error: %v", err)
	}
	if _, err := os.Stat(p); err != nil {
		t.Fatalf("expected config file at %s: %v", p, err)
	}
}

// TestWindowsConfigPathFallsBackToHome when APPDATA is unset uses user home for config path.
func TestWindowsConfigPathFallsBackToHome(t *testing.T) {
	withWindowsSeams(t)
	t.Setenv("APPDATA", "")
	home := t.TempDir()
	windowsUserHomeDirFn = func() (string, error) { return home, nil }
	p, err := windowsConfigPath()
	if err != nil {
		t.Fatalf("windowsConfigPath error: %v", err)
	}
	want := filepath.Join(home, "AppData", "Roaming", "dockpipe", "windows-config.env")
	if p != want {
		t.Fatalf("windowsConfigPath mismatch: got %q want %q", p, want)
	}
}
