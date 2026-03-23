package infrastructure

import (
	"os"
	"path/filepath"

	"dockpipe/src/lib/dockpipe/domain"
)

// LoadWorkflow reads a workflow YAML file from disk and parses it (including imports:).
func LoadWorkflow(path string) (*domain.Workflow, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	baseDir := filepath.Dir(path)
	return domain.ParseWorkflowFromDisk(data, baseDir, os.ReadFile)
}
