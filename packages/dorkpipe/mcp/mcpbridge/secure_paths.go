package mcpbridge

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"dockpipe/src/lib/infrastructure"
)

func effectiveRepoRoot() (string, error) {
	if v := strings.TrimSpace(os.Getenv("DOCKPIPE_MCP_REPO_ROOT")); v != "" {
		return filepath.Abs(v)
	}
	if v := strings.TrimSpace(os.Getenv("DOCKPIPE_WORKDIR")); v != "" {
		return filepath.Abs(v)
	}
	return infrastructure.RepoRoot()
}

// ResolvePathUnderRepoRoot resolves user-supplied paths for specs and validation targets.
// Absolute paths must still lie under the resolved project root.
func ResolvePathUnderRepoRoot(userPath string) (string, error) {
	userPath = strings.TrimSpace(userPath)
	if userPath == "" {
		return "", fmt.Errorf("path is empty")
	}
	if strings.Contains(userPath, "\x00") {
		return "", fmt.Errorf("invalid path")
	}
	rr, err := effectiveRepoRoot()
	if err != nil {
		return "", err
	}
	root := filepath.Clean(rr)
	var p string
	if filepath.IsAbs(userPath) {
		p = filepath.Clean(userPath)
	} else {
		p = filepath.Clean(filepath.Join(root, userPath))
	}
	rel, err := filepath.Rel(root, p)
	if err != nil {
		return "", fmt.Errorf("path outside repo root")
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("path escapes repo root")
	}
	return p, nil
}

// CheckAbsolutePathUnderRepoRoot returns nil if absPath is contained in the resolved repo root.
// Used to constrain MCP exec tool workdirs (default: restriction on; opt out with DOCKPIPE_MCP_RESTRICT_WORKDIR=0).
func CheckAbsolutePathUnderRepoRoot(absPath string) error {
	absPath = filepath.Clean(absPath)
	if strings.Contains(absPath, "\x00") {
		return fmt.Errorf("invalid path")
	}
	rr, err := effectiveRepoRoot()
	if err != nil {
		return err
	}
	root := filepath.Clean(rr)
	rel, err := filepath.Rel(root, absPath)
	if err != nil {
		return fmt.Errorf("path outside repo root")
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return fmt.Errorf("path escapes repo root")
	}
	return nil
}
