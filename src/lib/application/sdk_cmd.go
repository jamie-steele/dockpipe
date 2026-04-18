package application

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"dockpipe/src/lib/infrastructure"
)

const sdkUsageText = `dockpipe sdk — bootstrap SDK helpers for supported languages

Usage:
  dockpipe sdk [--workdir <path>]
  dockpipe sdk shell-env [--workdir <path>]

By default, sdk emits shell code suitable for:

  eval "$("${DOCKPIPE_BIN:-dockpipe}" sdk)"

When --workdir is omitted, sdk prefers DOCKPIPE_WORKDIR from the
environment and otherwise falls back to the current working directory.

This bootstraps the shell SDK object from the canonical core helper.
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

func cmdSDKShellEnv(args []string) error {
	workdir := ""
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--workdir":
			if i+1 >= len(args) {
				return fmt.Errorf("--workdir requires a path")
			}
			workdir = strings.TrimSpace(args[i+1])
			i++
		case "-h", "--help":
			return fmt.Errorf("usage: dockpipe sdk shell-env [--workdir <path>]")
		default:
			return fmt.Errorf("unknown option %q", args[i])
		}
	}

	if strings.TrimSpace(workdir) == "" {
		workdir = strings.TrimSpace(os.Getenv("DOCKPIPE_WORKDIR"))
		if workdir == "" {
			wd, err := os.Getwd()
			if err != nil {
				return err
			}
			workdir = wd
		}
	}
	var err error
	if workdir, err = filepath.Abs(workdir); err != nil {
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
