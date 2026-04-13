package application

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"dockpipe/src/lib/domain"
	"dockpipe/src/lib/infrastructure"
)

func cmdWorkflow(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf(`usage: dockpipe workflow validate [path]
       dockpipe workflow list [--workdir <path>] [--format json|text]

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
	case "list":
		format := "text"
		workdir := ""
		for i := 1; i < len(args); i++ {
			switch args[i] {
			case "--workdir":
				if i+1 >= len(args) {
					return fmt.Errorf("--workdir requires a path")
				}
				workdir = args[i+1]
				i++
			case "--format":
				if i+1 >= len(args) {
					return fmt.Errorf("--format requires json or text")
				}
				format = strings.ToLower(strings.TrimSpace(args[i+1]))
				i++
			case "--help", "-h":
				fmt.Print(`dockpipe workflow list [--workdir <path>] [--format json|text]

Print the workflow catalog resolved by DockPipe for the given project/workdir.
`)
				return nil
			default:
				if strings.HasPrefix(args[i], "-") {
					return fmt.Errorf("unknown option %s", args[i])
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
		projectRoot, err := domain.FindProjectRootWithDockpipeConfig(workdir)
		if err != nil {
			return err
		}
		out, err := buildCatalogListOutput(projectRoot, workdir)
		if err != nil {
			return err
		}
		switch format {
		case "json":
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(out.Workflows)
		case "text":
			for _, wf := range out.Workflows {
				fmt.Fprintf(os.Stdout, "%s\t%s\t%s\t%s\n", wf.WorkflowID, wf.DisplayName, wf.Category, wf.ConfigPath)
			}
			return nil
		default:
			return fmt.Errorf("unknown --format %q (use json or text)", format)
		}
	default:
		return fmt.Errorf("unknown workflow subcommand %q (try: validate or list)", args[0])
	}
}
