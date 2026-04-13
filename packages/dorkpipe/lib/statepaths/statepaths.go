package statepaths

import (
	"path/filepath"

	"dockpipe/src/lib/infrastructure"
)

const dorkpipeScope = "dorkpipe"

func PackageStateDir(workdir string) (string, error) {
	return infrastructure.PackageStateDir(workdir, dorkpipeScope)
}

func EditArtifactsDir(workdir, requestID string) (string, error) {
	root, err := PackageStateDir(workdir)
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "edit", requestID), nil
}

func ReasoningArtifactsDir(workdir, requestID string) (string, error) {
	root, err := PackageStateDir(workdir)
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "reasoning", requestID), nil
}

func MetricsPath(workdir string) (string, error) {
	root, err := PackageStateDir(workdir)
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "metrics.jsonl"), nil
}

func RunPath(workdir string) (string, error) {
	root, err := PackageStateDir(workdir)
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "run.json"), nil
}

func NodesDir(workdir string) (string, error) {
	root, err := PackageStateDir(workdir)
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "nodes"), nil
}
