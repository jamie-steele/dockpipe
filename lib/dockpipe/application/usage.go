package application

import "fmt"

func printUsage() {
	fmt.Print(`dockpipe — Run any CLI command in a disposable container; project at /work.

Usage:
  dockpipe [options] -- <command> [args...]
  dockpipe --workflow <name> [options] -- <command>
  dockpipe --workflow-file <path> [options]     workflow YAML (e.g. repo-root dockpipe.yml)
  dockpipe --workflow <name> [options]          multi-step (steps: in config.yml), optional -- for last step
  dockpipe workflow validate [path]             lint workflow YAML (default: dockpipe.yml)
  dockpipe init | action init | pre init | template init | windows setup
  dockpipe doctor                             verify bash, Docker, bundled assets

Options:
  --workflow, --workflow-file, --run, --isolate, --act, --runtime, --resolver, --strategy, --repo, --branch, --workdir,
  --data-dir, --data-vol, --no-data, --reinit, -f, --mount, --env, --env-file,
  --var, --build, -d/--detach, -h/--help, --version / -v / -V

Requires: docker, bash (host). Git (host) for worktree / --repo / commit-on-host. Config: YAML (steps:) parsed natively — no python3.

`)
}
