package infrastructure

import (
	"os"
	"path/filepath"
)

// RepoRoot returns the layout root for bundled assets: materialized bundle has dockpipe-experimental/core,
// dockpipe-experimental/workflows, lib/, and version; authoring checkouts use templates/ and templates/core/ instead.
// Projects may also have top-level scripts/ and images/ for local copies and custom Dockerfiles.
// By default this is the materialized embedded bundle in the user cache (see embed.go + bundled_extract.go).
// Set DOCKPIPE_REPO_ROOT to override (e.g. development against a git checkout).
func RepoRoot() (string, error) {
	if v := os.Getenv("DOCKPIPE_REPO_ROOT"); v != "" {
		return filepath.Abs(v)
	}
	return MaterializedBundledRoot()
}
