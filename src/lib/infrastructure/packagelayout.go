package infrastructure

import (
	"os"
	"path/filepath"
	"strings"
)

// Project-local layout for content installed from a package store (S3/R2, HTTPS).
// Uncompressed dev trees stay under templates/ and workflows/ as today; packaged installs default here.
//
// Default root: <workdir>/bin/.dockpipe/internal/packages (under bin/ so a common `bin/` gitignore applies):
//
//	workflows/   — dockpipe-workflow-*.tar.gz
//	core/        — compiled spine only: runtimes/, strategies/, assets/ (not resolvers/bundles/workflows)
//	resolvers/   — one package dir per resolver profile (same layout as templates/core/resolvers/<name>/)
//	assets/      — optional top-level shared assets (e.g. large binary packs)
//
// Override with DOCKPIPE_PACKAGES_ROOT (absolute path, or relative to workdir).
//
// User-wide installs (optional): GlobalDockpipeDataDir()/packages/{workflows,resolvers,core,...} — see
// `dockpipe install core --global` and infrastructure/globaldirs.go.
const PackageManifestFilename = "package.yml"

// DockpipeDirRel is the project-relative root for materialized DockPipe state (compiled packages,
// tarball cache, host runs, cleanup markers). Placed under bin/ so typical .gitignore `bin/` rules
// cover compile output without a separate `.dockpipe/` entry.
var (
	DockpipeDirRel = filepath.Join("bin", ".dockpipe")
	PackagesDirRel = filepath.Join(DockpipeDirRel, "internal", "packages")
)

// envPackagesRoot is the optional override for the packages root directory.
const envPackagesRoot = "DOCKPIPE_PACKAGES_ROOT"

// PackagesRoot returns the absolute directory for installed packages (default: workdir/bin/.dockpipe/internal/packages).
func PackagesRoot(workdir string) (string, error) {
	if v := strings.TrimSpace(os.Getenv(envPackagesRoot)); v != "" {
		if filepath.IsAbs(v) {
			return filepath.Clean(v), nil
		}
		wd, err := absHostWorkdir(workdir)
		if err != nil {
			return "", err
		}
		return filepath.Clean(filepath.Join(wd, v)), nil
	}
	wd, err := absHostWorkdir(workdir)
	if err != nil {
		return "", err
	}
	return filepath.Join(wd, PackagesDirRel), nil
}

// PackagesWorkflowsDir is bin/.dockpipe/internal/packages/workflows (or under DOCKPIPE_PACKAGES_ROOT).
func PackagesWorkflowsDir(workdir string) (string, error) {
	root, err := PackagesRoot(workdir)
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "workflows"), nil
}

// PackagesCoreDir is bin/.dockpipe/internal/packages/core — compiled runtimes, strategies, assets only.
func PackagesCoreDir(workdir string) (string, error) {
	root, err := PackagesRoot(workdir)
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "core"), nil
}

// PackagesResolversDir is bin/.dockpipe/internal/packages/resolvers — one subdirectory per resolver package.
func PackagesResolversDir(workdir string) (string, error) {
	root, err := PackagesRoot(workdir)
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "resolvers"), nil
}

// PackagesAssetsDir is bin/.dockpipe/internal/packages/assets (optional).
func PackagesAssetsDir(workdir string) (string, error) {
	root, err := PackagesRoot(workdir)
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "assets"), nil
}
