package mcpbridge

import (
	"dockpipe/src/lib/dockpipe/infrastructure"
)

// listWorkflowNames uses the same resolution as the dockpipe CLI (bundled / authoring roots).
func listWorkflowNames() ([]string, error) {
	rr, err := infrastructure.RepoRoot()
	if err != nil {
		return nil, err
	}
	return infrastructure.ListWorkflowNamesInRepoRoot(rr)
}
