# TASK-012 Pipeon Startup, Provisioning, And Provider Pools

## Goal

Make Pipeon open quickly from a normal developer machine while preserving the current local-first,
offline-capable stack.

Generalize warm provider execution into a DorkPipe provider-pool capability that can be used by
Pipeon, CLI workflows, and future app surfaces. Pipeon should consume the pool; it should not own the
pooling model.

Pipeon startup should prefer already-materialized state: repo bind mounts, stable package state under
`bin/.dockpipe`, cached Go/npm/Cargo outputs, existing Docker images, existing containers when safe,
and already-pulled Ollama models.

## Current Startup Cost Centers

- Branded `dockpipe-code-server:latest` image refresh: packaging VSIX files and rebuilding the image
  is correct but expensive when extension inputs changed.
- DorkPipe stack image refresh: Linux `dockpipe`, `dorkpipe`, and `mcpd` binaries are built for the
  container image and copied into a Docker build context.
- Docker image availability: first launch pays base image pulls and layer extraction.
- Ollama model provisioning: model pulls dominate startup when the model is absent or Docker volume
  state was reset.
- Signature checks: recursive source hashing is safer than mtime-only checks, but it is still a
  startup tax on large extension/source trees.
- Host bridge setup: MCP bridge restart is cheap, but stale allowlists must be detected before reuse.
- Code-server container setup: bind mounting is the right workspace model; copying the repo into a
  volume should not be part of normal Pipeon startup.

## Immediate Optimizations

- Skip `ollama pull <model>` when `ollama list` already shows the requested model.
- Keep non-interactive Windows PowerShell calls hidden so startup does not flash transient consoles.
- Reuse the host MCP bridge only when its tool catalog exposes the required tools for the current
  Pipeon build.
- Keep Go/npm/Cargo caches under package/build state, not global `.gocache` or `.gotemp` paths.
- Keep the workspace bind-mounted so repeated launches reuse the same repo and package state.

## DorkPipe Provider Pools

Provider pools are a DorkPipe orchestration feature. They keep a bounded number of provider workers
ready for low-latency top-level orchestration while preserving DockPipe's governed runtime boundary.

Core intent:

- DorkPipe owns provider identity, pool lifecycle, provider availability, session affinity, queueing,
  spend limits, auth state, worker health, and workflow escalation policy.
- DockPipe remains the generic spawn/run/act engine. Do not add Pipeon-, Claude-, Codex-, or
  Ollama-specific behavior to `src/lib/` or `src/cmd/`.
- Pipeon, CLI workflows, and future UI surfaces call the same DorkPipe pool contract instead of each
  inventing provider routing.
- Pools are for top-level low-latency orchestrators and reusable direct workers. YAML workflows remain
  the durable contract for fan-out, validation DAGs, isolated edits, approvals, and release/CI work.

### Warm Provider Lanes

Direct chat and direct CLI orchestrator prompts should not cold-start provider workers for every
prompt.

Desired steady-state model:

- `ollama` stays up as the local model service for cheap/local direct chat and workflow lanes.
- `codex` stays available as a direct host-provider lane. It can use Codex's own host sandbox/session
  model and should preserve the Pipeon session binding so follow-up prompts resume cheaply.
- `claude` should have a warm guarded lane owned by the Pipeon/DorkPipe stack. Direct Claude chat
  should send prompts to that already-running boundary instead of invoking
  `dockpipe --package agent --workflow claude ...` for every message.
- Agentic workflows remain separate. When a direct provider decides a task needs fan-out, validation
  DAGs, or isolated edits, it escalates into YAML-backed DorkPipe workflows; the top-level direct
  chat providers do not become the workflow engine.

Current gap:

- Claude direct chat currently routes through the guarded DockPipe workflow boundary per prompt. That
  preserves the resolver/auth/container boundary, but it creates a container, runs one prompt, returns,
  and tears down. This is correct for cold workflow execution but too slow for a top-level chat lane.

Implementation direction:

- Add package-owned warm provider services or worker processes under the DorkPipe package/provider
  layer. Pipeon composes those services into its stack as one consumer.
- Mount the same repo/workspace bind and resolver auth/config paths that the one-shot `claude`
  workflow uses today.
- Expose a small guarded request API through the existing host MCP bridge or the isolated DorkPipe MCP
  boundary. The bridge should preserve the same approval/auth checks and never expose raw unrestricted
  host execution.
- Do not assume the provider CLI will discover DockPipe/MCP from a bind mount. The pool manager must
  inject the explicit contract: provider identity, workspace path, allowed tools, host-bridge
  endpoint if permitted, workflow escalation rules, and approval boundaries.
- Keep provider availability visible in the Pipeon dropdown: disabled until the corresponding host or
  stack lane is present/authenticated, with an actionable setup message.
- Add coarse latency metrics for direct-provider calls: queue time, provider command time, container
  startup time if any, and total elapsed time. A warm direct prompt should make cold-start costs
  obvious if they regress.
- Keep cold workflow execution available for agentic runs, fan-outs, one-off isolated worktrees, and
  release/CI use cases.

### Pool Contract

Warm lanes should generalize into provider pools.

Pool model:

- `provider_pool.<name>.min_ready` keeps a small number of workers warm while the Pipeon stack is
  open. Example: one Claude worker for direct chat, one Ollama service, zero or one Codex worker
  depending on whether Codex exposes a worthwhile persistent protocol.
- `provider_pool.<name>.max_active` caps concurrent workers so fan-out cannot silently explode cloud
  spend or host resource use.
- `provider_pool.<name>.idle_ttl` drains unused workers after a quiet period, unless the lane is marked
  sticky for an active chat session.
- `provider_pool.<name>.session_affinity` controls whether follow-up prompts stay on the same worker
  for context/memory, or whether prompts can be routed to any ready worker.
- `provider_pool.<name>.role` separates direct chat workers from workflow fan-out workers. Direct chat
  should be low-latency and session-affine; workflow workers can be short-lived, pooled, or scaled by
  DAG demand.
- `provider_pool.<name>.budget` records cloud/provider spend limits, token caps, and halt behavior.

Default pool for Pipeon:

- `ollama`: `min_ready=1`, `max_active=1`, lifecycle tied to the existing Ollama compose service.
- `claude`: `min_ready=1`, `max_active=1`, guarded container worker, session-affine for direct chat.
- `codex`: `min_ready=1` only if a persistent Codex protocol/MCP lane is viable; otherwise direct chat
  keeps using host `codex exec resume` with session binding and no always-on container.
- Defaults must be configurable from package-owned config/environment. Expected knobs:
  `PIPEON_PROVIDER_POOL_CLAUDE`, `PIPEON_PROVIDER_POOL_CODEX`, `PIPEON_PROVIDER_POOL_OLLAMA` or an
  equivalent YAML/catalog-backed setting.

DorkPipe defaults should be independent of Pipeon and reusable from the CLI. Pipeon may override them
for its bundled stack profile, but the source of truth should be package-owned DorkPipe provider
catalog/config, not VS Code extension state.

Lifecycle:

- Pool lifecycle matches long-lived Pipeon stack services such as the project database/control
  services: start with the stack, stay warm while Pipeon is open, drain/stop with stack teardown.
- Workers should not be tied to one prompt. A worker can process many direct chat turns until it is
  idle, unhealthy, over budget, explicitly reset, or the stack closes.
- Direct chat workers are sticky to Pipeon chat sessions when context continuity matters.
- Workflow fan-out workers are leased from the pool by role/capability and released when the workflow
  node completes. They may use a different `max_active` than direct chat.
- If `max_active` is reached, new work queues with visible UI state instead of silently spawning more
  cloud workers.

Operational behavior:

- Startup warms only the configured minimum. It should not pre-spawn expensive cloud workers unless
  the user selected/enabled that provider.
- Direct chat first uses a session-affine warm worker. If none is ready, it can either queue briefly
  while one starts or fall back to a clear “warming provider” status.
- Agentic workflows request workers from the pool by provider/role/capability, but still execute under
  YAML policy with artifacts, approvals, and validation.
- Pool workers must be observable: ready/busy/failed/auth-required, current session binding, elapsed
  lifetime, prompt count, last error, and estimated spend.
- Pool teardown is tied to stack teardown. No orphan provider workers should outlive Pipeon unless an
  explicit detached mode exists.

Dispatch rules:

- Pipeon provider dropdown targets direct provider pools, not workflow fan-out by default.
- CLI provider selection targets the same direct provider pools by default.
- A top-level orchestrator prompt should be low-latency and use a ready direct worker.
- The direct worker can recommend or request escalation to an agentic workflow when the task needs
  parallelism, isolated edits, validation, artifacts, or higher spend.
- DorkPipe workflow YAML remains the durable contract for fan-out/subagents. Provider-native `/loop`
  or `/fanout` features can be used inside a bounded worker implementation, but they should not become
  the public workflow contract.

Claude-specific note:

- A current Claude worker transcript showed the CLI correctly denying any implicit host/MCP contract:
  the bind-mounted `/work` repo is file access, not host execution. The warm Claude lane must therefore
  be launched with an explicit DorkPipe/Pipeon contract and routed through a guarded bridge. It should
  not rely on the Claude Code session independently discovering `packages/dorkpipe/mcp`.

Non-goals:

- Do not move Claude-specific warm-worker behavior into `src/lib/` or `src/cmd/`.
- Do not replace workflow YAML with extension-local routing state.
- Do not keep hidden cloud workers alive when the stack is closed; warm lanes live and die with the
  Pipeon/DorkPipe stack.

## CLI Orchestrator Workflow

Add a simpler CLI entrypoint for core development and non-Pipeon users. It should exercise the same
DorkPipe provider-pool contract as Pipeon direct chat, so the CLI path becomes the reference
implementation rather than a second routing system.

Desired workflow shape:

- Package-owned workflow, likely under the DorkPipe package, for a direct orchestrator prompt.
- Inputs include prompt text, workspace path/session identity when needed, provider override, model
  override, budget/approval mode, and optional workflow-escalation policy.
- Default provider resolves from DorkPipe provider-pool config. Inline CLI override can select
  `ollama`, `codex`, `claude`, or future providers without editing the workflow file.
- Direct execution uses a warm provider worker when available. If the pool is not ready, the CLI shows
  `warming`, `auth-required`, `queued`, or `provider-disabled` state instead of silently falling back
  to an expensive cold path.
- A direct CLI worker can request escalation into YAML-backed agentic workflows, but the escalation
  remains explicit and artifact-backed.

Example target UX:

```bash
dockpipe --package dorkpipe --workflow orchestrator -- --prompt "summarize this repo"
dockpipe --package dorkpipe --workflow orchestrator -- --provider claude --prompt "review the package boundary"
dockpipe --package dorkpipe --workflow orchestrator -- --provider ollama --model llama3.2 --prompt "quick local answer"
```

Implementation notes:

- Keep provider pool config and provider availability in DorkPipe package-owned catalogs/config.
- Keep Pipeon provider dropdown labels and CLI provider names backed by the same catalog.
- Use the CLI orchestrator workflow for local/core development smoke tests before validating the
  Pipeon UX.
- If the CLI workflow changes authored YAML semantics, update schema/docs/language-support together.

## Prebuilt Image Strategy

The biggest release-time win is prebuilding common Pipeon images instead of building them on the
developer machine during first launch.

Candidate images:

- `dockpipe-code-server:<version>` with Pipeon and DockPipe language-support VSIX files already
  installed.
- `dockpipe-dorkpipe-stack:<version>-linux-amd64` with Linux `dockpipe`, `dorkpipe`, and `mcpd`
  binaries already present.
- Optional GPU-aware stack variants only if the runtime contract really differs. Prefer one image
  with compose/runtime GPU toggles when possible.
- Optional base/runtime variants for common host configs only after measuring actual demand.

Release flow:

- Build images from exact package inputs and versioned tool binaries.
- Tag by DockPipe version and content digest.
- Publish image metadata with expected package/version/signature.
- Let `pipeon-dev-stack` prefer matching prebuilt images when available.
- Fall back to local source builds when offline, unpublished, or running dirty development inputs.
- Keep local build mode available for open-source/offline users and maintainers.

This likely needs its own small release/helper application or package command that can generate and
publish the image matrix as new versions come out. Treat it like Docker layer caching plus package
release automation, not as ad hoc launch-script logic.

## Measurement Plan

Capture coarse timing around:

- code-server image signature check
- Pipeon VSIX packaging
- code-server image build or reuse
- Linux tool build or reuse
- DorkPipe stack image build or reuse
- Docker compose up
- MCP readiness
- host MCP bridge readiness
- Ollama readiness
- model pull or cached model check
- code-server container readiness
- desktop shell open

The launch script should emit enough timing to explain slow starts without forcing users to inspect
Docker logs manually.

## Open Questions

- Which image variants are worth publishing for the first release: CPU-only, NVIDIA GPU, or one
  runtime-configurable image?
- Should image selection be automatic by version/signature or explicit by `PIPEON_DEV_STACK_IMAGE_*`
  overrides?
- Where should image metadata live: package catalog, release manifest, or generated package state?
- How should offline installs seed images: tarball import, local registry, or documented `docker pull`
  cache warming?
- What is the acceptable first-launch target on Windows with Docker Desktop already running?
- Should the warm Claude lane be a long-running container service with an HTTP/MCP shim, or a host
  bridge-managed daemon that still uses the resolver container as its isolation boundary?
- Can Codex eventually use a persistent protocol/MCP mode for lower latency, or is `codex exec
  resume` the right direct lane until the CLI exposes a stable daemon interface?

## Current Status

In progress. Launch now skips cached Ollama model pulls and hides non-interactive Windows PowerShell
startup calls. Prebuilt image generation is documented here as the larger release-engineering item.

First vertical slice for shared DorkPipe provider pools landed:

- DorkPipe now owns a package-level provider-pool catalog/config for `ollama`, `claude`, and `codex`.
- DorkPipe exposes shared provider-pool catalog/status/chat operations through its CLI and host MCP
  bridge so Pipeon and non-Pipeon surfaces use the same contract.
- A package-owned DorkPipe `orchestrator` workflow now routes direct prompts through the provider pool
  contract and supports inline provider/model overrides such as `--provider claude` and
  `--model llama3.2`.
- Direct CLI orchestration now returns explicit `warming`, `auth-required`, `queued`, `disabled`, or
  `failed` states instead of silently cold-starting a provider lane.
- Pipeon provider selection and direct chat now consume the shared DorkPipe provider-pool catalog and
  MCP chat tool rather than owning a separate routing model.
- Pipeon dev-stack startup now explicitly calls the shared DorkPipe provider-pool warm lifecycle and
  writes a provider-pool status snapshot into stack state, so warm-up is deliberate and observable
  instead of being triggered implicitly by a read-only status/catalog call.

Current lane status on a normal core-dev machine is intentionally honest:

- Codex direct chat preserves session bindings and can use the host `codex exec` resume lane.
- Ollama reports `warming` until the local service is reachable instead of triggering an implicit cold
  path.
- Claude guarded warm-worker health is now materially better:
  - explicit `op inject` is no longer forced just because project config selects 1Password; workflows
    without referenced secret-template keys now skip vault injection cleanly
  - direct prompts wait briefly for a warming provider instead of immediately failing back to “retry”
  - the guarded worker now reuses the same pool identity across direct CLI, workflow host steps, and
    future app surfaces because relative and absolute workdirs canonicalize to the same repo-root key
  - the guarded worker bootstrap now avoids copying heavyweight host Claude state such as
    `file-history`, while still copying the smaller auth/session files needed for a viable warm lane
  - the keepalive no longer depends on `runuser`, which was missing from the current `dockpipe-claude`
    image and caused worker exits
- Claude remains the active tuning area rather than a correctness blocker:
  - shared-pool direct prompt latency is currently about 31 seconds on the measured Windows core-dev
    machine once the worker is warm
  - the full `dockpipe --package dorkpipe --workflow orchestrator` path is currently about 41 seconds
    against that warm worker
  - the remaining gap appears to be inside the guarded Claude container lane itself, not provider-pool
    identity drift, hidden cold starts, or workflow dispatch overhead
