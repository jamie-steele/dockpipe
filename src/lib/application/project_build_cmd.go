package application

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"dockpipe/src/lib/domain"
	"dockpipe/src/lib/infrastructure"
)

// cmdBuild runs package compile: full `compile all` by default, or `compile for-workflow` when --for-workflow is set.
func cmdBuild(args []string) error {
	if len(args) > 0 && (args[0] == "--help" || args[0] == "-h") {
		fmt.Print(buildUsageText)
		return nil
	}
	var (
		wfName      string
		buildImages = true
	)
	forward := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--for-workflow":
			if i+1 >= len(args) {
				return fmt.Errorf("--for-workflow requires a workflow name")
			}
			wfName = args[i+1]
			i++
			continue
		case "--images":
			buildImages = true
			continue
		case "--no-images":
			buildImages = false
			continue
		}
		forward = append(forward, args[i])
	}
	workdir, err := parseBuildWorkdir(forward)
	if err != nil {
		return err
	}
	if wfName != "" {
		if err := cmdPackage(append([]string{"compile", "for-workflow", wfName, "--force"}, forward...)); err != nil {
			return err
		}
	} else if err := cmdPackage(append([]string{"compile", "all", "--force"}, forward...)); err != nil {
		return err
	}
	if !buildImages {
		return nil
	}
	n, err := prebuildCompiledImageArtifacts(workdir)
	if err != nil {
		return err
	}
	if n > 0 {
		fmt.Fprintf(os.Stderr, "[dockpipe] image: materialized %d image artifact(s)\n", n)
	}
	return nil
}

func parseBuildWorkdir(args []string) (string, error) {
	var workdir string
	for i := 0; i < len(args); i++ {
		if args[i] == "--workdir" && i+1 < len(args) {
			workdir = args[i+1]
			i++
		}
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
		return filepath.Abs(root)
	}
	return filepath.Abs(workdir)
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

const buildUsageText = `dockpipe build [--for-workflow <name>] [options]

Without --for-workflow: same as dockpipe package compile all --force (full store).
PipeLang sources are compiled during package staging and included in package tarballs.
Dockerfile-backed image artifacts are prebuilt by default after compile.

With --for-workflow <name>: same as dockpipe package compile for-workflow <name> --force
(transitive core + resolver + workflow closure only).

Options:
  --for-workflow <name>   Dependency-scoped compile instead of compile all
  --images                Prebuild Dockerfile-backed image artifacts after compile (default)
  --no-images             Only compile package/runtime/image manifests; do not run docker build
  Otherwise same as package compile all / for-workflow: --workdir
  (see: dockpipe package compile all --help)

`

const cleanUsageText = `dockpipe clean

Remove the compiled package store (default: <workdir>/bin/.dockpipe/internal/packages).
Other bin/.dockpipe/ content (runs, caches, etc.) is left in place.

When --workdir is omitted, the project directory is the folder containing
dockpipe.config.json (walking up from the current directory), or the current
directory if that file is not found.

Usage:
  dockpipe clean [--workdir <path>]

Environment:
  DOCKPIPE_PACKAGES_ROOT  If set, that directory is removed (instead of <workdir>/bin/.dockpipe/internal/packages).

`

const rebuildUsageText = `dockpipe rebuild

Runs dockpipe clean, then dockpipe build (compile all with --force, or compile for-workflow
if you pass --for-workflow). Only --workdir is forwarded to clean; all other flags apply
to the build step.

Default project directory (when --workdir omitted) is the same as compile: the directory
with dockpipe.config.json, found by walking up from the current directory.

Usage:
  dockpipe rebuild [options]

Options:
  Same as dockpipe build / package compile all (--workdir).
  build implies --force for compile outputs. See: dockpipe package compile all --help

`
