# DorkPipe Package Authoring

Use this skill for package tree changes.

## Package Boundary

Packages ship YAML, assets, resolver/runtime wiring, prompts, skills, policies, examples, and tests.
They do not inject product-specific behavior into DockPipe core.

## Hard Rules

- Keep package logic inside the package tree.
- Prefer repo-local build outputs before `PATH` for first-party binaries.
- Use shared SDK helpers where possible.
- Do not copy local caches, env files, generated state, or binaries unless the package explicitly owns them.
- Keep package tests self-contained.

## Checks

- `dockpipe package test --workdir . --only <package>`
- `dockpipe package compile workflows --workdir . --from packages/<package> --force`
- `dockpipe package compile resolvers --workdir . --from packages/<package> --force`
