# YAML Workflows

Read when editing workflow YAML, schema, runner semantics, or examples.

## Workflow Rules

- Workflows define what happens.
- Runtime defines where execution runs.
- Resolver defines which tool/profile performs the work.
- Strategy defines lifecycle wrapping.
- Packaged workflow calls use `workflow:` plus `package:`.

## Step Rules

| Field | Rule |
| --- | --- |
| `kind: host` | Runs on host. Do not set runtime/resolver/isolate on host steps. |
| `cwd` | Step working directory. Use `artifacts` when relative generated outputs should land under workflow state, not source control. |
| `runtime` | Execution substrate profile. |
| `resolver` | Tool/profile selection. |
| `isolate` | Low-level image/template override. Prefer runtime/resolver first. |
| `outputs` | Dotenv file merged into later steps. |
| `group: { mode: async }` | Async task batch. Avoid old plain-step `is_blocking: false`. |
| `workflow` + `package` | Packaged child workflow invocation. |
| `agent` | DorkPipe-consumed agentic declaration, not core AI behavior. |

## Surface Sync

When changing authored workflow/config surfaces, update in the same change:

- Go domain structs and validation
- JSON schema at `src/lib/infrastructure/schema/workflow.schema.json`
- DockPipe Language Support at `src/app/tooling/vscode-extensions/dockpipe-language-support/extension.js`
- docs near `docs/workflow-yaml.md`
- package/workflow examples affected by the change

## Template Development Rule

When working on templates/workflows, act as a DockPipe user:

- allowed: YAML, scripts, images, docs
- not allowed: template-specific core logic
- if blocked: propose a general primitive

## Checks

- `./src/bin/dockpipe workflow validate <config.yml>`
- `./src/bin/dockpipe package compile workflows --workdir . --from <package-or-root> --force`
- `go test ./src/lib/...` for runner/schema changes
