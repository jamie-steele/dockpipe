package application

import (
	"fmt"
	"os"
	"strings"
)

var (
	runPackageTestFromFlagsFn   = RunPackageTestFromFlags
	runWorkflowTestsFromFlagsFn = RunWorkflowTestsFromFlags
)

func cmdTest(args []string) error {
	if len(args) > 0 {
		switch args[0] {
		case "--help", "-h":
			fmt.Print(testUsageText)
			return nil
		case "package":
			return cmdPackageTest(args[1:])
		case "workflow":
			return cmdWorkflowTest(args[1:])
		}
	}
	var (
		workdir      string
		only         string
		runPackages  = true
		runWorkflows = true
	)
	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "--workdir" && i+1 < len(args):
			workdir = args[i+1]
			i++
		case args[i] == "--only" && i+1 < len(args):
			only = args[i+1]
			i++
		case args[i] == "--packages":
			runPackages = true
		case args[i] == "--no-packages":
			runPackages = false
		case args[i] == "--workflows":
			runWorkflows = true
		case args[i] == "--no-workflows":
			runWorkflows = false
		case strings.HasPrefix(args[i], "-"):
			return fmt.Errorf("unknown option %s (try: dockpipe test --help)", args[i])
		default:
			return fmt.Errorf("unexpected argument %q", args[i])
		}
	}
	if !runPackages && !runWorkflows {
		return fmt.Errorf("dockpipe test: nothing selected (both packages and workflows are disabled)")
	}
	if workdir == "" {
		wd, err := os.Getwd()
		if err != nil {
			return err
		}
		workdir = wd
	}
	if runPackages {
		if err := runPackageTestFromFlagsFn(workdir, only); err != nil {
			return err
		}
	}
	if runWorkflows {
		if err := runWorkflowTestsFromFlagsFn(workdir, only); err != nil {
			return err
		}
	}
	return nil
}

const testUsageText = `dockpipe test

Run DockPipe-owned test hooks from the current project/workdir.

By default this runs:
  - package tests declared via package.yml test.script
  - workflow-local tests under workflows that contain tests/run.*

Usage:
  dockpipe test [--workdir <path>] [--only <name>] [--packages|--no-packages] [--workflows|--no-workflows]
  dockpipe test package [--workdir <path>] [--only <package>]
  dockpipe test workflow [--workdir <path>] [--only <workflow>]

Options:
  --workdir <path>   Project/worktree root (default: current directory)
  --only <name>      Run one package/workflow name when applicable
  --packages         Include package tests (default: on)
  --no-packages      Skip package tests
  --workflows        Include workflow-local tests (default: on)
  --no-workflows     Skip workflow-local tests
`
