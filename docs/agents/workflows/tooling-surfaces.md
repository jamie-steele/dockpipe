# Tooling Surfaces

Read when changing DockPipe Language Support, the DorkPipe/Pipeon VS Code extension, PipeDeck,
model browser, template designer, run inspector, or any editor/app-facing workflow UX.

## Contract Rule

Workflow YAML is the durable contract. Tooling may make it easier to author, inspect, or launch, but
must not create a second durable workflow system.

PipeDeck follows the same rule. It is a modern control surface over YAML, package-owned catalogs,
launcher context, operation-result events, and artifacts. It does not own a separate execution or
configuration model.

## Layers

| Layer | Owns |
| --- | --- |
| DockPipe Language Support | YAML/schema completions, hovers, diagnostics, snippets. |
| DorkPipe/Pipeon extension | Chat, designer, model browser, run inspector, draft UI state. |
| PipeDeck | Launcher-context workflow control, YAML-backed authoring, run inspection, approvals, artifact/log views, diff preview. |
| DorkPipe package | Agent orchestration, model escalation, skills, artifacts, package-owned catalogs. |
| DockPipe engine | Generic workflow/package/runtime/resolver execution primitives. |

## PipeDeck

PipeDeck should be invoked by the DockPipe launcher, using the same context-passing model as Pipeon.
The launcher provides the current repo/workspace, selected workflow or package, session
identity, artifact root, allowed scopes, model/resolver lanes, and MCP connector availability.

PipeDeck should focus on governed workflow work, not full IDE replacement:

- create and edit workflows, agents, tasks, MCP connectors, model-lane policy, approval/apply
  policy, and package workflow references
- derive saved durable state from YAML and package-owned catalogs
- show generated YAML before save when using guided editors
- run workflows from the launcher context
- render operation-result events, task graph state, logs, artifacts, approvals, verifier findings,
  model usage, and apply/publish status
- preview generated diffs, apply targets, conflicts, and repair options
- map workflows and agents to `AGENTS.md`, `docs/agents/index.yaml`, routed markdown, TODOs, and
  rendered skills
- optionally expose the same app/server through Qt mobile, Cloudflare Tunnel, Let's Encrypt,
  free-domain, or bring-your-own-domain setup when explicitly approved

UI/chat surfaces should sit on top of the CLI/master protocol. They send requests, stream the same
structured events the CLI uses, render approval prompts, and forward approve/deny/escalate
decisions. A future UI should not fork execution semantics from CLI runs.

Deep code editing can remain in the user's normal editor. The app should provide enough diff,
artifact, and conflict context to decide what to apply, retry, repair, or defer.

Remote access and mobile access are governed setup surfaces. Creating tunnels, DNS records,
certificates, public endpoints, mobile auth, or remote app auth must be explicit, reversible, logged
through operation-result events, and backed by YAML/package config rather than UI-only state.

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
- If PipeDeck UX changes, document the launcher context, YAML mapping, event stream, and
  artifact/run-inspection behavior.

## Validation

- `npm run typecheck` in the extension package when TypeScript changes.
- `make package-dockpipe-language-support` when language support packaging changes.
- `./src/bin/dockpipe workflow validate <config.yml>` for changed workflow examples.
