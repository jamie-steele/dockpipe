package application

import "fmt"

const mainUsageText = `  ____             _     ____  _            
 |  _ \  ___   ___| | __|  _ \(_)_ __   ___ 
 | | | |/ _ \ / __| |/ /| | | | | '_ \ / _ \
 | |_| | (_) | (__|   < | |_| | | |_) |  __/
 |____/ \___/ \___|_|\_\|____/|_| .__/ \___|
                               |_|          

DockPipe — run anything, anywhere, in isolation.

Usage:
  dockpipe [options] -- <command> [args...]
  dockpipe --workflow <name> [options] [--] [args for last step]

Core:
  --workflow <name>     Workflow to run (e.g. test)
  --runtime <name>      Runtime profile (e.g. docker)
  --resolver <name>     Resolver profile (optional)

Optional:
  --workdir <path>      Host directory mounted at /work (default: current dir)
  --isolate <name>      Image or template for the container
  --                    End of dockpipe flags; rest goes to your command

More flags:
  --workflow-file, --run, --act, --strategy, --repo, --branch, --mount
  --env, --env-file, --var, --data-dir, --data-vol, --no-data, --reinit, -f, -d/--detach

Commands:
  init                    Add DockPipe to the current project
  workflow validate       Check workflow YAML (default: dockpipe.yml)
  doctor                  Check docker, bash, and bundled assets
  windows setup|doctor    Windows: optional WSL bridge setup
  action|pre|template init  Copy sample scripts (use each with --help)

Examples:
  dockpipe init
  dockpipe --workflow test --runtime docker
  dockpipe --workflow test --runtime docker -- go test ./...

Requires: docker and bash on the host. Git for --repo and worktree-style flows.
Workflows: YAML. See docs/cli-reference.md for every flag.
`

const initUsageText = `dockpipe init

Add DockPipe to the current project (scripts, templates, shared core files).

Usage:
  dockpipe init [flags]
  dockpipe init <name> [flags]   add a workflow from a template

Flags:
  --from <source>          Template to copy from (with <name>)
  --runtime <name>         Written into new config (with <name>)
  --resolver <name>        Written into new config (with <name>)
  --strategy <name>        Written into new config (with <name>)
  --gitignore              Append a marked block to .gitignore at the git repo root (idempotent; requires a git working tree)

Examples:
  dockpipe init
  dockpipe init --gitignore
  dockpipe init my-pipeline --from run-apply --resolver codex --runtime docker
`

func printUsage() {
	fmt.Print(mainUsageText)
}

func printInitUsage() {
	fmt.Print(initUsageText)
}
