package application

import (
	"fmt"
	"os"

	"dockpipe/src/lib/dockpipe/infrastructure"
)

func cmdWorkflow(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf(`usage: dockpipe workflow validate [path]

  Validates workflow YAML (parse + JSON Schema). Default path: dockpipe.yml`)
	}
	switch args[0] {
	case "validate":
		path := "dockpipe.yml"
		if len(args) > 1 {
			path = args[1]
		}
		if err := infrastructure.ValidateWorkflowYAML(path); err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "OK: workflow %q\n", path)
		return nil
	default:
		return fmt.Errorf("unknown workflow subcommand %q (try: validate)", args[0])
	}
}
