package mcpbridge

import (
	"os"

	"dockpipe/src/lib/dockpipe/infrastructure"
)

// listWorkflowNames uses the same resolution as the dockpipe CLI (bundled / authoring roots + installed packages).
func listWorkflowNames() ([]string, error) {
	rr, err := infrastructure.RepoRoot()
	if err != nil {
		return nil, err
	}
	wd := os.Getenv("DOCKPIPE_WORKDIR")
	if wd == "" {
		wd, _ = os.Getwd()
	}
	return infrastructure.ListWorkflowNamesInRepoRootAndPackages(rr, wd)
}
