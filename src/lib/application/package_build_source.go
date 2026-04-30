package application

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"dockpipe/src/lib/domain"
)

// RunPackageBuildSourceFromFlags runs package-owned source build scripts for package authoring trees.
// Installed tarballs should already contain the artifacts they ship; this is source-checkout only behavior.
func RunPackageBuildSourceFromFlags(workdir, only string) error {
	root, err := filepath.Abs(filepath.Clean(workdir))
	if err != nil {
		return err
	}
	targets, err := discoverPackageSourceBuildTargets(root, only)
	if err != nil {
		return err
	}
	if len(targets) == 0 {
		if strings.TrimSpace(only) != "" {
			fmt.Fprintf(os.Stderr, "[dockpipe] package build source: no source-build package matched %q\n", strings.TrimSpace(only))
		}
		return nil
	}
	for _, target := range targets {
		fmt.Fprintf(os.Stderr, "[dockpipe] package build source: %s (%s)\n", target.Name, target.ScriptRel)
		if err := runPackageScriptTarget(root, target, packageSourceBuildEnv(root, target), "build.source.script"); err != nil {
			return fmt.Errorf("package %q source build: %w", target.Name, err)
		}
	}
	return nil
}

func discoverPackageSourceBuildTargets(workdir, only string) ([]packageScriptTarget, error) {
	return discoverPackageScriptTargets(workdir, only, func(manifest *domain.PackageManifest) string {
		if manifest.Build.Source == nil {
			return ""
		}
		return manifest.Build.Source.Script
	})
}

func packageSourceBuildEnv(workdir string, target packageScriptTarget) []string {
	dockpipeBin, _ := resolveDockpipeBinForSDK(workdir)
	return []string{
		"DOCKPIPE_SOURCE_BUILD=1",
		"DOCKPIPE_PACKAGE_SOURCE_BUILD_SCRIPT=" + target.ScriptRel,
		"DOCKPIPE_BIN=" + dockpipeBin,
	}
}
