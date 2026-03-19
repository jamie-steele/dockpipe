# dockpipe Go packages (DDD-style layout)

| Layer | Package | Responsibility |
|-------|---------|----------------|
| **Domain** | `dockpipe/lib/dockpipe/domain` | Workflow/step model, YAML parse from **bytes** (`ParseWorkflowYAML`), env merge helpers, resolver key semantics, branch-prefix rules. No `docker` / subprocess / file I/O in non-test code. |
| **Infrastructure** | `dockpipe/lib/dockpipe/infrastructure` | Filesystem, `docker`, `bash` pre-scripts, git commit-on-host, repo root discovery, `.env` files, template→image paths, version tags. |
| **Application** | `dockpipe/lib/dockpipe/application` | CLI flags, subcommands (`init`, `template`, …), and the **run** use-case that wires domain + infrastructure. |

`cmd/dockpipe` is a thin entrypoint that calls `application.Run`.

### Application package files (baseline)

Keep new orchestration in the right file so `run.go` stays the single-command path only:

| File | Role |
|------|------|
| `run.go` | `Run()` — main CLI flow for `dockpipe [opts] -- cmd` |
| `run_steps.go` | Multi-step workflow loop (`steps:` in config) |
| `workflow_env.go` | Workflow/env merge helpers (`.env`, outputs, branch prefix, `--var` locks) |
| `flags.go` | `CliOpts`, `ParseFlags` |
| `subcmds.go` | `init`, `action`, `pre`, `template` |
| `usage.go` | `--help` text |

Shell assets (`lib/entrypoint.sh`, etc.) stay alongside this tree; only **Go** lives under `lib/dockpipe/`.
