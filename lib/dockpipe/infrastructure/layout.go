package infrastructure

import (
	"os"
	"path/filepath"
)

// BundledDockpipeDir is the top-level directory name for the materialized bundle layout
// (dockpipe/core/..., dockpipe/workflows/...) under RepoRoot.
const BundledDockpipeDir = "dockpipe"

// UsesBundledAssetLayout reports whether repoRoot is the materialized embedded bundle:
// it contains dockpipe/core (and typically dockpipe/workflows). Authoring checkouts use
// templates/ and templates/core/ instead.
func UsesBundledAssetLayout(repoRoot string) bool {
	p := filepath.Join(repoRoot, BundledDockpipeDir, "core")
	st, err := os.Stat(p)
	return err == nil && st.IsDir()
}

// CoreDir returns .../templates/core (authoring) or .../dockpipe/core (materialized bundle).
func CoreDir(repoRoot string) string {
	if UsesBundledAssetLayout(repoRoot) {
		return filepath.Join(repoRoot, BundledDockpipeDir, "core")
	}
	return filepath.Join(repoRoot, "templates", "core")
}

// WorkflowsRootDir returns .../templates (authoring) or .../dockpipe/workflows (materialized bundle).
func WorkflowsRootDir(repoRoot string) string {
	if UsesBundledAssetLayout(repoRoot) {
		return filepath.Join(repoRoot, BundledDockpipeDir, "workflows")
	}
	return filepath.Join(repoRoot, "templates")
}

// AuthoringDockpipeWorkflowsDir is .../dockpipe/workflows on a git checkout (not the materialized bundle).
// Optional repo-local workflows (e.g. dockpipe init --dogfood-*) install here; bundled authoring stays under templates/.
func AuthoringDockpipeWorkflowsDir(repoRoot string) string {
	return filepath.Join(repoRoot, BundledDockpipeDir, "workflows")
}
