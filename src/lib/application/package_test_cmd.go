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

func cmdPackageTest(args []string) error {
	if len(args) > 0 && (args[0] == "--help" || args[0] == "-h") {
		fmt.Print(packageTestUsageText)
		return nil
	}
	var err error
	args, err = injectCompileWorkdirFromProjectConfig(args)
	if err != nil {
		return err
	}
	var (
		workdir string
		only    string
	)
	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "--workdir" && i+1 < len(args):
			workdir = args[i+1]
			i++
		case args[i] == "--only" && i+1 < len(args):
			only = args[i+1]
			i++
		case strings.HasPrefix(args[i], "-"):
			return fmt.Errorf("unknown option %s (try: dockpipe package test --help)", args[i])
		default:
			return fmt.Errorf("unexpected argument %q", args[i])
		}
	}
	if workdir == "" {
		wd, err := os.Getwd()
		if err != nil {
			return err
		}
		workdir = wd
	}
	return RunPackageTestFromFlags(workdir, only)
}

func RunPackageTestFromFlags(workdir, only string) error {
	root, err := filepath.Abs(filepath.Clean(workdir))
	if err != nil {
		return err
	}
	targets, err := discoverPackageTestTargets(root, only)
	if err != nil {
		return err
	}
	ids := mergeOperationResultIDs(buildOperationIDs(root, ""), map[string]string{
		"count": strconv.Itoa(len(targets)),
	})
	if strings.TrimSpace(only) != "" {
		ids["package"] = strings.TrimSpace(only)
	}
	if len(targets) == 0 {
		ids["result"] = "noop"
		infrastructure.LogOperationResult(os.Stderr, infrastructure.OperationResult{
			Unit:       "package.test.packages",
			Status:     infrastructure.OperationStatusDone,
			DurationMs: 0,
			IDs:        ids,
		})
		return nil
	}
	if err := infrastructure.RunOperationWithOptions(os.Stderr, "package.test.packages", "Running package tests…", ids, infrastructure.OperationOptions{Spinner: false, ProgressEvery: 5 * time.Second}, func() error {
		for _, target := range targets {
			targetIDs := mergeOperationResultIDs(buildOperationIDs(root, ""), map[string]string{
				"package": target.Name,
				"script":  filepath.ToSlash(target.ScriptRel),
			})
			if err := infrastructure.RunOperationWithOptions(os.Stderr, "package.test.package", "Running package test…", targetIDs, infrastructure.OperationOptions{Spinner: false, ProgressEvery: 5 * time.Second}, func() error {
				if err := runPackageScriptTarget(root, target, packageTestEnv(root, target), "test.script"); err != nil {
					return fmt.Errorf("package %q test: %w", target.Name, err)
				}
				return nil
			}); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return err
	}
	return nil
}

func discoverPackageTestTargets(workdir, only string) ([]packageScriptTarget, error) {
	return discoverPackageScriptTargets(workdir, only, func(manifest *domain.PackageManifest) string {
		return manifest.Test.Script
	})
}

func packageTestEnv(workdir string, target packageScriptTarget) []string {
	dockpipeBin, _ := resolveDockpipeBinForChildProcess(workdir)
	return []string{
		"DOCKPIPE_PACKAGE_TEST=1",
		"DOCKPIPE_PACKAGE_TEST_SCRIPT=" + target.ScriptRel,
		"DOCKPIPE_BIN=" + dockpipeBin,
	}
}

const packageTestUsageText = `dockpipe package test

Run package-owned tests for packages in the current source checkout that declare
test.script in package.yml.

Usage:
  dockpipe package test [--workdir <path>] [--only <package>]

Options:
  --workdir <path>  Project/worktree root (default: current directory)
  --only <package>  Run one package by package.yml name (or directory name)
`
