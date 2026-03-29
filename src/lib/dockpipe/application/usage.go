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
  --workflow-file, --run, --act, --strategy, --repo, --branch,   --mount
  --workflows-dir <path>  Repo-relative or absolute root for named workflows (default: workflows/; env: DOCKPIPE_WORKFLOWS_DIR)
  --env, --env-file, --var, --data-dir, --data-vol, --no-data, --reinit, -f, -d/--detach

Commands:
  init                    Add DockPipe to the current project
  install                 Fetch templates/core from HTTPS (e.g. Cloudflare R2); see install core --help
  clone <name>            Copy a compiled workflow package to workflows/ when allow_clone is true (see package manifest)
  build                   Compile packages into .dockpipe/internal (same as package compile all --force; replaces existing outputs)
  clean                   Remove compiled package store (.dockpipe/internal/packages)
  rebuild                 clean then build
  package list|manifest|build|compile   Packages: list, manifest, author core tarball, or compile into .dockpipe/internal
  compile                 Same as dockpipe package compile (core, resolvers, workflows)
  release upload          Upload a file to S3-compatible storage (self-hosted; uses aws CLI)
  workflow validate       Check workflow YAML ([path] relative to cwd or repo root; omit if one workflow)
  doctor                  Check docker, bash, and bundled assets
  core script-path <dots> Print absolute path to a core asset (same as scripts/core.<dots> in YAML)
  terraform pipeline-path Print absolute path to terraform-pipeline.sh (core assets)
  runs list [--workdir]   List active host-run records under .dockpipe/runs/
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

Project setup in the current directory, or add a new workflow.

Usage:
  dockpipe init [flags]              merge templates/core, scripts/, images/, dockpipe.config.json (no workflow name)
  dockpipe init <name> [flags]       create workflows/<name>/config.yml as an empty starter (see --workflows-dir)
  dockpipe init <name> --from <src>  copy a bundled template or filesystem path into workflows/<name>/

Flags:
  --from <source>          With <name>: copy from blank (same as default), init, run, run-apply, a path, …
  --workflows-dir <path>   With <name>: repo-relative or absolute directory for named workflows (default: workflows). Same as DOCKPIPE_WORKFLOWS_DIR for dockpipe run.
  --runtime <name>         Written into new config (with <name>)
  --resolver <name>        Written into new config (with <name>)
  --strategy <name>        Written into new config (with <name>)
  --gitignore              Append a marked block to .gitignore at the git repo root (idempotent; requires a git working tree)

Examples:
  dockpipe init
  dockpipe init --gitignore
  dockpipe init my-pipeline
  dockpipe init my-pipeline --from run-apply --resolver codex --runtime docker
  dockpipe init my-starter --from init
`

func printUsage() {
	fmt.Print(mainUsageText)
}

func printInitUsage() {
	fmt.Print(initUsageText)
}

const runsUsageText = `dockpipe runs list — show host-run registry entries

While a skip_container workflow step runs a host script, dockpipe may write
workdir/.dockpipe/runs/<id>.json (and optional sidecars). This command lists
those JSON files.

Usage:
  dockpipe runs list [--workdir <path>]

  --workdir   Project directory (default: DOCKPIPE_WORKDIR or current directory)

`
