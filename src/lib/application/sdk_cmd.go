package application

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"dockpipe/src/lib/infrastructure"
)

const sdkUsageText = `dockpipe sdk — bootstrap SDK helpers for supported languages

Usage:
  dockpipe sdk [--workdir <path>]
  dockpipe sdk shell-env [--workdir <path>]

By default, sdk emits shell code suitable for:

  eval "$(dockpipe sdk)"

When --workdir is omitted, sdk prefers DOCKPIPE_WORKDIR from the
environment and otherwise falls back to the current working directory.

This bootstraps the shell SDK object from the canonical core helper.
`

const getUsageText = `dockpipe get — print generic DockPipe context fields

Usage:
  dockpipe get <field> [--workdir <path>]

Fields:
  workdir
  dockpipe_bin
  workflow_name
  script_dir
  package_root
  assets_dir

When --workdir is omitted, get prefers DOCKPIPE_WORKDIR from the
environment and otherwise falls back to the current working directory.

Fields such as script_dir, package_root, and assets_dir are env-backed and
print the current injected values when available.
`

func cmdSDK(args []string) error {
	if len(args) == 0 {
		return cmdSDKShellEnv(nil)
	}
	if args[0] == "-h" || args[0] == "--help" {
		fmt.Print(sdkUsageText)
		return nil
	}
	if strings.HasPrefix(args[0], "-") {
		return cmdSDKShellEnv(args)
	}
	switch args[0] {
	case "shell-env":
		return cmdSDKShellEnv(args[1:])
	default:
		return fmt.Errorf("unknown sdk subcommand %q (try: dockpipe sdk --help)", args[0])
	}
}

func cmdGet(args []string) error {
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" {
		fmt.Print(getUsageText)
		return nil
	}
	field := normalizeGetField(args[0])
	workdir, err := parseSDKWorkdirFlag(args[1:])
	if err != nil {
		return err
	}
	switch field {
	case "workdir":
		fmt.Println(workdir)
		return nil
	case "dockpipe_bin":
		bin, err := resolveDockpipeBinForSDK(workdir)
		if err != nil {
			return err
		}
		fmt.Println(bin)
		return nil
	case "workflow_name":
		v := strings.TrimSpace(os.Getenv("DOCKPIPE_WORKFLOW_NAME"))
		if v == "" {
			return fmt.Errorf("DOCKPIPE_WORKFLOW_NAME is not set")
		}
		fmt.Println(v)
		return nil
	case "script_dir":
		v := strings.TrimSpace(os.Getenv("DOCKPIPE_SCRIPT_DIR"))
		if v == "" {
			return fmt.Errorf("DOCKPIPE_SCRIPT_DIR is not set")
		}
		fmt.Println(v)
		return nil
	case "package_root":
		v := strings.TrimSpace(os.Getenv("DOCKPIPE_PACKAGE_ROOT"))
		if v == "" {
			return fmt.Errorf("DOCKPIPE_PACKAGE_ROOT is not set")
		}
		fmt.Println(v)
		return nil
	case "assets_dir":
		v := strings.TrimSpace(os.Getenv("DOCKPIPE_ASSETS_DIR"))
		if v == "" {
			return fmt.Errorf("DOCKPIPE_ASSETS_DIR is not set")
		}
		fmt.Println(v)
		return nil
	default:
		return fmt.Errorf("unknown get field %q (try: dockpipe get --help)", args[0])
	}
}

func cmdSDKShellEnv(args []string) error {
	workdir, err := parseSDKWorkdirFlag(args)
	if err != nil {
		return err
	}
	sdkPath, err := resolveShellSDKPath(workdir)
	if err != nil {
		return err
	}

	fmt.Printf("export DOCKPIPE_WORKDIR=%s\n", shellQuote(workdir))
	fmt.Printf("source %s\n", shellQuote(sdkPath))
	return nil
}

func parseSDKWorkdirFlag(args []string) (string, error) {
	workdir := ""
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--workdir":
			if i+1 >= len(args) {
				return "", fmt.Errorf("--workdir requires a path")
			}
			workdir = strings.TrimSpace(args[i+1])
			i++
		case "-h", "--help":
			return "", fmt.Errorf("usage: dockpipe sdk shell-env [--workdir <path>]")
		default:
			return "", fmt.Errorf("unknown option %q", args[i])
		}
	}

	if strings.TrimSpace(workdir) == "" {
		workdir = strings.TrimSpace(os.Getenv("DOCKPIPE_WORKDIR"))
		if workdir == "" {
			wd, err := os.Getwd()
			if err != nil {
				return "", err
			}
			workdir = wd
		}
	}
	if abs, err := filepath.Abs(workdir); err != nil {
		return "", err
	} else {
		workdir = abs
	}
	return workdir, nil
}

func resolveDockpipeBinForSDK(workdir string) (string, error) {
	if configured := strings.TrimSpace(os.Getenv("DOCKPIPE_BIN")); configured != "" {
		return configured, nil
	}
	candidate := filepath.Join(workdir, "src", "bin", "dockpipe")
	if st, err := os.Stat(candidate); err == nil && !st.IsDir() {
		return candidate, nil
	}
	path, err := exec.LookPath("dockpipe")
	if err != nil {
		return "", fmt.Errorf("dockpipe binary not found; set DOCKPIPE_BIN or add dockpipe to PATH")
	}
	return path, nil
}

func normalizeGetField(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	s = strings.ReplaceAll(s, "-", "_")
	return s
}

func resolveShellSDKPath(workdir string) (string, error) {
	for cur := workdir; ; cur = filepath.Dir(cur) {
		if infrastructure.DockpipeAuthoringSourceTree(cur) {
			return filepath.Join(infrastructure.CoreDir(cur), "assets", "scripts", "lib", "dockpipe-sdk.sh"), nil
		}
		next := filepath.Dir(cur)
		if next == cur {
			break
		}
	}
	rr, err := infrastructure.RepoRoot()
	if err != nil {
		return "", err
	}
	return filepath.Join(infrastructure.CoreDir(rr), "assets", "scripts", "lib", "dockpipe-sdk.sh"), nil
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'"'"'`) + "'"
}
