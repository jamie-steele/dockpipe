package application

import (
	"fmt"
	"os"

	"dockpipe/src/lib/dockpipe/infrastructure"
)

func cmdWorkflow(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf(`usage: dockpipe workflow validate [path]

  Validates workflow YAML (parse + JSON Schema). Relative paths resolve from the current directory, then from the repo root (DOCKPIPE_REPO_ROOT / materialized bundle). Omit path when exactly one workflows/*/config.yml exists under the workflows root.`)
	}
	switch args[0] {
	case "validate":
		path := ""
		if len(args) > 1 {
			path = args[1]
		}
		resolved, err := infrastructure.ResolveWorkflowYAMLPath(path)
		if err != nil {
			return err
		}
		if err := infrastructure.ValidateResolvedWorkflowYAML(resolved); err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "OK: workflow %q\n", resolved)
		return nil
	default:
		return fmt.Errorf("unknown workflow subcommand %q (try: validate)", args[0])
	}
}
