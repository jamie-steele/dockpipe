package application

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"dockpipe/src/lib/dockpipe/domain"
	"dockpipe/src/lib/dockpipe/infrastructure"
)

func cmdClone(args []string) error {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" {
		fmt.Print(cloneUsageText)
		return nil
	}
	var (
		workdir string
		to      string
		force   bool
		name    string
	)
	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "--workdir" && i+1 < len(args):
			workdir = args[i+1]
			i++
		case args[i] == "--to" && i+1 < len(args):
			to = args[i+1]
			i++
		case args[i] == "--force":
			force = true
		case strings.HasPrefix(args[i], "-"):
			return fmt.Errorf("unknown option %s (try: dockpipe clone --help)", args[i])
		default:
			if name == "" {
				name = args[i]
				continue
			}
			return fmt.Errorf("unexpected argument %q", args[i])
		}
	}
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("usage: dockpipe clone <package-name> [--workdir <path>] [--to <dest-dir>] [--force]")
	}
	if workdir == "" {
		wd, err := os.Getwd()
		if err != nil {
			return err
		}
		workdir = wd
	}
	wfRoot, err := infrastructure.PackagesWorkflowsDir(workdir)
	if err != nil {
		return err
	}
	srcRoot := filepath.Join(wfRoot, name)
	if st, err := os.Stat(srcRoot); err != nil || !st.IsDir() {
		return fmt.Errorf("compiled workflow package not found: %s (expected under %s)", name, wfRoot)
	}
	manifestPath := filepath.Join(srcRoot, infrastructure.PackageManifestFilename)
	m, err := domain.ParsePackageManifest(manifestPath)
	if err != nil {
		return fmt.Errorf("read package manifest: %w", err)
	}
	if !m.AllowClone {
		return fmt.Errorf(
			`package %q does not allow cloning (allow_clone is false or omitted in %s). `+
				`Authors who publish commercial or binary-only artifacts should keep allow_clone false; `+
				`set allow_clone: true in package.yml to permit dockpipe clone for education or source recovery`,
			name, manifestPath)
	}
	if strings.TrimSpace(to) == "" {
		to = filepath.Join(workdir, infrastructure.DefaultUserWorkflowsDirRel, name)
	}
	toAbs, err := filepath.Abs(filepath.Clean(to))
	if err != nil {
		return err
	}
	if _, err := os.Stat(toAbs); err == nil {
		if !force {
			return fmt.Errorf("destination exists: %s (use --force to replace)", toAbs)
		}
		if err := os.RemoveAll(toAbs); err != nil {
			return fmt.Errorf("remove existing: %w", err)
		}
	}
	if err := copyDir(srcRoot, toAbs); err != nil {
		return fmt.Errorf("clone: %w", err)
	}
	fmt.Fprintf(os.Stderr, "[dockpipe] cloned %s → %s\n", srcRoot, toAbs)
	return nil
}

const cloneUsageText = `dockpipe clone

Copy a compiled workflow package from .dockpipe/internal/packages/workflows/<name>/ into an authoring
tree when package.yml allows it (allow_clone: true). Use this to study, fork, or recover sources for
packages the author marked as cloneable.

Authors who sell binary-only or obfuscated drops should set allow_clone: false (default when omitted)
and distribution: binary in package.yml so consumers know clone is intentionally disabled.

Usage:
  dockpipe clone <package-name> [--workdir <path>] [--to <dest-dir>] [--force]

Arguments:
  <package-name>   Directory name under .../packages/workflows/

Options:
  --workdir <path>  Project root (default: current directory)
  --to <dest-dir>    Destination directory (default: <workdir>/workflows/<package-name>)
  --force            Replace existing destination directory

See docs/package-model.md (clone & commercial packages).

`
