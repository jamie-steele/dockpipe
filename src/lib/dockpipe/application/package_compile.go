package application

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"dockpipe/src/lib/dockpipe/domain"
	"dockpipe/src/lib/dockpipe/infrastructure"

	"gopkg.in/yaml.v3"
)

func cmdPackageCompile(args []string) error {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" {
		fmt.Print(packageCompileUsageText)
		return nil
	}
	switch args[0] {
	case "workflow":
		return cmdPackageCompileWorkflow(args[1:])
	default:
		return fmt.Errorf("unknown package compile target %q (try: dockpipe package compile --help)", args[0])
	}
}

func cmdPackageCompileWorkflow(args []string) error {
	if len(args) > 0 && (args[0] == "--help" || args[0] == "-h") {
		fmt.Print(packageCompileWorkflowUsageText)
		return nil
	}
	var (
		workdir string
		src     string
		name    string
		force   bool
	)
	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "--workdir" && i+1 < len(args):
			workdir = args[i+1]
			i++
		case (args[i] == "--from" || args[i] == "--source") && i+1 < len(args):
			src = args[i+1]
			i++
		case args[i] == "--name" && i+1 < len(args):
			name = args[i+1]
			i++
		case args[i] == "--force":
			force = true
		case strings.HasPrefix(args[i], "-"):
			return fmt.Errorf("unknown option %s (try: dockpipe package compile workflow --help)", args[i])
		default:
			if src == "" {
				src = args[i]
				continue
			}
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
	if strings.TrimSpace(src) == "" {
		return fmt.Errorf("missing source directory (use --from <path> or a positional path)")
	}
	srcAbs, err := filepath.Abs(filepath.Clean(src))
	if err != nil {
		return err
	}
	cfgPath := filepath.Join(srcAbs, "config.yml")
	if _, err := os.Stat(cfgPath); err != nil {
		return fmt.Errorf("workflow source must contain config.yml: %w", err)
	}
	if err := infrastructure.ValidateWorkflowYAML(cfgPath); err != nil {
		return fmt.Errorf("validate workflow: %w", err)
	}
	b, err := os.ReadFile(cfgPath)
	if err != nil {
		return err
	}
	wf, err := domain.ParseWorkflowYAML(b)
	if err != nil {
		return fmt.Errorf("parse workflow: %w", err)
	}
	pkgName := strings.TrimSpace(name)
	if pkgName == "" {
		pkgName = strings.TrimSpace(wf.Name)
	}
	if pkgName == "" {
		pkgName = filepath.Base(srcAbs)
	}
	destRoot, err := infrastructure.PackagesWorkflowsDir(workdir)
	if err != nil {
		return err
	}
	destRoot = filepath.Join(destRoot, pkgName)
	if _, err := os.Stat(destRoot); err == nil {
		if !force {
			return fmt.Errorf("destination exists: %s (use --force to replace)", destRoot)
		}
		if err := os.RemoveAll(destRoot); err != nil {
			return fmt.Errorf("remove existing: %w", err)
		}
	}
	if err := copyDir(srcAbs, destRoot); err != nil {
		return fmt.Errorf("copy workflow: %w", err)
	}
	manifestPath := filepath.Join(destRoot, infrastructure.PackageManifestFilename)
	if _, err := os.Stat(manifestPath); os.IsNotExist(err) {
		pm := map[string]any{
			"schema":       1,
			"name":         pkgName,
			"version":      "0.1.0",
			"title":        pkgName,
			"description":  "Compiled from " + srcAbs,
			"kind":         "workflow",
			"allow_clone":  true,
			"distribution": "source",
		}
		out, err := yaml.Marshal(pm)
		if err != nil {
			return err
		}
		if err := os.WriteFile(manifestPath, out, 0o644); err != nil {
			return err
		}
	}
	fmt.Fprintf(os.Stderr, "[dockpipe] compiled workflow package → %s\n", destRoot)
	return nil
}

const packageCompileUsageText = `dockpipe package compile

Validate and materialize content into .dockpipe/internal/packages/ (see docs/package-model.md).

Usage:
  dockpipe package compile workflow [options] [--from] <source-dir>

`

const packageCompileWorkflowUsageText = `dockpipe package compile workflow <source-dir>

Runs workflow YAML validation (same rules as dockpipe workflow validate), then copies the
directory into <workdir>/.dockpipe/internal/packages/workflows/<name>/.

Options:
  --workdir <path>   Project directory (default: current directory)
  --from <path>      Source workflow directory (same as positional <source-dir>)
  --name <n>         Package folder name (default: workflow name from config.yml, else basename of source)
  --force            Replace existing package directory

`
