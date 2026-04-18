package repotools

import (
	"os"
	"os/exec"
	"path/filepath"
)

type SDK struct {
	Workdir     string
	DockpipeBin string
	DorkpipeBin string
	WorkflowName string
}

func RepoRoot(root string) (string, error) {
	if root == "" {
		root = os.Getenv("DOCKPIPE_WORKDIR")
	}
	if root == "" {
		var err error
		root, err = os.Getwd()
		if err != nil {
			return "", err
		}
	}
	return filepath.Abs(root)
}

func ResolveDockpipeBin(root string) (string, error) {
	if configured := os.Getenv("DOCKPIPE_BIN"); configured != "" {
		return configured, nil
	}
	resolvedRoot, err := RepoRoot(root)
	if err != nil {
		return "", err
	}
	candidate := filepath.Join(resolvedRoot, "src", "bin", "dockpipe")
	if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
		return candidate, nil
	}
	return exec.LookPath("dockpipe")
}

func ResolveDorkpipeBin(root string) (string, error) {
	if configured := os.Getenv("DORKPIPE_BIN"); configured != "" {
		return configured, nil
	}
	resolvedRoot, err := RepoRoot(root)
	if err != nil {
		return "", err
	}
	candidate := filepath.Join(resolvedRoot, "packages", "dorkpipe", "bin", "dorkpipe")
	if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
		return candidate, nil
	}
	return exec.LookPath("dorkpipe")
}

func Load(root string) (SDK, error) {
	workdir, err := RepoRoot(root)
	if err != nil {
		return SDK{}, err
	}
	dockpipeBin, _ := ResolveDockpipeBin(workdir)
	dorkpipeBin, _ := ResolveDorkpipeBin(workdir)
	return SDK{
		Workdir:      workdir,
		DockpipeBin:  dockpipeBin,
		DorkpipeBin:  dorkpipeBin,
		WorkflowName: os.Getenv("DOCKPIPE_WORKFLOW_NAME"),
	}, nil
}
