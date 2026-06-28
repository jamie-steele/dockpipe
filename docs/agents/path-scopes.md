# Path Scopes

Read when moving generated files out of source control, migrating package/workflow artifacts, or
changing `cwd`, `scopes`, `outputs`, or `dockpipe scope` behavior.

## Default Flow

| Need | Use |
| --- | --- |
| Step only writes generated files | `cwd: artifacts`; use plain relative paths in the script. |
| Step must inspect/edit the checkout and write generated files | `cwd: repo` plus `scopes: { source: repo, artifacts: artifacts }`; use `dockpipe scope artifacts ...` for generated files. |
| Step needs checkout path explicitly | `dockpipe scope source ...`. |
| Workflow-run artifacts | `dockpipe scope artifacts ...`; do not write to repo-root `tmp/`, `.dockpipe/`, or package state. |
| Package-owned long-lived state | `dockpipe scope --package <name> ...`. |
| Resolver-owned auth/config paths | Declare them in the resolver profile and read with `dockpipe scope resolver <name> <field>`. |

## Migration Rules

- Prefer `cwd: artifacts` for simple producer steps so relative writes naturally land in workflow artifacts.
- Use `cwd: repo` only when the process must run from the checkout.
- Do not introduce package/workflow-specific env vars for artifact roots when `dockpipe scope` can resolve the path.
- Do not hardcode `bin/.dockpipe/...`, `tmp/...`, package names, workflow names, or resolver auth directories in scripts.
- Keep generated workflow outputs under workflow artifact scope, not package scope.
- Keep package state for package-owned caches, credentials, shared metrics, and durable package data.
- Keep resolver auth/config defaults in resolver profiles, not workflow scripts.

## Surface Sync

When changing scope semantics, update together:

- `src/lib/domain/workflow.go` and runner behavior
- `src/lib/application/sdk_cmd.go`
- `src/lib/infrastructure/schema/workflow.schema.json`
- `docs/workflow-yaml.md`
- `src/app/tooling/vscode-extensions/dockpipe-language-support/extension.js`
- affected package workflows/resolver profiles/tests

## Checks

- `./src/bin/dockpipe workflow validate <config.yml>`
- `./src/bin/dockpipe package test --workdir . --only <package>`
- `./src/bin/dockpipe package compile workflows --workdir . --from packages/<package> --force`
- `go test ./src/lib/domain ./src/lib/application` for scope/runner changes
