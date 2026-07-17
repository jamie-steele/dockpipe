package application

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"dockpipe/src/lib/domain"
	"dockpipe/src/lib/infrastructure"
)

// RunPackageBuildSourceFromFlags runs package-owned source build scripts for package authoring trees.
// Installed tarballs should already contain the artifacts they ship; this is source-checkout only behavior.
func RunPackageBuildSourceFromFlags(workdir, only string) (int, error) {
	root, err := filepath.Abs(filepath.Clean(workdir))
	if err != nil {
		return 0, err
	}
	targets, err := discoverPackageSourceBuildTargets(root, only)
	if err != nil {
		return 0, err
	}
	ids := mergeOperationResultIDs(buildOperationIDs(root, ""), map[string]string{
		"count": strconv.Itoa(len(targets)),
	})
	if strings.TrimSpace(only) != "" {
		ids["package"] = strings.TrimSpace(only)
	}
	if len(targets) == 0 {
		infrastructure.LogOperationResult(os.Stderr, infrastructure.OperationResult{
			Unit:       "build.source.packages",
			Status:     infrastructure.OperationStatusDone,
			DurationMs: 0,
			IDs:        ids,
		})
		return 0, nil
	}
	if err := infrastructure.RunOperationWithOptions(os.Stderr, "build.source.packages", "Running package source builds…", ids, infrastructure.OperationOptions{Spinner: false, ProgressEvery: 5 * time.Second}, func() error {
		for _, target := range targets {
			targetIDs := mergeOperationResultIDs(buildOperationIDs(root, ""), map[string]string{
				"package": target.Name,
				"script":  filepath.ToSlash(target.ScriptRel),
			})
			if err := infrastructure.RunOperationWithOptions(os.Stderr, "build.source.package", "Running package source build…", targetIDs, infrastructure.OperationOptions{Spinner: false, ProgressEvery: 5 * time.Second}, func() error {
				if err := runPackageScriptTarget(root, target, packageSourceBuildEnv(root, target), "build.source.script"); err != nil {
					return fmt.Errorf("package %q source build: %w", target.Name, err)
				}
				return nil
			}); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return 0, err
	}
	return len(targets), nil
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
	dockpipeBin, _ := resolveDockpipeBinForChildProcess(workdir)
	return []string{
		"DOCKPIPE_SOURCE_BUILD=1",
		"DOCKPIPE_PACKAGE_SOURCE_BUILD_SCRIPT=" + target.ScriptRel,
		"DOCKPIPE_BIN=" + dockpipeBin,
	}
}
