package application

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"dockpipe/src/lib/domain"
	"dockpipe/src/lib/infrastructure"
	"dockpipe/src/lib/infrastructure/packagebuild"

	"gopkg.in/yaml.v3"
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
	for _, tc := range []struct{ sub, glob string }{
		{"core", "dockpipe-core-*.tar.gz"},
		{"workflows", "dockpipe-workflow-*.tar.gz"},
		{"resolvers", "dockpipe-resolver-*.tar.gz"},
	} {
		matches, err := filepath.Glob(filepath.Join(root, tc.sub, tc.glob))
		if err != nil {
			return err
		}
		for _, tgz := range matches {
			members, err := packagebuild.ListTarGzMemberPaths(tgz)
			if err != nil {
				fmt.Fprintf(os.Stderr, "[dockpipe] %s: %v\n", tgz, err)
				continue
			}
			pmPath := packageYMLPathInTarMembers(tc.sub, members)
			if pmPath == "" {
				continue
			}
			b, err := packagebuild.ReadFileFromTarGz(tgz, pmPath)
			if err != nil {
				fmt.Fprintf(os.Stderr, "[dockpipe] %s: %v\n", tgz, err)
				continue
			}
			var m domain.PackageManifest
			if err := yaml.Unmarshal(b, &m); err != nil {
				fmt.Fprintf(os.Stderr, "[dockpipe] %s: %v\n", tgz, err)
				continue
			}
			if err := domain.ValidatePackageManifest(&m); err != nil {
				fmt.Fprintf(os.Stderr, "[dockpipe] %s: %v\n", tgz, err)
				continue
			}
			rel, _ := filepath.Rel(root, tgz)
			fmt.Fprintf(os.Stderr, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n", rel, m.Name, m.Version, m.Provider, m.Capability, joinRequiresComma(m.RequiresCapabilities), trimDesc(m.Description))
		}
	}
	return filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if strings.HasSuffix(path, ".tar.gz") || strings.HasSuffix(path, ".sha256") {
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
		fmt.Fprintf(os.Stderr, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n", rel, m.Name, m.Version, m.Provider, m.Capability, joinRequiresComma(m.RequiresCapabilities), trimDesc(m.Description))
		return nil
	})
}

func packageYMLPathInTarMembers(sub string, members []string) string {
	switch sub {
	case "core":
		return "core/package.yml"
	case "workflows":
		for _, m := range members {
			if strings.HasPrefix(m, "workflows/") && strings.HasSuffix(m, "/package.yml") {
				return m
			}
		}
	case "resolvers":
		for _, m := range members {
			if strings.HasPrefix(m, "resolvers/") && strings.HasSuffix(m, "/package.yml") {
				return m
			}
		}
	}
	return ""
}

func joinRequiresComma(xs []string) string {
	if len(xs) == 0 {
		return ""
	}
	return strings.Join(xs, ",")
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

const examplePackageManifestYAML = `# package.yml — metadata for a DockPipe package (workflow, resolver, core slice, or assets).
# Place next to the package tree under bin/.dockpipe/internal/packages/ (see docs/package-model.md).
schema: 1
name: my-package
version: 0.1.0
title: Short human title
description: |
  Longer description for humans and search.
author: Your name or org
website: https://example.com
license: Apache-2.0
# Optional: workflow | resolver | core | assets | bundle | package
kind: workflow
# kind: package — umbrella at e.g. .staging/packages/agent/package.yml (metadata only; resolvers are under resolvers/):
# includes_resolvers: [codex, claude, ollama]
# kind: resolver — set capability to the dotted id this package provides (see docs/capabilities.md):
# capability: cli.codex
# kind: workflow — capability dependencies:
# requires_capabilities: [cli.codex, app.vscode]
# Optional — platform/vendor id for filtering (short label, e.g. cloudflare, aws — not a URL):
# provider: cloudflare
# Optional — authoring / store discovery (see docs/package-model.md):
# tags: [ci, security]
# keywords: [dockpipe, workflow]
# min_dockpipe_version: "0.9.0"
# repository: https://github.com/org/repo
# provides: [codex]           # kind: resolver — capability ids
# requires_resolvers: [claude] # kind: workflow — hints
# depends: [other-package]
# allow_clone: true       # allow dockpipe clone to export to workflows/ (omit or false for commercial/binary-only)
# distribution: source    # optional: source | binary — policy hint for store pages
`

func printPackageUsage() {
	fmt.Print(packageUsageText)
}

const packageUsageText = `dockpipe package

Inspect installed packages and package metadata. Installed store content lives under
bin/.dockpipe/internal/packages/ by default (see docs/package-model.md).

Usage:
  dockpipe package list [--workdir <path>]
  dockpipe package manifest
  dockpipe package build core|store [options]
  dockpipe package compile core|resolvers|bundles|workflows|all|for-workflow|workflow [options]  (bundles = alias for workflows)

  list      Find package.yml under bin/.dockpipe/internal/packages and print rel path, name, version, provider, capability, requires_capabilities (comma-separated), description.
  manifest  Print an example package.yml schema to stdout.
  build     core: templates-core tarball + install-manifest; store: gzip tar per compiled package + packages-store-manifest.json.
  compile   Materialize core / resolvers / workflows into bin/.dockpipe/internal/packages/ (see compile --help).

Optional repo-root dockpipe.config.json lists compile.workflows roots (resolver discovery uses the same list plus src/core/resolvers); compile.bundles merged into workflows when set; see docs/package-model.md.

Environment:
  DOCKPIPE_PACKAGES_ROOT   Override packages root (default: <workdir>/bin/.dockpipe/internal/packages).

`

const packageListUsageText = `dockpipe package list [--workdir <path>]

Scans bin/.dockpipe/internal/packages (recursive) for package.yml files.
Output columns (tab-separated): path, name, version, provider, capability, requires_capabilities, description.

`

const packageManifestUsageText = `dockpipe package manifest

Prints an example package.yml to stdout (copy into your package tree).

`
