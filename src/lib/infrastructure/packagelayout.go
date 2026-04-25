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
const (
	envPackagesRoot    = "DOCKPIPE_PACKAGES_ROOT"
	EnvStateDir        = "DOCKPIPE_STATE_DIR"
	EnvPackageID       = "DOCKPIPE_PACKAGE_ID"
	EnvPackageStateDir = "DOCKPIPE_PACKAGE_STATE_DIR"
)

// StateRoot returns the absolute project-local DockPipe state root (default: workdir/bin/.dockpipe).
func StateRoot(workdir string) (string, error) {
	wd, err := absHostWorkdir(workdir)
	if err != nil {
		return "", err
	}
	return filepath.Join(wd, DockpipeDirRel), nil
}

// StateInternalDir returns the absolute root for DockPipe-owned internal state
// (default: workdir/bin/.dockpipe/internal). New internal artifact families should
// derive from this helper instead of spelling ".dockpipe" paths by hand.
func StateInternalDir(workdir string) (string, error) {
	root, err := StateRoot(workdir)
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "internal"), nil
}

// StateInternalCacheDir returns the absolute root for internal caches
// (default: workdir/bin/.dockpipe/internal/cache).
func StateInternalCacheDir(workdir string) (string, error) {
	root, err := StateInternalDir(workdir)
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "cache"), nil
}

// ImageArtifactCacheDir returns the absolute root for cached image artifact records
// (default: workdir/bin/.dockpipe/internal/cache/images).
func ImageArtifactCacheDir(workdir string) (string, error) {
	root, err := StateInternalCacheDir(workdir)
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "images"), nil
}

// ImageArtifactIndexDir returns the absolute root for future image artifact indexes
// (default: workdir/bin/.dockpipe/internal/images).
func ImageArtifactIndexDir(workdir string) (string, error) {
	root, err := StateInternalDir(workdir)
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "images"), nil
}

// SanitizePackageStateScope reduces package/workflow/resolver ids into a stable filesystem segment.
func SanitizePackageStateScope(scope string) string {
	scope = strings.TrimSpace(strings.ToLower(scope))
	if scope == "" {
		return "default"
	}
	var b strings.Builder
	b.Grow(len(scope))
	lastDash := false
	for _, r := range scope {
		keep := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')
		if keep {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		switch r {
		case '.', '_', '-', '/', '\\', ' ':
			if !lastDash {
				b.WriteByte('-')
				lastDash = true
			}
		default:
			if !lastDash {
				b.WriteByte('-')
				lastDash = true
			}
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		return "default"
	}
	return out
}

// PackageStateDir returns the absolute package-scoped state root under bin/.dockpipe/packages/<scope>.
func PackageStateDir(workdir, scope string) (string, error) {
	root, err := StateRoot(workdir)
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "packages", SanitizePackageStateScope(scope)), nil
}

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
