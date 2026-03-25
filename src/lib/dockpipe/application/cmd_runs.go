package application

import (
	"fmt"
	"os"

	"dockpipe/src/lib/dockpipe/infrastructure"
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
	if sub != "list" {
		return fmt.Errorf("dockpipe runs: unknown subcommand %q (try: list)", sub)
	}
	rest := argv[1:]
	workdir := ""
	for i := 0; i < len(rest); i++ {
		switch rest[i] {
		case "--workdir":
			if i+1 >= len(rest) {
				return fmt.Errorf("dockpipe runs list: --workdir requires a path")
			}
			workdir = rest[i+1]
			i++
		default:
			return fmt.Errorf("dockpipe runs list: unexpected argument %q", rest[i])
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
	return infrastructure.ListHostRuns(workdir, os.Stdout)
}
