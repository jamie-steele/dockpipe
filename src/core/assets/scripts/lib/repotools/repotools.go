package repotools

import (
	"os"
	"os/exec"
	"path/filepath"
)

type SDK struct {
	Workdir      string
	DockpipeBin  string
	WorkflowName string
	ScriptDir    string
	PackageRoot  string
	AssetsDir    string
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

func Load(root string) (SDK, error) {
	workdir, err := RepoRoot(root)
	if err != nil {
		return SDK{}, err
	}
	dockpipeBin, _ := ResolveDockpipeBin(workdir)
	return SDK{
		Workdir:      workdir,
		DockpipeBin:  dockpipeBin,
		WorkflowName: os.Getenv("DOCKPIPE_WORKFLOW_NAME"),
		ScriptDir:    os.Getenv("DOCKPIPE_SCRIPT_DIR"),
		PackageRoot:  os.Getenv("DOCKPIPE_PACKAGE_ROOT"),
		AssetsDir:    os.Getenv("DOCKPIPE_ASSETS_DIR"),
	}, nil
}
