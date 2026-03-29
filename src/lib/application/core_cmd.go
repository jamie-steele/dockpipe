package application

import (
	"fmt"

	"dockpipe/src/lib/infrastructure"
)

const coreUsageText = `dockpipe core — resolve paths to bundled core assets (same namespace as workflow YAML)

Usage:
  dockpipe core script-path <dotted>

  dotted   Omit the scripts/ prefix — e.g. assets.scripts.terraform-pipeline.sh
           (same as scripts/core.assets.scripts.terraform-pipeline.sh in config.yml).

Environment: DOCKPIPE_REPO_ROOT overrides the bundled assets root (same as dockpipe run).

`

func cmdCore(args []string) error {
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" {
		fmt.Print(coreUsageText)
		return nil
	}
	switch args[0] {
	case "script-path":
		return cmdCoreScriptPath(args[1:])
	default:
		return fmt.Errorf("unknown core subcommand %q (try: dockpipe core --help)", args[0])
	}
}

func cmdCoreScriptPath(args []string) error {
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" {
		return fmt.Errorf("usage: dockpipe core script-path <dotted>")
	}
	dotted := args[0]
	if len(args) > 1 {
		return fmt.Errorf("unexpected argument %q", args[1])
	}
	rr, err := infrastructure.RepoRoot()
	if err != nil {
		return err
	}
	p, err := infrastructure.ResolveCoreNamespacedScriptPath(rr, dotted)
	if err != nil {
		return err
	}
	fmt.Println(p)
	return nil
}
