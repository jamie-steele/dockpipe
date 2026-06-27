# Package Promotion

Read when moving material from `.staging/` into tracked `packages/`.

## Goal

Promote only source-of-truth package content. Leave experiments, generated state, caches, and local secrets behind.

## Promote

- `package.yml`
- workflow `config.yml` and workflow-local scripts/docs
- resolver `config.yml`, `profile`, assets, README
- package tests
- curated prompts/skills/policies/examples that are meant to ship

## Do Not Promote

- generated `bin/.dockpipe/` or `.dorkpipe/` outputs
- local `.env` or resolved secret material
- caches, logs, temporary outputs
- shell scripts that encode one workflow shape when YAML should carry the contract
- stale staging-only references in docs

## Required Review

- Confirm package tree compiles from `packages/<name>`.
- Confirm staging copies are either intentionally left for a later cleanup or explicitly removed by request.
- Search for old workflow/script names after deletion.
- Verify docs describe package boundaries, not checkout-only paths as a public contract.

## Checks

- `find packages/<name> -type f | sort`
- `rg -n "<old-name>|<old-script>" packages docs src workflows .github -S`
- `./src/bin/dockpipe package test --workdir . --only <name>`
- `./src/bin/dockpipe package compile workflows --workdir . --from packages/<name> --force`
- `./src/bin/dockpipe package compile resolvers --workdir . --from packages/<name> --force`
