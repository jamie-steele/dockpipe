# TASK-008 PipeDeck Agentic App UI

## Goal

Design and build PipeDeck, a standalone DockPipe-launched agentic app for creating, editing, running, and
inspecting DockPipe/DorkPipe workflows through a clean modern interface.

The app should make the YAML contracts approachable without replacing them. Workflow, agent, MCP,
model-lane, approval, and package contracts should still derive from durable YAML and package-owned
catalogs.

## Current Decisions

- Build PipeDeck as a standalone app launched by DockPipe, using the same launcher-context model as
  Pipeon.
- Treat the app as a control and inspection surface over DockPipe, not a second runtime.
- Keep YAML and package-owned catalogs as the durable source of truth.
- Use one shared local/server API for desktop Qt, Qt mobile, and web clients.
- Use the CLI/master protocol and operation-result stream for execution, approvals, logs, artifacts,
  and run state.
- Support remote access only as an explicit governed setup with approval, teardown, audit, and
  secret-reference handling.
- Provide diff, conflict, artifact, and repair review without becoming a full IDE.

## Product Shape

PipeDeck is a standalone application invoked through the same DockPipe launcher model as Pipeon. It
inherits execution context from the launcher: selected repo, workflow/package context, environment,
scopes, and runtime/session identity.

PipeDeck should be agentic, but not a full IDE. It should focus on governed workflow creation,
execution, review, and iteration.

Primary jobs:

- create and edit workflows
- create and edit agent/task definitions
- configure MCP connectors and capability declarations
- inspect model lanes, budgets, and escalation policy
- run workflows from the current launcher context
- review approvals, diffs, artifacts, logs, operation results, and follow-up tasks
- map agent-facing docs, markdown guidance, skills, and TODOs into a navigable view
- optionally expose the same app/server through a governed remote-access setup

## Contract Rule

YAML remains the source of truth.

The app may provide rich editors, forms, graph views, previews, and guided flows, but saved durable
state should round-trip through:

- workflow `config.yml`
- package-owned catalogs
- agent/task YAML
- MCP connector YAML
- model-lane policy YAML
- repo-owned task packs
- docs/agents indexes and markdown guidance

App-local state is acceptable for drafts, UI layout, selected views, cached metadata, and transient
chat context. It should not become the only durable definition of a workflow, agent, connector, or
approval policy.

## Launcher Integration

The app should launch from DockPipe using the same context-passing approach as Pipeon.

Expected launcher context:

- current repo/workspace
- selected workflow or package, when present
- active session/run identity, when present
- artifact root, operation-result stream, and event projection path, discoverable through
  `dockpipe get event_log`, `dockpipe get event_index`, workflow `dockpipe scope`, and
  `dockpipe session inspect --json`
- allowed scopes and access policy
- available resolver/model lanes
- MCP connector availability

The app should be able to start a new run or attach to an existing run from that context.

## Remote Access And Web App Mode

The same app/server should be able to run as a web-accessible control surface when the user opts in.
The desktop Qt app, Qt mobile app, and web app should speak to the same local/server API instead of
creating separate products.

Supported setup options should include:

- local-only desktop app
- local or remote Qt mobile app
- local web app bound to localhost
- Cloudflare Tunnel for remote access without directly opening inbound ports
- Let's Encrypt certificate setup when serving through a user-controlled host/domain
- free starter domain or subdomain flow where practical
- bring-your-own-domain setup with DNS guidance and verification

Remote access setup is security-sensitive and should be treated as a governed operation:

- show exactly what will be exposed before enabling it
- require explicit approval before creating tunnels, DNS records, certificates, or public endpoints
- emit operation-result events for tunnel creation, DNS verification, certificate issuance, server
  start, endpoint health, and shutdown
- support disable/teardown as a first-class operation
- persist remote-access config in YAML or package-owned config, never only in UI state
- keep secrets as references, not plaintext tokens in repo files

The remote web app should use the same approval, event, artifact, and operation-result contracts as
local CLI/app runs.

Mobile should follow the same rule. A Qt mobile app can provide a compact control and monitoring
surface for runs, approvals, artifacts, logs, and endpoint status, but durable workflow and connector
state should still come from YAML/package config through the shared API.

## Runtime UX

When a workflow runs, the app should show richer information over the same CLI/master protocol:

- live operation-result timeline
- rebuildable operation-event index summary from `dockpipe runs events --index`
- current stage, worker, and task graph state
- logs by operation/task
- artifact browser
- approval prompts and decisions
- model lane usage and budget state
- verifier findings and repair suggestions
- apply/publish status

The UI should render the same structured events emitted by CLI runs. It should not require a
different execution path.

## Authoring UX

Authoring should expose YAML contracts through purpose-built views:

- workflow graph and stage editor
- agent/task definition editor
- prompt and context editor with source/read/write policy visibility
- MCP connector editor
- model-lane and budget policy editor
- approval/apply/publish policy editor
- package workflow browser
- schema diagnostics and validation
- generated YAML preview before save

The app should make it easy to switch between guided UI and raw YAML for advanced users.

## Review UX

The app should support reviewing changes without becoming a full IDE:

- generated diff preview
- apply preview by target file
- conflict detection and merge-conflict guidance
- accept/reject/retry/repair controls for generated changes
- artifact-to-diff traceability
- checklist of required artifacts and verifier status
- final summary suitable for commit/PR notes

Git merge conflicts should be surfaced clearly with enough context to resolve or defer, but deep
code editing can remain in the user's normal editor.

## Agent Docs Map

The app should provide a map from agents/workflows to markdown guidance:

- `AGENTS.md`
- `docs/agents/index.yaml`
- routed docs under `docs/agents/`
- workflow/package README files
- task-pack docs
- TODO index and linked TODO markdown
- rendered skills and provider-facing guidance

This view should explain which guidance a run will load and where durable agent knowledge should be
updated after the run.

## First Slices

1. Define the app contract and launcher context payload.
2. Add a read-only run inspector over operation-result events, logs, and artifacts.
3. Add YAML-backed workflow/agent/MCP connector browsers with validation status.
4. Add diff/apply preview and approval rendering over the same CLI/master bridge protocol.
5. Add guided workflow/task-pack authoring that previews generated YAML before save.
6. Add the agent-docs map and TODO index view.
7. Add opt-in remote access setup for Cloudflare Tunnel, certificate/domain configuration, endpoint
   health, and teardown.
8. Add a Qt mobile client over the same app/server API for run monitoring, approvals, and lightweight
   workflow control.

## Still Open

- Decide where PipeDeck lives in the first-party package tree.
- Define the launcher context payload shared with Pipeon-style app launches.
- Decide which YAML contracts exist for MCP connectors and agent/task packs before building rich
  editors.
- Extend the initial `dockpipe.operation_event.v1` JSONL stream into the full PipeDeck run inspector
  feed, including logs, artifact references, approvals, and task graph state.
- Decide how much editing happens in-app versus handing off to the user's normal editor.
- Design conflict preview and repair flows without turning the app into a full IDE.
- Decide where remote-access YAML lives and how it maps to Cloudflare Tunnel, Let's Encrypt,
  free-domain/subdomain, and bring-your-own-domain flows.
- Define the threat model for exposing the app remotely, including auth, allowed operations,
  approval prompts, audit logs, and teardown.
- Decide the Qt mobile packaging path, authentication model, offline behavior, and which actions are
  safe to expose from mobile.
