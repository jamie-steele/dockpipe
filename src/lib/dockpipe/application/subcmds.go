package application

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"dockpipe/src/lib/dockpipe/infrastructure"
)

const actionBoilerplate = `#!/usr/bin/env bash
# dockpipe action — runs inside the container after your command.
set -euo pipefail
cd "${DOCKPIPE_CONTAINER_WORKDIR:-/work}"
if [[ "${DOCKPIPE_EXIT_CODE:-1}" -eq 0 ]]; then
  echo "Command succeeded, acting on results..."
else
  echo "Command failed with code ${DOCKPIPE_EXIT_CODE}" >&2
fi
exit "${DOCKPIPE_EXIT_CODE:-1}"
`

const preBoilerplate = `#!/usr/bin/env bash
# dockpipe pre-script — runs on the host before the container.
set -euo pipefail
`

func cmdInit(args []string) error {
	for _, a := range args {
		if a == "--help" || a == "-h" {
			printInitUsage()
			return nil
		}
	}

	repoRoot, err := infrastructure.RepoRoot()
	if err != nil {
		return err
	}
	projectDir, err := os.Getwd()
	if err != nil {
		return err
	}
	projectDir, err = filepath.Abs(projectDir)
	if err != nil {
		return err
	}

	var name, from string
	var resolver, runtime, strategy string
	var gitignore bool
	var workflowsDir string
	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "--gitignore":
			gitignore = true
		case args[i] == "--workflows-dir" && i+1 < len(args):
			workflowsDir = args[i+1]
			i++
		case args[i] == "--from" && i+1 < len(args):
			from = args[i+1]
			i++
		case args[i] == "--resolver" && i+1 < len(args):
			resolver = args[i+1]
			i++
		case args[i] == "--runtime" && i+1 < len(args):
			runtime = args[i+1]
			i++
		case args[i] == "--strategy" && i+1 < len(args):
			strategy = args[i+1]
			i++
		case strings.HasPrefix(args[i], "-"):
			return fmt.Errorf("unknown option %s", args[i])
		default:
			if name != "" {
				return fmt.Errorf("unexpected argument %q", args[i])
			}
			name = args[i]
		}
	}

	if (resolver != "" || runtime != "" || strategy != "") && name == "" {
		return fmt.Errorf("--resolver, --runtime, and --strategy require a workflow name: dockpipe init <name> ...")
	}
	if strings.TrimSpace(from) != "" && name == "" {
		return fmt.Errorf("--from requires a workflow name: dockpipe init <name> --from <source>")
	}
	if strings.TrimSpace(workflowsDir) != "" && name == "" {
		return fmt.Errorf("--workflows-dir requires a workflow name: dockpipe init <name> --workflows-dir <path>")
	}
	if workflowsDir != "" {
		infrastructure.SetWorkflowsDirForProcess(workflowsDir)
		defer infrastructure.SetWorkflowsDirForProcess("")
	}

	if err := ensureProjectScaffold(repoRoot, projectDir); err != nil {
		return err
	}
	if gitignore {
		if err := appendDockpipeGitignore(projectDir); err != nil {
			return err
		}
	}
	if name == "" {
		fmt.Fprintf(os.Stderr, "[dockpipe] Initialized Dockpipe in %s\n", projectDir)
		return nil
	}

	fromSource := strings.TrimSpace(from)
	if fromSource == "" {
		fromSource = "blank"
	}
	return createNamedWorkflow(repoRoot, projectDir, name, fromSource, resolver, runtime, strategy)
}

func cmdAction(args []string) error {
	return cmdInitLikeScript(args, "my-action.sh", []string{"commit-worktree", "export-patch", "print-summary"}, actionBoilerplate)
}

func cmdPre(args []string) error {
	return cmdInitLikeScript(args, "my-pre.sh", []string{"clone-worktree"}, preBoilerplate)
}

func cmdTemplate(args []string) error {
	repoRoot, err := infrastructure.RepoRoot()
	if err != nil {
		return err
	}
	if len(args) == 0 || (args[0] != "init" && args[0] != "create") {
		return fmt.Errorf("usage: dockpipe template init [--from <bundled>] <dirname>")
	}
	args = args[1:]
	var name, from string
	for i := 0; i < len(args); i++ {
		if args[i] == "--from" {
			if i+1 >= len(args) {
				return fmt.Errorf("--from requires argument")
			}
			from = args[i+1]
			i++
			continue
		}
		if name == "" {
			name = args[i]
		}
	}
	if name == "" {
		name = "my-workflow"
	}
	if from == "" {
		from = "init"
	}
	src := bundledWorkflowSourceDir(repoRoot, from)
	if _, err := os.Stat(src); err != nil {
		return fmt.Errorf("unknown bundled template %q", from)
	}
	dest := name
	if !filepath.IsAbs(dest) {
		wd, _ := os.Getwd()
		dest = filepath.Join(wd, dest)
	}
	if _, err := os.Stat(dest); err == nil {
		return fmt.Errorf("%s already exists", dest)
	}
	if err := copyDir(src, dest); err != nil {
		return err
	}
	// Pull in shared templates/core next to the new workflow if not already present (resolvers, strategies).
	wdParent := filepath.Dir(dest)
	coreDest := filepath.Join(wdParent, "templates", "core")
	if _, err := os.Stat(coreDest); err != nil && os.IsNotExist(err) {
		if err := os.MkdirAll(filepath.Join(wdParent, "templates"), 0o755); err != nil {
			return err
		}
		if err := copyBundledCoreInto(coreDest, repoRoot); err != nil {
			return fmt.Errorf("copy shared templates/core: %w", err)
		}
	}
	_ = filepath.WalkDir(dest, func(p string, d fs.DirEntry, err error) error {
		if err == nil && strings.HasSuffix(p, ".sh") {
			_ = os.Chmod(p, 0o755)
		}
		return nil
	})
	fmt.Printf("Created: %s (from template %s)\n", dest, from)
	return nil
}

func cmdInitLikeScript(args []string, defaultName string, bundled []string, boiler string) error {
	repoRoot, err := infrastructure.RepoRoot()
	if err != nil {
		return err
	}
	if len(args) == 0 || (args[0] != "init" && args[0] != "create") {
		return fmt.Errorf("usage: dockpipe <cmd> init [--from <bundled>] <filename>")
	}
	args = args[1:]
	var name, from string
	for i := 0; i < len(args); i++ {
		if args[i] == "--from" {
			if i+1 >= len(args) {
				return fmt.Errorf("--from requires argument")
			}
			from = args[i+1]
			i++
			continue
		}
		if name == "" {
			name = args[i]
		}
	}
	if name == "" {
		name = defaultName
	}
	if !strings.HasSuffix(name, ".sh") {
		name += ".sh"
	}
	wd, _ := os.Getwd()
	dest := name
	if !filepath.IsAbs(dest) {
		dest = filepath.Join(wd, dest)
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	if _, err := os.Stat(dest); err == nil {
		return fmt.Errorf("%s already exists", dest)
	}
	if from != "" {
		base := strings.TrimSuffix(from, ".sh")
		src := filepath.Join(infrastructure.CoreDir(repoRoot), "assets", "scripts", base+".sh")
		if _, err := os.Stat(src); err != nil {
			return fmt.Errorf("unknown bundled script %q (try: %v)", from, bundled)
		}
		if err := copyFile(src, dest); err != nil {
			return err
		}
		return os.Chmod(dest, 0o755)
	}
	return os.WriteFile(dest, []byte(boiler), 0o755)
}

// copyBundledCoreInto merges category dirs (assets, resolvers, runtimes, strategies) into coreDest.
// When the install tree is dockpipe-style src/core (has workflows/), bundled example workflows are not copied.
func copyBundledCoreInto(coreDest, repoRoot string) error {
	coreSrc := infrastructure.CoreDir(repoRoot)
	wfDir := filepath.Join(coreSrc, "workflows")
	if st, err := os.Stat(wfDir); err == nil && st.IsDir() {
		for _, name := range []string{"assets", "resolvers", "runtimes", "strategies"} {
			if err := copyDirMaybe(filepath.Join(coreSrc, name), filepath.Join(coreDest, name)); err != nil {
				return err
			}
		}
		return nil
	}
	return copyDirMaybe(coreSrc, coreDest)
}

// mergeBundledTemplatesCore copies templates/core from the dockpipe install into dest,
// matching dockpipe init without --from. Tool resolvers and bundles ship in the binary via embed (.staging/*), not via templates/vendor.
func mergeBundledTemplatesCore(repoRoot, dest string) error {
	_ = os.MkdirAll(filepath.Join(dest, "templates"), 0o755)
	return copyBundledCoreInto(filepath.Join(dest, "templates", "core"), repoRoot)
}

func copyFile(src, dst string) error {
	b, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	_ = os.MkdirAll(filepath.Dir(dst), 0o755)
	return os.WriteFile(dst, b, 0o644)
}

func copyFileMaybe(src, dst string) error {
	if _, err := os.Stat(src); err != nil {
		return nil
	}
	return copyFile(src, dst)
}

func copyDirMaybe(src, dst string) error {
	if _, err := os.Stat(src); err != nil {
		return nil
	}
	return copyDir(src, dst)
}

func copyDir(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return os.MkdirAll(dst, 0o755)
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
}
