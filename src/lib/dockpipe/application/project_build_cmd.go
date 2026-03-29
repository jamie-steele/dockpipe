package application

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"dockpipe/src/lib/dockpipe/domain"
	"dockpipe/src/lib/dockpipe/infrastructure"
)

// cmdBuild runs `dockpipe package compile all` with --force so a normal build replaces
// existing compiled slices (core, workflows) without requiring an extra flag.
func cmdBuild(args []string) error {
	if len(args) > 0 && (args[0] == "--help" || args[0] == "-h") {
		fmt.Print(buildUsageText)
		return nil
	}
	return cmdPackage(append([]string{"compile", "all", "--force"}, args...))
}

func cmdClean(args []string) error {
	if len(args) > 0 && (args[0] == "--help" || args[0] == "-h") {
		fmt.Print(cleanUsageText)
		return nil
	}
	workdir, err := parseWorkdirOnly(args)
	if err != nil {
		return err
	}
	root, err := infrastructure.PackagesRoot(workdir)
	if err != nil {
		return err
	}
	if _, err := os.Stat(root); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "[dockpipe] clean: nothing to remove (%s)\n", root)
		return nil
	}
	if err := os.RemoveAll(root); err != nil {
		return fmt.Errorf("clean: %w", err)
	}
	fmt.Fprintf(os.Stderr, "[dockpipe] clean: removed %s\n", root)
	return nil
}

func parseWorkdirOnly(args []string) (string, error) {
	var workdir string
	for i := 0; i < len(args); i++ {
		if args[i] == "--workdir" && i+1 < len(args) {
			workdir = args[i+1]
			i++
			continue
		}
		if strings.HasPrefix(args[i], "-") {
			return "", fmt.Errorf("unknown option %s (try: dockpipe clean --help)", args[i])
		}
		return "", fmt.Errorf("unexpected argument %q (try: dockpipe clean --help)", args[i])
	}
	if workdir == "" {
		wd, err := os.Getwd()
		if err != nil {
			return "", err
		}
		root, err := domain.FindProjectRootWithDockpipeConfig(wd)
		if err != nil {
			return "", err
		}
		wdAbs, err := filepath.Abs(wd)
		if err != nil {
			return "", err
		}
		if root != wdAbs {
			fmt.Fprintf(os.Stderr, "[dockpipe] using project root %s (%s)\n", root, domain.DockpipeProjectConfigFileName)
		}
		return filepath.Abs(root)
	}
	return filepath.Abs(workdir)
}

// filterCleanArgs keeps only --workdir <path> for the clean step of rebuild.
func filterCleanArgs(args []string) []string {
	var out []string
	for i := 0; i < len(args); i++ {
		if args[i] == "--workdir" && i+1 < len(args) {
			out = append(out, args[i], args[i+1])
			i++
		}
	}
	return out
}

func cmdRebuild(args []string) error {
	if len(args) > 0 && (args[0] == "--help" || args[0] == "-h") {
		fmt.Print(rebuildUsageText)
		return nil
	}
	if err := cmdClean(filterCleanArgs(args)); err != nil {
		return err
	}
	return cmdBuild(args)
}

const buildUsageText = `dockpipe build

Same as dockpipe package compile all, but always replaces existing compiled packages
(--force is implied for core and workflow outputs). Use dockpipe clean first if you need
an empty store before compiling.

Options:
  Same as dockpipe package compile all: --workdir, --no-staging
  (see: dockpipe package compile all --help)

`

const cleanUsageText = `dockpipe clean

Remove the compiled package store (default: <workdir>/.dockpipe/internal/packages).
Other .dockpipe/ content (runs, caches, etc.) is left in place.

When --workdir is omitted, the project directory is the folder containing
dockpipe.config.json (walking up from the current directory), or the current
directory if that file is not found.

Usage:
  dockpipe clean [--workdir <path>]

Environment:
  DOCKPIPE_PACKAGES_ROOT  If set, that directory is removed (instead of <workdir>/.dockpipe/internal/packages).

`

const rebuildUsageText = `dockpipe rebuild

Runs dockpipe clean, then dockpipe build (package compile all with --force). Only --workdir
is forwarded to clean; all other flags apply to the build step.

Default project directory (when --workdir omitted) is the same as compile: the directory
with dockpipe.config.json, found by walking up from the current directory.

Usage:
  dockpipe rebuild [options]

Options:
  Same as dockpipe build / package compile all (--workdir, --no-staging).
  build implies --force for compile outputs. See: dockpipe package compile all --help

`
