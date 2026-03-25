package application

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"dockpipe/src/lib/dockpipe/infrastructure"
	"dockpipe/src/lib/dockpipe/infrastructure/packagebuild"
)

func cmdPackageBuild(args []string) error {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" {
		fmt.Print(packageBuildUsageText)
		return nil
	}
	switch args[0] {
	case "core":
		return cmdPackageBuildCore(args[1:])
	default:
		return fmt.Errorf("unknown package build target %q (try: dockpipe package build --help)", args[0])
	}
}

func cmdPackageBuildCore(args []string) error {
	if len(args) > 0 && (args[0] == "--help" || args[0] == "-h") {
		fmt.Print(packageBuildCoreUsageText)
		return nil
	}
	var (
		repoRoot string
		outDir   string
		version  string
	)
	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "--repo-root" && i+1 < len(args):
			repoRoot = args[i+1]
			i++
		case args[i] == "--out" && i+1 < len(args):
			outDir = args[i+1]
			i++
		case args[i] == "--version" && i+1 < len(args):
			version = args[i+1]
			i++
		case strings.HasPrefix(args[i], "-"):
			return fmt.Errorf("unknown option %s (try: dockpipe package build core --help)", args[i])
		default:
			return fmt.Errorf("unexpected argument %q", args[i])
		}
	}
	root, err := resolvePackageRepoRoot(repoRoot)
	if err != nil {
		return err
	}
	coreParent, err := resolveTemplatesCoreParent(root)
	if err != nil {
		return err
	}
	if outDir == "" {
		outDir = filepath.Join(root, infrastructure.DefaultRepoArtifactsDir)
	} else {
		outDir = filepath.Clean(outDir)
		if !filepath.IsAbs(outDir) {
			outDir = filepath.Join(root, outDir)
		}
	}
	if version == "" {
		v, err := readRepoVersion(root)
		if err != nil {
			return err
		}
		version = v
	}
	path, err := packagebuild.WriteCoreRelease(coreParent, outDir, version)
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "[dockpipe] wrote %s (+ .sha256, install-manifest.json) under %s\n", filepath.Base(path), filepath.Dir(path))
	return nil
}

func resolvePackageRepoRoot(flagRoot string) (string, error) {
	if flagRoot != "" {
		return filepath.Abs(filepath.Clean(flagRoot))
	}
	if v := os.Getenv("DOCKPIPE_REPO_ROOT"); v != "" {
		return filepath.Abs(filepath.Clean(v))
	}
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	if top, err := infrastructure.GitTopLevel(wd); err == nil {
		return top, nil
	}
	return filepath.Abs(wd)
}

func resolveTemplatesCoreParent(repoRoot string) (string, error) {
	repoRoot = filepath.Clean(repoRoot)
	a := filepath.Join(repoRoot, "src", "core", "runtimes")
	if st, err := os.Stat(a); err == nil && st.IsDir() {
		return filepath.Join(repoRoot, "src"), nil
	}
	b := filepath.Join(repoRoot, "templates", "core")
	if st, err := os.Stat(b); err == nil && st.IsDir() {
		return filepath.Join(repoRoot, "templates"), nil
	}
	return "", fmt.Errorf("no templates/core under %q (expected src/core with runtimes/ or templates/core)", repoRoot)
}

func readRepoVersion(repoRoot string) (string, error) {
	b, err := os.ReadFile(filepath.Join(repoRoot, "VERSION"))
	if err != nil {
		return "", fmt.Errorf("read VERSION: %w (use --version)", err)
	}
	v := strings.TrimSpace(string(b))
	if v == "" {
		return "", fmt.Errorf("VERSION is empty (use --version)")
	}
	return v, nil
}

const packageBuildUsageText = `dockpipe package build

Author release artifacts (tarball + checksum + install-manifest) for self-hosted
package sources. Official DockPipe releases use a separate pipeline; this matches
the layout expected by dockpipe install core and scripts/dockpipe/package-templates-core.sh.

Usage:
  dockpipe package build core [options]

`

const packageBuildCoreUsageText = `dockpipe package build core

Writes templates-core-<version>.tar.gz under release/artifacts (or --out), matching .sha256, and install-manifest.json
with schema 1 and packages.core (version, tarball name, sha256).

Options:
  --repo-root <path>   Repository root (default: DOCKPIPE_REPO_ROOT, else git top-level, else cwd)
  --out <dir>          Output directory (default: <repo-root>/release/artifacts)
  --version <ver>      Version string (default: trim contents of VERSION at repo root)

`
