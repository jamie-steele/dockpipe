package infrastructure

import (
	"os"
	"path/filepath"
	"strings"
)

// Project-local layout for content installed from a package store (S3/R2, HTTPS).
// Uncompressed dev trees stay under templates/ and workflows/ as today; packaged installs default here.
//
// Default root: <workdir>/.dockpipe/internal/packages:
//
//	workflows/   — workflow-shaped dirs (config.yml, steps, …)
//	core/        — compiled spine only: runtimes/, strategies/, assets/ (not resolvers/bundles/workflows)
//	resolvers/   — one package dir per resolver profile (same layout as templates/core/resolvers/<name>/)
//	bundles/     — one package dir per bundle
//	assets/      — optional top-level shared assets (e.g. large binary packs)
//
// Override with DOCKPIPE_PACKAGES_ROOT (absolute path, or relative to workdir).
const (
	DockpipeDirRel          = ".dockpipe"
	PackagesDirRel          = ".dockpipe/internal/packages"
	PackageManifestFilename = "package.yml"
)

// envPackagesRoot is the optional override for the packages root directory.
const envPackagesRoot = "DOCKPIPE_PACKAGES_ROOT"

// PackagesRoot returns the absolute directory for installed packages (default: workdir/.dockpipe/internal/packages).
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

// PackagesWorkflowsDir is .dockpipe/internal/packages/workflows (or under DOCKPIPE_PACKAGES_ROOT).
func PackagesWorkflowsDir(workdir string) (string, error) {
	root, err := PackagesRoot(workdir)
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "workflows"), nil
}

// PackagesCoreDir is .dockpipe/internal/packages/core — compiled runtimes, strategies, assets only.
func PackagesCoreDir(workdir string) (string, error) {
	root, err := PackagesRoot(workdir)
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "core"), nil
}

// PackagesResolversDir is .dockpipe/internal/packages/resolvers — one subdirectory per resolver package.
func PackagesResolversDir(workdir string) (string, error) {
	root, err := PackagesRoot(workdir)
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "resolvers"), nil
}

// PackagesBundlesDir is .dockpipe/internal/packages/bundles — one subdirectory per bundle package.
func PackagesBundlesDir(workdir string) (string, error) {
	root, err := PackagesRoot(workdir)
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "bundles"), nil
}

// PackagesAssetsDir is .dockpipe/internal/packages/assets (optional top-level asset packs).
func PackagesAssetsDir(workdir string) (string, error) {
	root, err := PackagesRoot(workdir)
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "assets"), nil
}
