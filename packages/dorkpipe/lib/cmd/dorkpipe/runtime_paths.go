package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func resolveToolExecutable(envName, commandName string) (string, error) {
	if configured := strings.TrimSpace(os.Getenv(envName)); configured != "" {
		return configured, nil
	}
	if p, err := exec.LookPath(commandName); err == nil {
		return p, nil
	}
	return "", fmt.Errorf("%s not found; set %s or add %s to PATH", commandName, envName, commandName)
}

func resolveDockpipeExecutable() (string, error) {
	return resolveToolExecutable("DOCKPIPE_BIN", "dockpipe")
}

func resolvePipeonExecutable() (string, error) {
	return resolveToolExecutable("PIPEON_BIN", "pipeon")
}

func resolveDorkpipeScriptsDir() (string, error) {
	if configured := strings.TrimSpace(os.Getenv("DORKPIPE_SCRIPTS_DIR")); configured != "" {
		return configured, nil
	}
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("resolve dorkpipe scripts: %w", err)
	}
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		exe = filepath.Clean(exe)
	}
	candidates := []string{
		filepath.Join(filepath.Dir(exe), "..", "resolvers", "dorkpipe", "assets", "scripts"),
		filepath.Join(filepath.Dir(exe), "resolvers", "dorkpipe", "assets", "scripts"),
	}
	for _, candidate := range candidates {
		if st, err := os.Stat(candidate); err == nil && st.IsDir() {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("dorkpipe scripts not found; set DORKPIPE_SCRIPTS_DIR")
}

func resolveDorkpipeScriptPath(name string) (string, error) {
	dir, err := resolveDorkpipeScriptsDir()
	if err != nil {
		return "", err
	}
	p := filepath.Join(dir, filepath.Base(strings.TrimSpace(name)))
	if st, err := os.Stat(p); err == nil && !st.IsDir() {
		return p, nil
	}
	return "", fmt.Errorf("dorkpipe script %q not found under %s", name, dir)
}

func mustResolveDorkpipeScriptPath(name string) string {
	p, err := resolveDorkpipeScriptPath(name)
	if err != nil {
		panic(err)
	}
	return p
}
