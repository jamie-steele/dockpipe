package infrastructure

import (
	"os"
	"path/filepath"
)

// ShipyardDir is the top-level directory name for the materialized bundle layout
// (…/core/…, …/workflows/…) under RepoRoot. Authoring checkouts use this name for repo-local
// maintainer workflows (CI, dogfood, quick iteration) — distinct from stable templates/ and future CDN bundles.
const ShipyardDir = "shipyard"

// EmbeddedTemplatesPrefix is the path prefix of bundled workflow/core files inside dockpipe.BundledFS (see embed.go).
const EmbeddedTemplatesPrefix = "src/templates"

// authoringTemplatesRoot returns the directory containing workflow dirs and core/ for authoring checkouts.
// Prefers src/templates when src/templates/core exists (dockpipe source tree); otherwise templates/ (user projects from dockpipe init).
func authoringTemplatesRoot(repoRoot string) string {
	src := filepath.Join(repoRoot, "src", "templates")
	if st, err := os.Stat(filepath.Join(src, "core")); err == nil && st.IsDir() {
		return src
	}
	return filepath.Join(repoRoot, "templates")
}

// UsesBundledAssetLayout reports whether repoRoot is the materialized embedded bundle:
// it contains …/core (and typically …/workflows). Authoring checkouts use
// src/templates/ or templates/ and …/core/ instead.
func UsesBundledAssetLayout(repoRoot string) bool {
	p := filepath.Join(repoRoot, ShipyardDir, "core")
	st, err := os.Stat(p)
	return err == nil && st.IsDir()
}

// CoreDir returns .../src/templates/core or .../templates/core (authoring) or .../<ShipyardDir>/core (materialized bundle).
func CoreDir(repoRoot string) string {
	if UsesBundledAssetLayout(repoRoot) {
		return filepath.Join(repoRoot, ShipyardDir, "core")
	}
	return filepath.Join(authoringTemplatesRoot(repoRoot), "core")
}

// WorkflowsRootDir returns .../src/templates or .../templates (authoring) or .../<ShipyardDir>/workflows (materialized bundle).
func WorkflowsRootDir(repoRoot string) string {
	if UsesBundledAssetLayout(repoRoot) {
		return filepath.Join(repoRoot, ShipyardDir, "workflows")
	}
	return authoringTemplatesRoot(repoRoot)
}

// AuthoringShipyardWorkflowsDir is .../<ShipyardDir>/workflows on a git checkout (not the materialized bundle).
// Repo-local maintainer workflows live here — not under src/templates/ or templates/. User-facing examples stay under the authoring templates root.
func AuthoringShipyardWorkflowsDir(repoRoot string) string {
	return filepath.Join(repoRoot, ShipyardDir, "workflows")
}
