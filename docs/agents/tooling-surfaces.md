# Tooling Surfaces

Read when changing DockPipe Language Support, the DorkPipe/Pipeon VS Code extension, model browser,
template designer, run inspector, or any editor-facing workflow UX.

## Contract Rule

Workflow YAML is the durable contract. Tooling may make it easier to author, inspect, or launch, but
must not create a second durable workflow system.

## Layers

| Layer | Owns |
| --- | --- |
| DockPipe Language Support | YAML/schema completions, hovers, diagnostics, snippets. |
| DorkPipe/Pipeon extension | Chat, designer, model browser, run inspector, draft UI state. |
| DorkPipe package | Agent orchestration, model escalation, skills, artifacts, package-owned catalogs. |
| DockPipe engine | Generic workflow/package/runtime/resolver execution primitives. |

## Rules

- Round-trip template/designer data through `config.yml` fields such as `model_policy`,
  `steps[].agent`, orchestration tasks, access, verification, and approval.
- Treat model-browser entries as DorkPipe escalation lanes with availability, capability, context,
  local/cloud, and budget metadata.
- Keep provider-specific behavior in package assets/resolvers, not `src/lib/` or `src/cmd/`.
- User-created GUI templates and model-browser entries should live in user/global extension storage;
  chat sessions can stay workspace-local.
- Extension-local state is only for drafts, caches, active selections, and UI preferences.
- Repo files should change only when the user exports/saves a `config.yml` workflow or edits package
  assets explicitly.
- If the YAML surface changes, update DockPipe Language Support in the same change.
- If the rich extension UX changes, document how it maps back to YAML and package-owned catalogs.

## Validation

- `npm run typecheck` in the extension package when TypeScript changes.
- `make package-dockpipe-language-support` when language support packaging changes.
- `./src/bin/dockpipe workflow validate <config.yml>` for changed workflow examples.
