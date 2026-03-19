package application

import "fmt"

func printUsage() {
	fmt.Print(`dockpipe — Run, isolate, and act (Go). Pipe commands into disposable containers.

Usage:
  dockpipe [options] -- <command> [args...]
  dockpipe --workflow <name> [options] -- <command>
  dockpipe --workflow <name> [options]          multi-step (steps: in config.yml), optional -- for last step
  dockpipe init | action init | pre init | template init | windows setup

Options:
  --workflow, --run, --isolate, --act, --resolver, --repo, --branch, --workdir,
  --data-dir, --data-vol, --no-data, --reinit, -f, --mount, --env, --env-file,
  --var, --build, -d/--detach, -h/--help, --version / -v / -V

Requires: docker, bash (for pre-scripts). Config: YAML (steps:) parsed natively — no python3.

`)
}
