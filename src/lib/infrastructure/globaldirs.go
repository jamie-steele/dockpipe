package infrastructure

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

// envGlobalRoot overrides GlobalDockpipeDataDir (absolute path recommended).
const envGlobalRoot = "DOCKPIPE_GLOBAL_ROOT"

// envSystemRoot overrides the system-shared DockPipe data root used by OS package installs.
const envSystemRoot = "DOCKPIPE_SYSTEM_ROOT"

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
// (flat under the global data dir — not under bin/.dockpipe/internal).
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

// SystemDockpipeDataDirs returns candidate system-shared DockPipe data roots used by
// OS package installs. These are searched after the per-user global root.
//
// Override with DOCKPIPE_SYSTEM_ROOT for tests or custom layouts.
func SystemDockpipeDataDirs() []string {
	if v := strings.TrimSpace(os.Getenv(envSystemRoot)); v != "" {
		if abs, err := filepath.Abs(filepath.Clean(v)); err == nil {
			return []string{abs}
		}
		return []string{filepath.Clean(v)}
	}
	switch runtime.GOOS {
	case "windows":
		if d := strings.TrimSpace(os.Getenv("ProgramData")); d != "" {
			return []string{filepath.Join(d, "dockpipe")}
		}
		return []string{filepath.Join(string(filepath.Separator), "ProgramData", "dockpipe")}
	case "darwin":
		return []string{filepath.Join(string(filepath.Separator), "Library", "Application Support", "dockpipe")}
	default:
		return []string{
			filepath.Join(string(filepath.Separator), "usr", "local", "share", "dockpipe"),
			filepath.Join(string(filepath.Separator), "usr", "share", "dockpipe"),
		}
	}
}

// SystemPackagesRoots returns candidate system-shared package roots.
func SystemPackagesRoots() []string {
	return uniqueExistingLikeDirs(SystemDockpipeDataDirs(), "packages")
}

// SystemPackagesCoreDirs returns candidate system-shared packages/core directories.
func SystemPackagesCoreDirs() []string {
	return uniqueExistingLikeDirs(SystemPackagesRoots(), "core")
}

// SystemPackagesWorkflowsDirs returns candidate system-shared packages/workflows directories.
func SystemPackagesWorkflowsDirs() []string {
	return uniqueExistingLikeDirs(SystemPackagesRoots(), "workflows")
}

// SystemPackagesResolversDirs returns candidate system-shared packages/resolvers directories.
func SystemPackagesResolversDirs() []string {
	return uniqueExistingLikeDirs(SystemPackagesRoots(), "resolvers")
}

// SystemTemplatesCoreDirs returns candidate system-shared templates/core directories.
func SystemTemplatesCoreDirs() []string {
	return uniqueExistingLikeDirs(SystemDockpipeDataDirs(), filepath.Join("templates", "core"))
}

func uniqueExistingLikeDirs(roots []string, suffix string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(roots))
	for _, root := range roots {
		root = strings.TrimSpace(root)
		if root == "" {
			continue
		}
		p := filepath.Clean(filepath.Join(root, suffix))
		if _, ok := seen[p]; ok {
			continue
		}
		seen[p] = struct{}{}
		out = append(out, p)
	}
	sort.Strings(out)
	return out
}

// GlobalImagesRoot holds user-wide image artifact records and indexes.
// Docker layers still live in the Docker daemon/registry; this directory is DockPipe metadata only.
func GlobalImagesRoot() (string, error) {
	root, err := GlobalDockpipeDataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "images"), nil
}

// GlobalImageArtifactIndexDir holds global image artifact index files.
func GlobalImageArtifactIndexDir() (string, error) {
	return GlobalImagesRoot()
}

// GlobalImageArtifactByFingerprintDir holds global image artifact records keyed by fingerprint.
func GlobalImageArtifactByFingerprintDir() (string, error) {
	root, err := GlobalImagesRoot()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "by-fingerprint"), nil
}

// GlobalCacheRoot holds user-wide cache metadata such as downloaded tarballs.
func GlobalCacheRoot() (string, error) {
	root, err := GlobalDockpipeDataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "cache"), nil
}
