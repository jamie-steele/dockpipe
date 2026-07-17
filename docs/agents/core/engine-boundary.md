# Engine Boundary

Read before editing `src/lib/` or `src/cmd/`.

## Boundary

`src/lib/` and `src/cmd/` are the engine. They must not know this checkout's package/workflow/staging layout.

## Allowed In `src/`

- generic workflow resolution
- generic resolver/runtime/profile loading
- generic script path resolution
- package manifest and install wire shapes
- project-local package store paths through infrastructure helpers
- global paths through global helper functions
- one embed-root indirection through the existing embedded FS helpers

## Forbidden In `src/`

- hardcoded repo-relative `packages/<name>/...`
- hardcoded repo-root `workflows/<name>/...`
- any `.staging/...` dependency or user-facing prescription
- maintainer-only workflow/resolver names in control flow, tests, or user-facing strings
- product-specific logic for Codex, Claude, DorkPipe, Pipeon, or a package workflow
- "the way to do X is workflow foo under packages/..." baked into engine behavior

## Package/Workflow Interaction

Treat `packages/`, `workflows/`, and `.staging/` as separate products. They interact with the engine only through:

- compile into `bin/.dockpipe/internal/packages/`
- package/store tarballs
- embed roots listed centrally
- declarative YAML fields already supported by the runner
- public CLI behavior

## If You Need More

If a workflow/package cannot express something, propose a general primitive. Do not add a special case.

## Checks

- `rg -n "packages/|\\.staging/|workflows/" src/lib src/cmd`
- `go test ./src/lib/...`
- update docs/schema/language support for authored surface changes
