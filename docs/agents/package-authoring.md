# Package Authoring

Read when editing `packages/` or package-owned workflows/resolvers/assets.

## Package Boundary

Packages are self-contained YAML plus assets plus resolver/runtime wiring. They cannot inject engine primitives without a separate core change.

## Hard Rules

- Keep package logic inside the package tree.
- Do not make maintainer/dev flows depend silently on whatever happens to be on `PATH`.
- Prefer real repo-local build outputs first, then fall back to `PATH`.
- Prefer shared SDK helpers under `src/core/assets/scripts/lib/` instead of copying lookup logic.
- In package workflow YAML, reference local package files by the path the author sees, such as
  `assets/scripts/run.sh` or `../assets/scripts/run.sh`. Use `scripts/<name>/...` only when
  intentionally calling a shared/cross-package script namespace.
- For workflow scripts, prefer direct `dockpipe scope ...` calls for source/artifact/package/resolver paths.
- For reusable shell libraries, prefer `eval "$(dockpipe sdk)"` and `dockpipe_sdk ...` actions.
- Keep resolver-owned auth/config defaults in resolver profiles and read them with `dockpipe scope resolver <name> <field>`.
- Keep package tests self-contained.

## Repo-Local Binary Preferences

| Binary | Preferred repo-local path |
| --- | --- |
| `dockpipe` | `src/bin/dockpipe` |
| `dorkpipe` | `packages/dorkpipe/bin/dorkpipe` |
| `mcpd` | `packages/dorkpipe-mcp/bin/mcpd` |
| `pipeon` | `packages/pipeon/resolvers/pipeon/bin/pipeon` |
| `pipeon-desktop` | `packages/pipeon/apps/pipeon-desktop/bin/pipeon-desktop` |

## Checks

- `./src/bin/dockpipe package test --workdir . --only <package>`
- `./src/bin/dockpipe package compile workflows --workdir . --from packages/<package> --force`
- `./src/bin/dockpipe package compile resolvers --workdir . --from packages/<package> --force`
