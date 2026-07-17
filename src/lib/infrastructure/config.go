package infrastructure

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"dockpipe/src/lib/domain"
)

// LoadWorkflow reads a workflow YAML file from disk or from a gzip tar (see tryResolveWorkflowTarballURI)
// and parses it (including imports:).
func LoadWorkflow(path string) (*domain.Workflow, error) {
	if strings.HasPrefix(path, "tar://") {
		tarPath, entry, ok := SplitTarWorkflowURI(path)
		if !ok {
			return nil, fmt.Errorf("invalid tar workflow URI")
		}
		return loadWorkflowFromTarball(tarPath, entry)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	baseDir := filepath.Dir(path)
	return domain.ParseWorkflowFromDisk(data, baseDir, os.ReadFile)
}
