package infrastructure

import (
	"os"

	"dockpipe/lib/dockpipe/domain"
)

// LoadWorkflow reads config.yml from disk and parses it as a workflow.
func LoadWorkflow(path string) (*domain.Workflow, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return domain.ParseWorkflowYAML(data)
}
