# Core And Package Model

Read when touching package resolution, compile/install flows, state paths, or binary lookup.

## Stores

| Store | Path | Meaning |
| --- | --- | --- |
| Project-local compiled store | `bin/.dockpipe/internal/packages/` | Workflows, resolvers, core slices built for this project. |
| Project runtime state | `bin/.dockpipe/packages/` | Package-scoped run artifacts and state. |
| Global install | `GlobalDockpipeDataDir()` / `DOCKPIPE_GLOBAL_ROOT` | User/machine shared package/core installs. |
| Authoring trees | `src/core/`, `workflows/`, `packages/`, legacy `templates/` | Editable source trees. |

## Hard Rules

- In Go, derive project paths from `infrastructure.DockpipeDirRel`, `StateRoot`, `PackagesRoot`, and related helpers.
- In Go, derive global paths from `GlobalDockpipeDataDir` and global helper functions.
- Do not hand-write bare `.dockpipe/internal` paths.
- Use `dockpipe scope artifacts ...` for workflow-run artifacts and `dockpipe scope --package <name> ...` for package-owned state.
- Runtime resolution uses compiled packages and configured roots, not hardcoded checkout paths.
- Published compiled artifacts are the official versioned reference for external consumers.

## `dockpipe init`

- Bare `dockpipe init` creates a minimal project scaffold in the current directory.
- It creates `workflows/`, `README.md`, `dockpipe.config.json`, and `.env.vault.template.example` when missing.
- If no workflows exist yet, it seeds `workflows/example/config.yml`.
- `dockpipe init <name>` creates `workflows/<name>/config.yml` as a minimal workflow.
- `dockpipe init <name> --from <template>` copies a bundled/filesystem workflow tree.
- It does not clone Git repos and does not copy `templates/core/`, `scripts/`, or `images/` by default.

## Canonical Docs

- `docs/package-model.md`
- `docs/core-vs-packages-audit.md`
- `docs/cli-reference.md`
- `docs/templates-core-assets.md`
