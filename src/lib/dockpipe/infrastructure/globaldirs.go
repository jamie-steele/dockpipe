package infrastructure

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// envGlobalRoot overrides GlobalDockpipeDataDir (absolute path recommended).
const envGlobalRoot = "DOCKPIPE_GLOBAL_ROOT"

// GlobalDockpipeDataDir returns the OS-appropriate root for user-wide DockPipe data:
//   - Windows: %LOCALAPPDATA%\dockpipe (or %USERPROFILE%\AppData\Local\dockpipe if LOCALAPPDATA unset)
//   - macOS:   ~/Library/Application Support/dockpipe
//   - other:   $XDG_DATA_HOME/dockpipe, else ~/.local/share/dockpipe
//
// Override with DOCKPIPE_GLOBAL_ROOT for tests or custom layouts.
func GlobalDockpipeDataDir() (string, error) {
	if v := strings.TrimSpace(os.Getenv(envGlobalRoot)); v != "" {
		return filepath.Abs(filepath.Clean(v))
	}
	switch runtime.GOOS {
	case "windows":
		d := os.Getenv("LOCALAPPDATA")
		if d == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				return "", fmt.Errorf("global dockpipe dir: %w", err)
			}
			d = filepath.Join(home, "AppData", "Local")
		}
		return filepath.Join(d, "dockpipe"), nil
	case "darwin":
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("global dockpipe dir: %w", err)
		}
		return filepath.Join(home, "Library", "Application Support", "dockpipe"), nil
	default:
		if xdg := strings.TrimSpace(os.Getenv("XDG_DATA_HOME")); xdg != "" {
			return filepath.Join(filepath.Clean(xdg), "dockpipe"), nil
		}
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("global dockpipe dir: %w", err)
		}
		return filepath.Join(home, ".local", "share", "dockpipe"), nil
	}
}

// GlobalTemplatesCoreDir is where `dockpipe install core --global` extracts templates/core
// (same layout as a project: <root>/templates/core).
func GlobalTemplatesCoreDir() (string, error) {
	root, err := GlobalDockpipeDataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "templates", "core"), nil
}

// GlobalPackagesRoot holds global package trees: packages/{workflows,resolvers,core}/...
// (flat under the global data dir — not under .dockpipe/internal).
func GlobalPackagesRoot() (string, error) {
	root, err := GlobalDockpipeDataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "packages"), nil
}

// GlobalPackagesWorkflowsDir is global packages/workflows (named workflow dirs with config.yml).
func GlobalPackagesWorkflowsDir() (string, error) {
	root, err := GlobalPackagesRoot()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "workflows"), nil
}

// GlobalPackagesResolversDir is global packages/resolvers.
func GlobalPackagesResolversDir() (string, error) {
	root, err := GlobalPackagesRoot()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "resolvers"), nil
}
