package infrastructure

import (
	"os"
	"path/filepath"
)

// BundledDockpipeDir is the top-level directory name for the materialized bundle layout
// (…/core/…, …/workflows/…) under RepoRoot. Authoring checkouts use this name for repo-local
// experimental workflows (CI, dogfood, quick iteration) — distinct from stable templates/ and future CDN bundles.
const BundledDockpipeDir = "dockpipe-experimental"

// UsesBundledAssetLayout reports whether repoRoot is the materialized embedded bundle:
// it contains …/core (and typically …/workflows). Authoring checkouts use
// templates/ and templates/core/ instead.
func UsesBundledAssetLayout(repoRoot string) bool {
	p := filepath.Join(repoRoot, BundledDockpipeDir, "core")
	st, err := os.Stat(p)
	return err == nil && st.IsDir()
}

// CoreDir returns .../templates/core (authoring) or .../<BundledDockpipeDir>/core (materialized bundle).
func CoreDir(repoRoot string) string {
	if UsesBundledAssetLayout(repoRoot) {
		return filepath.Join(repoRoot, BundledDockpipeDir, "core")
	}
	return filepath.Join(repoRoot, "templates", "core")
}

// WorkflowsRootDir returns .../templates (authoring) or .../<BundledDockpipeDir>/workflows (materialized bundle).
func WorkflowsRootDir(repoRoot string) string {
	if UsesBundledAssetLayout(repoRoot) {
		return filepath.Join(repoRoot, BundledDockpipeDir, "workflows")
	}
	return filepath.Join(repoRoot, "templates")
}

// AuthoringDockpipeWorkflowsDir is .../<BundledDockpipeDir>/workflows on a git checkout (not the materialized bundle).
// Repo-local experimental workflows live here — not under templates/. User-facing examples stay under templates/.
func AuthoringDockpipeWorkflowsDir(repoRoot string) string {
	return filepath.Join(repoRoot, BundledDockpipeDir, "workflows")
}
