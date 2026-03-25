package application

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"dockpipe/src/lib/dockpipe/domain"
	"dockpipe/src/lib/dockpipe/infrastructure"
)

func cmdPackage(args []string) error {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" {
		printPackageUsage()
		return nil
	}
	switch args[0] {
	case "list":
		return cmdPackageList(args[1:])
	case "manifest":
		return cmdPackageManifest(args[1:])
	case "build":
		return cmdPackageBuild(args[1:])
	case "compile":
		return cmdPackageCompile(args[1:])
	default:
		return fmt.Errorf("unknown package subcommand %q (try: dockpipe package --help)", args[0])
	}
}

func cmdPackageList(args []string) error {
	if len(args) > 0 && (args[0] == "--help" || args[0] == "-h") {
		fmt.Print(packageListUsageText)
		return nil
	}
	var workdir string
	for i := 0; i < len(args); i++ {
		if args[i] == "--workdir" && i+1 < len(args) {
			workdir = args[i+1]
			i++
			continue
		}
		if strings.HasPrefix(args[i], "-") {
			return fmt.Errorf("unknown option %s", args[i])
		}
		return fmt.Errorf("unexpected argument %q", args[i])
	}
	if workdir == "" {
		wd, err := os.Getwd()
		if err != nil {
			return err
		}
		workdir = wd
	}
	root, err := infrastructure.PackagesRoot(workdir)
	if err != nil {
		return err
	}
	if _, err := os.Stat(root); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "[dockpipe] no packages directory yet (%s)\n", root)
		return nil
	}
	return walkPackageDirs(root)
}

func walkPackageDirs(root string) error {
	return filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if filepath.Base(path) != infrastructure.PackageManifestFilename {
			return nil
		}
		m, err := domain.ParsePackageManifest(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "[dockpipe] %s: %v\n", path, err)
			return nil
		}
		dir := filepath.Dir(path)
		rel, _ := filepath.Rel(root, dir)
		fmt.Fprintf(os.Stderr, "%s\t%s\t%s\t%s\n", rel, m.Name, m.Version, trimDesc(m.Description))
		return nil
	})
}

func trimDesc(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) > 60 {
		return s[:57] + "..."
	}
	return s
}

func cmdPackageManifest(args []string) error {
	if len(args) > 0 && (args[0] == "--help" || args[0] == "-h") {
		fmt.Print(packageManifestUsageText)
		return nil
	}
	if len(args) > 0 {
		return fmt.Errorf("unexpected arguments (try: dockpipe package manifest)")
	}
	fmt.Print(examplePackageManifestYAML)
	return nil
}

const examplePackageManifestYAML = `# package.yml — metadata for a DockPipe package (workflow, core slice, or assets).
# Place next to the package tree under .dockpipe/internal/packages/ (see docs/package-model.md).
schema: 1
name: my-package
version: 0.1.0
title: Short human title
description: |
  Longer description for humans and search.
author: Your name or org
website: https://example.com
license: MIT
# Optional: workflow | core | assets | bundle
kind: workflow
`

func printPackageUsage() {
	fmt.Print(packageUsageText)
}

const packageUsageText = `dockpipe package

Inspect installed packages and package metadata. Installed store content lives under
.dockpipe/internal/packages/ by default (see docs/package-model.md).

Usage:
  dockpipe package list [--workdir <path>]
  dockpipe package manifest
  dockpipe package build core [options]
  dockpipe package compile workflow [options] <source-dir>

  list      Find package.yml under .dockpipe/internal/packages and print rel path, name, version, description.
  manifest  Print an example package.yml schema to stdout.
  build     Author templates-core tarball + checksum + install-manifest (self-hosted / dogfood).
  compile   Validate workflow YAML and copy into .dockpipe/internal/packages/workflows/.

Environment:
  DOCKPIPE_PACKAGES_ROOT   Override packages root (default: <workdir>/.dockpipe/internal/packages).

`

const packageListUsageText = `dockpipe package list [--workdir <path>]

Scans .dockpipe/internal/packages (recursive) for package.yml files.

`

const packageManifestUsageText = `dockpipe package manifest

Prints an example package.yml to stdout (copy into your package tree).

`
