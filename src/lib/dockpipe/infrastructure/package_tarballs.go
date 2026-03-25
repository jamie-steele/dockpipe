package infrastructure

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"dockpipe/src/lib/dockpipe/infrastructure/packagebuild"
)

// TarballExtractCacheRoot is where full tarballs are extracted for tools that need real paths (scripts, resolver profiles).
func TarballExtractCacheRoot(repoRoot string) string {
	return filepath.Join(repoRoot, DockpipeDirRel, "internal", "cache", "tarball")
}

// FindLatestCoreTarball returns the newest matching dockpipe-core-*.tar.gz under packages/core (then global packages/core), or "" if none.
func FindLatestCoreTarball(workdir string) (string, error) {
	d, err := PackagesCoreDir(workdir)
	if err != nil {
		return "", err
	}
	if p, err := findLatestGlob(filepath.Join(d, "dockpipe-core-*.tar.gz")); err != nil {
		return "", err
	} else if p != "" {
		return p, nil
	}
	gp, err := GlobalPackagesRoot()
	if err != nil {
		return "", err
	}
	return findLatestGlob(filepath.Join(gp, "core", "dockpipe-core-*.tar.gz"))
}

// FindLatestWorkflowTarball returns the newest dockpipe-workflow-<name>-*.tar.gz under packages/workflows (then global).
func FindLatestWorkflowTarball(workdir, name string) (string, error) {
	d, err := PackagesWorkflowsDir(workdir)
	if err != nil {
		return "", err
	}
	tok := packagebuild.SafeTarballToken(name)
	pattern := filepath.Join(d, fmt.Sprintf("dockpipe-workflow-%s-*.tar.gz", tok))
	if p, err := findLatestGlob(pattern); err != nil {
		return "", err
	} else if p != "" {
		return p, nil
	}
	gp, err := GlobalPackagesRoot()
	if err != nil {
		return "", err
	}
	return findLatestGlob(filepath.Join(gp, "workflows", fmt.Sprintf("dockpipe-workflow-%s-*.tar.gz", tok)))
}

// FindLatestResolverTarball returns the newest dockpipe-resolver-<name>-*.tar.gz under packages/resolvers (then global).
func FindLatestResolverTarball(workdir, name string) (string, error) {
	d, err := PackagesResolversDir(workdir)
	if err != nil {
		return "", err
	}
	tok := packagebuild.SafeTarballToken(name)
	pattern := filepath.Join(d, fmt.Sprintf("dockpipe-resolver-%s-*.tar.gz", tok))
	if p, err := findLatestGlob(pattern); err != nil {
		return "", err
	} else if p != "" {
		return p, nil
	}
	gp, err := GlobalPackagesRoot()
	if err != nil {
		return "", err
	}
	return findLatestGlob(filepath.Join(gp, "resolvers", fmt.Sprintf("dockpipe-resolver-%s-*.tar.gz", tok)))
}

func findLatestGlob(pattern string) (string, error) {
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return "", err
	}
	if len(matches) == 0 {
		return "", nil
	}
	sort.Strings(matches)
	return matches[len(matches)-1], nil
}

// RemoveGlob removes files matching pattern (no directories).
func RemoveGlob(pattern string) error {
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return err
	}
	for _, p := range matches {
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			_ = os.Remove(p)
			_ = os.Remove(p + ".sha256")
		}
	}
	return nil
}

// RemoveLegacyPackageSubdirs deletes expanded package directories under base (migration from dir layout to tarballs only).
func RemoveLegacyPackageSubdirs(base string) error {
	entries, err := os.ReadDir(base)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		if err := os.RemoveAll(filepath.Join(base, e.Name())); err != nil {
			return err
		}
	}
	return nil
}
