# Repo Map

Read when orienting in the DockPipe checkout.

## Core Paths

| Path | Purpose |
| --- | --- |
| `src/lib/` | DockPipe engine library. Must stay generic. |
| `src/cmd/` | DockPipe CLI entrypoint. Must stay generic. |
| `src/core/` | Bundled core authoring: runtimes, resolvers, strategies, assets, and shipped example workflows. |
| `workflows/` | This repo's lean CI/dogfood workflows. Not an engine contract. |
| `packages/` | First-party package authoring trees. Treat each like a separate product repo. |
| `.staging/` | Maintainer packaging and experiments. Not an engine contract. |
| `bin/.dockpipe/` | Generated project-local state, compiled packages, handoffs, metrics. |
| `.dorkpipe/` | Generated DorkPipe handoffs/analysis where present. |
| `docs/` | Human docs and agent routing. |
| `docs/TODO.md` | Cross-cutting backlog for operational/runtime follow-ups; update it when work materially completes or advances one of its items. |

## Generated State

Use generated artifacts as read-only grounding unless the user asks to refresh them.

| Generated path | Notes |
| --- | --- |
| `bin/.dockpipe/internal/packages/` | Project-local compiled package store. |
| package scope (`dockpipe scope --package <name> ...`) | Package runtime state/artifacts. |
| `bin/.dockpipe/runs/` | Host step run records. |
| `.dorkpipe/` | Optional DorkPipe analysis/handoff state. |

## Fast Orientation

- Architecture terms: `docs/agents/architecture.md`
- Engine boundary: `docs/agents/engine-boundary.md`
- Package/store model: `docs/agents/core-package-model.md`
- Validation commands: `docs/agents/validation-commands.md`

## Internal Workflow Locations

| Location | Use |
| --- | --- |
| `src/core/workflows/<name>/` | Bundled reusable examples only. |
| `workflows/<name>/` | This repo's lean CI/dogfood workflows. |
| `.staging/...` | Maintainer packaging and experiments. |
| `packages/<name>/workflows/` | Package-owned workflows. |

Do not put this repo's CI/demo/internal automation in `src/core/workflows/`.
First-party workflow scripts belong beside the workflow `config.yml`, not in repo-root shadow script trees.

## Editor Mirrors

If a local editor-rule mirror of `AGENTS.md` exists, keep it in sync. Do not assume every checkout has one.
