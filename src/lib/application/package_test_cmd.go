package application

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"dockpipe/src/lib/domain"
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
	if len(targets) == 0 {
		if strings.TrimSpace(only) != "" {
			fmt.Fprintf(os.Stderr, "[dockpipe] package test: no package test matched %q\n", strings.TrimSpace(only))
		}
		return nil
	}
	for _, target := range targets {
		fmt.Fprintf(os.Stderr, "[dockpipe] package test: %s (%s)\n", target.Name, target.ScriptRel)
		if err := runPackageScriptTarget(root, target, packageTestEnv(root, target), "test.script"); err != nil {
			return fmt.Errorf("package %q test: %w", target.Name, err)
		}
	}
	return nil
}

func discoverPackageTestTargets(workdir, only string) ([]packageScriptTarget, error) {
	return discoverPackageScriptTargets(workdir, only, func(manifest *domain.PackageManifest) string {
		return manifest.Test.Script
	})
}

func packageTestEnv(workdir string, target packageScriptTarget) []string {
	dockpipeBin, _ := resolveDockpipeBinForSDK(workdir)
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
