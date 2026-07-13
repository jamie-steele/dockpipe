# TASK-014 Native Dev Container Discovery And Lifecycle

## Goal

Let Pipeon recognize a repository-owned `.devcontainer` definition and offer a governed way to
prepare, start, attach to, inspect, and stop that environment. The same lifecycle must be available
through a CLI/MCP contract so Pipeon is a consumer of the capability, not a second Dev Container
runtime.

## Current Context

The repository has experimental IDE session-container and Dev Containers-style remote flows under
`packages/ide/resolvers/`, but those create DockPipe-owned environments. They do not yet discover
or honor a user's `.devcontainer/devcontainer.json` (including named or multi-root definitions).

Pipeon already owns a separate local stack and provider-pool lifecycle. Native Dev Container support
must compose with that stack without treating the Dev Container as Pipeon's private state or silently
replacing a user's existing container session.

## Research Questions

- What is the supported cross-platform invocation and machine-readable output of the Dev Container
  CLI for `read-configuration`, `up`, `exec`, and lifecycle inspection on Windows, macOS, and Linux?
- How should discovery handle `.devcontainer/devcontainer.json`, alternate JSON files, and a
  workspace with multiple definitions without guessing which environment to start?
- Which lifecycle facts can be derived safely from Docker/Dev Container labels versus editor-only
  state, and how should an existing user-started environment be adopted or left alone?
- How do Docker Compose-based definitions, features, mounts, `remoteUser`, forwarded ports, and
  rebuild requirements map to bounded DockPipe operation-result events?
- Which operations require explicit approval: image pull/build, feature install, Compose startup,
  container stop/remove, rebuild, and opening a host editor attachment?
- How should Pipeon expose readiness, build progress, logs, container identity, attach targets, and
  repair actions while preserving the CLI as the execution authority?
- Can the DorkPipe provider pool safely use a ready Dev Container as a declared execution location,
  or must provider workers and the Dev Container remain separate until an explicit resolver contract
  exists?

## Proposed Product Shape

1. Discovery is read-only and automatic: Pipeon shows a `Dev Container available` state when a
   valid repository definition is found. It does not build or start anything merely because the
   folder exists.
2. Starting, rebuilding, stopping, or attaching is an explicit governed action. Pipeon renders the
   same request, risk, approval, and operation-result events as the CLI/MCP path.
3. The first CLI surface should be provider-neutral and lifecycle-oriented, for example
   `dockpipe devcontainer discover|status|up|stop|exec`, with exact command names deferred until
   research establishes the existing CLI vocabulary. It must accept an explicit definition when
   discovery finds more than one.
4. Pipeon consumes that contract to offer `Use Dev Container`, status, logs, attach/open, rebuild,
   and stop controls. It stores only UI selections and drafts locally; the repository's
   `.devcontainer` files remain the durable source of truth.
5. The lifecycle operation returns an opaque environment/session reference plus normalized state and
   artifact/log pointers. It does not expose raw Docker or Dev Container command payloads to other
   app layers.

## Safety And Boundary Rules

- Keep Dev Container-specific resolution, CLI integration, and Docker behavior package/resolver
  owned unless research identifies a genuinely generic DockPipe primitive.
- Never auto-run a discovered configuration. Builds, pulls, feature installation, Compose changes,
  stop/remove, rebuild, and host-editor launch require explicit intent and applicable approval.
- Respect the user's existing containers and labels. Do not stop, remove, or rebuild a container
  not proven to belong to the selected definition and requested DockPipe session.
- Do not copy repository contents into a Pipeon volume when the Dev Container contract already owns
  workspace mounting. Do not infer editor attachment state from unsupported host process heuristics.
- Treat secrets only as existing Dev Container references or governed secret references; never read
  or serialize resolved secret values into Pipeon state, artifacts, or events.
- Keep Pipeon UI, CLI, and MCP on one structured event/approval contract. No extension-only
  lifecycle implementation or durable Pipeon-specific Dev Container configuration.

## First Research Deliverables

- Compatibility matrix for Dev Container CLI, Docker Desktop, Docker Compose, and host editor
  attachment across supported host platforms.
- Inventory of the existing `packages/ide/resolvers/` flows, Pipeon stack lifecycle, and their
  overlap/conflicts with repository-owned `.devcontainer` definitions.
- Proposed normalized lifecycle state machine, operation-result schema, approval classes, ownership
  labels, and cleanup/recovery rules.
- CLI/MCP contract proposal with multi-definition selection and non-interactive fail-closed behavior.
- Pipeon UX wireflow showing discovery, explicit start, progress, attach, error/repair, and teardown.
- A minimal vertical-slice recommendation with tests that use fixture Dev Container definitions and
  no live image pull by default.

## Open Decisions

- Whether the first implementation wraps an installed Dev Container CLI, uses its reference library,
  or supports a limited direct Docker path behind the same resolver contract.
- Whether a started Dev Container becomes an eligible generic workflow runtime/resolver target or is
  initially limited to Pipeon/editor attachment and explicit CLI exec.
- How to model shared versus DockPipe-started environments, including handoff, detach, and cleanup.
- Which editor attachments are supported first: VS Code, Cursor, Pipeon code-server, or a
  container-only status/exec surface.
