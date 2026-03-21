package application

import (
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"dockpipe/lib/dockpipe/infrastructure"
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

const dockpipeYmlBoilerplate = `# Dockpipe workflow at repository root (same shape as templates/<name>/config.yml).
# Run: dockpipe --workflow-file dockpipe.yml [options] -- <command>
name: my-project
description: Local workflow
imports: []
steps: []
`

func cmdInit(args []string) error {
	repoRoot, err := infrastructure.RepoRoot()
	if err != nil {
		return err
	}
	var fromURL, templateName, dest string
	for i := 0; i < len(args); i++ {
		if args[i] == "--from" {
			if i+1 >= len(args) {
				return fmt.Errorf("--from requires URL")
			}
			fromURL = args[i+1]
			i++
			continue
		}
		if strings.HasPrefix(args[i], "-") {
			return fmt.Errorf("unknown option %s", args[i])
		}
		if templateName == "" && dest == "" {
			if strings.Contains(args[i], "/") || args[i] == "." {
				dest = args[i]
			} else {
				templateName = args[i]
			}
		} else if dest == "" {
			dest = args[i]
		}
	}
	if dest == "" {
		dest = "."
	}
	dest, err = filepath.Abs(dest)
	if err != nil {
		return err
	}

	if fromURL != "" {
		if _, err := os.Stat(dest); err == nil {
			entries, _ := os.ReadDir(dest)
			if len(entries) > 0 {
				return fmt.Errorf("destination exists and is not empty: %s", dest)
			}
		}
		fmt.Fprintf(os.Stderr, "[dockpipe] Cloning %s into %s ...\n", fromURL, dest)
		c := exec.Command("git", "clone", fromURL, dest)
		c.Stdout = os.Stdout
		c.Stderr = os.Stderr
		return c.Run()
	}

	if fi, err := os.Stat(dest); err == nil {
		if !fi.IsDir() {
			return fmt.Errorf("destination is not a directory: %s", dest)
		}
		entries, _ := os.ReadDir(dest)
		if len(entries) > 0 {
			return fmt.Errorf("destination exists and is not empty: %s", dest)
		}
	} else {
		if err := os.MkdirAll(dest, 0o755); err != nil {
			return err
		}
	}

	initTpl := filepath.Join(repoRoot, "templates/init")
	if _, err := os.Stat(filepath.Join(initTpl, "config.yml")); err != nil {
		return fmt.Errorf("init template not found: %s", initTpl)
	}
	fmt.Fprintf(os.Stderr, "[dockpipe] Creating workspace at %s ...\n", dest)
	_ = os.MkdirAll(filepath.Join(dest, "scripts"), 0o755)
	_ = os.MkdirAll(filepath.Join(dest, "images"), 0o755)
	_ = os.MkdirAll(filepath.Join(dest, "templates"), 0o755)
	_ = os.MkdirAll(filepath.Join(dest, "templates", "core"), 0o755)
	_ = copyDirMaybe(filepath.Join(repoRoot, "templates/core"), filepath.Join(dest, "templates/core"))
	readme := `# Dockpipe workspace

- **scripts/** — Run and act scripts.
- **images/** — Dockerfiles.
- **templates/** — Your workflows (**config.yml**). Use **dockpipe --workflow &lt;name&gt;** (each folder is one workflow).
- **templates/core/** — Shared **resolvers/**, **strategies/**, optional **scripts/** and **images/** (copied from the bundled tree so workflows can reference them).
- **dockpipe.yml** (optional) — Repo-root workflow; use **dockpipe --workflow-file dockpipe.yml**.
`
	_ = os.WriteFile(filepath.Join(dest, "README.md"), []byte(readme), 0o644)
	_ = os.WriteFile(filepath.Join(dest, "dockpipe.yml"), []byte(dockpipeYmlBoilerplate), 0o644)

	if templateName != "" {
		td := filepath.Join(dest, "templates", templateName)
		if err := os.MkdirAll(td, 0o755); err != nil {
			return err
		}
		if err := copyFile(filepath.Join(initTpl, "config.yml"), filepath.Join(td, "config.yml")); err != nil {
			return err
		}
		_ = copyFileMaybe(filepath.Join(repoRoot, "scripts/example-run.sh"), filepath.Join(dest, "scripts/example-run.sh"))
		_ = copyFileMaybe(filepath.Join(repoRoot, "scripts/example-act.sh"), filepath.Join(dest, "scripts/example-act.sh"))
		_ = copyDirMaybe(filepath.Join(repoRoot, "images/example"), filepath.Join(dest, "images/example"))
		fmt.Printf("Created: %s with templates/%s/ (shared resolvers/strategies under templates/core/)\n", dest, templateName)
	} else {
		fmt.Printf("Created: %s (scripts/, images/, templates/)\n", dest)
	}
	return nil
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
		from = "run-worktree"
	}
	src := filepath.Join(repoRoot, "templates", from)
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
	coreSrc := filepath.Join(repoRoot, "templates", "core")
	if _, err := os.Stat(coreDest); err != nil && os.IsNotExist(err) {
		if err := os.MkdirAll(filepath.Join(wdParent, "templates"), 0o755); err != nil {
			return err
		}
		if err := copyDirMaybe(coreSrc, coreDest); err != nil {
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
		src := filepath.Join(repoRoot, "scripts", base+".sh")
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
