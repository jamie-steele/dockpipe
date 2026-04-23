package application

import (
	"fmt"
	"os"

	"dockpipe/src/lib/infrastructure"
)

func cmdRuns(argv []string) error {
	for _, a := range argv {
		if a == "-h" || a == "--help" {
			fmt.Print(runsUsageText)
			return nil
		}
	}
	sub := "list"
	if len(argv) > 0 {
		sub = argv[0]
	}
	switch sub {
	case "list", "policy", "decisions":
	default:
		return fmt.Errorf("dockpipe runs: unknown subcommand %q (try: list or policy)", sub)
	}
	publicSub := sub
	if publicSub == "decisions" {
		publicSub = "policy"
	}
	rest := argv[1:]
	workdir := ""
	policyOpts := infrastructure.RunPolicyListOptions{}
	for i := 0; i < len(rest); i++ {
		switch rest[i] {
		case "--workdir":
			if i+1 >= len(rest) {
				return fmt.Errorf("dockpipe runs %s: --workdir requires a path", publicSub)
			}
			workdir = rest[i+1]
			i++
		case "--workflow":
			if publicSub != "policy" {
				return fmt.Errorf("dockpipe runs %s: --workflow is only valid with policy", publicSub)
			}
			if i+1 >= len(rest) {
				return fmt.Errorf("dockpipe runs policy: --workflow requires a value")
			}
			policyOpts.WorkflowName = rest[i+1]
			i++
		case "--step":
			if publicSub != "policy" {
				return fmt.Errorf("dockpipe runs %s: --step is only valid with policy", publicSub)
			}
			if i+1 >= len(rest) {
				return fmt.Errorf("dockpipe runs policy: --step requires a value")
			}
			policyOpts.StepID = rest[i+1]
			i++
		case "--json":
			if publicSub != "policy" {
				return fmt.Errorf("dockpipe runs %s: --json is only valid with policy", publicSub)
			}
			policyOpts.JSON = true
		default:
			return fmt.Errorf("dockpipe runs %s: unexpected argument %q", publicSub, rest[i])
		}
	}
	if workdir == "" {
		if w := os.Getenv("DOCKPIPE_WORKDIR"); w != "" {
			workdir = w
		} else {
			wd, err := os.Getwd()
			if err != nil {
				return err
			}
			workdir = wd
		}
	}
	if publicSub == "policy" {
		return infrastructure.ListRunPolicyRecords(workdir, os.Stdout, policyOpts)
	}
	return infrastructure.ListHostRuns(workdir, os.Stdout)
}
