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

### Provider Stream Worker Contract

The next provider-pool milestone is a generic stream-worker contract. The public contract must stay
provider-neutral and DorkPipe-owned; Claude is the first implementation because its CLI exposes a
working stream JSON mode, not because the pool API should become Claude-specific.

Terminology:

- **Pool provider**: catalog entry such as `claude`, `codex`, or `ollama`.
- **Pool session**: DorkPipe-owned session identity selected by CLI/Pipeon/MCP. It carries provider,
  workdir, role, selected model, budget policy, approval mode, and optional UI chat identity.
- **Worker instance**: runtime-owned execution boundary for a provider session. It can be a guarded
  container, host process, local service, or future remote worker depending on catalog/runtime policy.
- **Stream process**: optional long-lived provider process inside a worker instance. It receives
  prompts on stdin or a local socket/protocol and emits structured events until reset or terminated.
- **Prompt turn**: one request/response exchange within a pool session. Turns must be metered,
  observable, cancelable, and attributable even when they share a stream process.

Generic API shape:

- `provider-pool catalog`: returns provider capabilities including whether the provider supports
  `stream_worker`, `single_prompt`, `host_resume`, `service`, or another execution mode.
- `provider-pool status`: reports provider state plus worker/session details:
  `ready`, `warming`, `busy`, `queued`, `failed`, `auth-required`, `disabled`, selected mode,
  worker id, session id, prompt count, last turn timing, last error, and spend/budget state.
- `provider-pool warm`: materializes the minimum ready worker count for enabled providers. For
  stream-capable providers, warming starts both the worker boundary and the stream process when
  policy permits.
- `provider-pool prompt`: remains the stable direct prompt API for CLI, Pipeon, and MCP. It chooses
  a stream worker when available, falls back only according to explicit catalog policy, and returns
  the same response envelope as today with richer metadata.
- `provider-pool reset`: future operation to reset a pool session or worker when context,
  authentication, budget, or health requires it. Reset must be explicit and observable.

Existing MCP leverage:

- The stack already has the right MCP front door. `packages/dorkpipe/mcp` exposes
  `dorkpipe.provider_pool_catalog`, `dorkpipe.provider_pool_status`, and
  `dorkpipe.provider_pool_chat`; Pipeon already calls the host MCP bridge tool
  `dorkpipe.provider_pool_chat` for direct provider chat.
- The first stream-worker implementation should preserve those MCP tool names and schemas where
  possible. The `dorkpipe.provider_pool_chat` handler should keep invoking the generic
  `dorkpipe provider-pool prompt --json` contract, and the provider-pool implementation should choose
  the fast stream worker behind that CLI contract.
- New MCP tools are only needed for new lifecycle operations that do not exist today, such as
  `dorkpipe.provider_pool_warm`, `dorkpipe.provider_pool_reset`, or future streaming/event-subscribe
  support. Do not create a Claude-only MCP tool for this fast path.
- If richer streaming to the UI is needed, prefer a generic provider-pool event stream contract
  keyed by provider/session/turn id. Keep the current JSON response path as the compatibility
  baseline for CLI workflows and simple MCP clients.
- The Pipeon stack MCP proxy should remain the externally exposed control-plane surface. The
  upstream DorkPipe MCP service and any provider stream worker internals stay private to the stack or
  host bridge boundary according to existing MCP tier/auth policy.

Required response metadata for direct prompt turns:

- `provider_preset`, `selected_model`, `session_id`, `worker_id`, `worker_mode`
- timing fields:
  `queue_wait_ms`, `status_ms`, `worker_start_ms`, `stream_start_ms`, `stream_ready_ms`,
  `time_to_request_ms`, `time_to_first_token_ms`, `provider_api_ms`, `provider_turn_ms`,
  `total_ms`
- stream fields when available:
  `provider_session_id`, `provider_request_id`, `prompt_turn_id`, `prompt_count`,
  `stream_reused`, `stream_restart_reason`
- budget fields when available:
  `estimated_input_tokens`, `estimated_output_tokens`, `estimated_cost_usd`, `budget_remaining`,
  `budget_halt_reason`

Worker lifecycle rules:

- Session affinity is honored first. A Pipeon chat session or CLI `--session-id` should reuse the
  same stream worker while healthy and under budget.
- A stream worker may process many turns, but never more than one active turn unless the provider
  protocol explicitly supports multiplexing and the catalog declares it safe.
- `max_active` caps active workers or active turns according to the provider mode. Queueing must be
  visible instead of silently spawning more cloud/provider work.
- Idle workers drain after `idle_ttl_seconds` unless pinned by an active Pipeon session or explicit
  stack lifecycle policy.
- Failed stream processes restart inside the existing worker boundary when possible. Repeated
  failures mark the provider `failed` with the last error and restart count.
- Stack teardown owns provider-pool teardown. No hidden Claude/Codex/Ollama workers should outlive
  Pipeon unless the catalog exposes an explicit detached mode.

Boundary and ownership rules:

- Keep provider-specific protocol adapters in DorkPipe package-owned assets/code/catalogs, not
  DockPipe engine code. Core DockPipe remains spawn/run/act.
- The generic provider-pool contract is provider/session/worker oriented. It must not expose fields
  such as `claude_session_id` as public API; provider-native IDs belong under provider metadata.
- MCP is a front door and session router, not the only implementation. CLI workflows, Pipeon, and
  future app surfaces all call the same DorkPipe provider-pool operations.
- Pipeon must not own pool state in the VS Code extension. It should store UI chat identity and call
  the DorkPipe MCP/CLI provider-pool contract for catalog, status, prompt, warm, and reset.
- Provider workers must receive an explicit DorkPipe contract: workspace path, allowed tools, budget,
  escalation rules, approval mode, and optional MCP bridge endpoint. Do not rely on a provider CLI
  discovering host capabilities implicitly from a bind mount.

First implementation target: Claude stream worker.

- Preserve the guarded container boundary and copied/allowlisted subscription auth state already used
  by the Claude pool.
- Replace the sleeping warm container plus per-prompt `docker exec claude -p` path with a
  session-affine in-container worker manager that starts:

```bash
claude --dangerously-skip-permissions --model <model> -p \
  --input-format stream-json \
  --output-format stream-json \
  --include-partial-messages \
  --replay-user-messages \
  --verbose
```

- Send each prompt turn as one JSONL user message to the stream process and read structured events
  until a `result` event closes the turn.
- Extract response text from the `result.result` field or final assistant text events, preserving raw
  JSONL diagnostics in debug metadata/artifacts when requested.
- Reuse the same stream process for follow-up turns while session affinity, model, workdir, auth,
  budget, and approval mode are unchanged.
- If model/workdir/auth/policy changes, start a new stream process and report
  `stream_restart_reason`.
- Keep the existing single-prompt `docker exec claude -p` path as a fallback behind an explicit
  catalog mode or failure policy until the stream worker is stable.

Pipeon consumption rules:

- Pipeon provider selection continues to read the shared provider-pool catalog.
- Pipeon direct chat sends messages through the DorkPipe MCP `provider_pool.prompt` operation with a
  stable session id for the chat tab/conversation.
- Pipeon displays provider status from `provider-pool status`, including warming/queued/failed states
  and last-turn timing. It should not infer readiness from extension-local state.
- Pipeon stack startup may call `provider-pool warm`, but it should not eagerly start costly stream
  workers unless the provider is enabled by catalog/environment policy.
- Pipeon should benefit automatically from stream workers because the MCP bridge and CLI orchestrator
  call the same provider-pool prompt contract.

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

Latest Claude provider-pool tuning pass:

- The warm guarded worker was already reusing the expected container
  `dorkpipe-provider-pool-claude-4ca0fbabc6` and image `dockpipe-claude:latest`; direct Docker
  inspection showed the worker was up before prompt measurements.
- Cheap container probes were not the bottleneck on the Windows core-dev machine:
  - `docker exec ... true`: about 0.6 seconds
  - `docker exec ... node -e`: about 0.6 seconds
  - `docker exec ... claude --version`: about 1.0 seconds
- The prompt path was still using `docker exec -i` even though Claude is invoked with `-p` and does
  not need stdin. Removing `-i` avoids keeping an interactive stdin pipe open for every pooled
  prompt. Raw warm-worker probes went from about 37.6 seconds with `-i` to about 21.6-25.2 seconds
  without `-i` for the same "latency probe" prompt.
- Provider-pool prompt JSON now emits coarse timing metadata so CLI, workflow, and Pipeon callers can
  see `auth_check_ms`, `image_check_ms`, `image_recheck_ms`, `container_running_check_ms`,
  `worker_start_ms` when a worker is started, `status_ms`, `readiness_wait_ms`, `queue_wait_ms`,
  `provider_prompt_ms`, `claude_command_ms`, and `total_ms`.
- Representative post-change measurements:
  - direct `dorkpipe provider-pool prompt --workdir . --provider claude --json`: 23.7 seconds total
    in the best warmed sample, with `claude_command_ms=22037`, `status_ms=1180`, and
    `queue_wait_ms=3`
  - direct final sample after adding readiness timing: 44.0 seconds total, with
    `claude_command_ms=42272`, `status_ms=1245`, and `queue_wait_ms=2`
  - CLI orchestrator workflow warmed sample:
    `dockpipe --package dorkpipe --workflow orchestrator -- --provider claude --json`: 27.0 seconds
    end-to-end, with `claude_command_ms=23235`, `provider_prompt_ms=23362`, `status_ms=505`, and
    `queue_wait_ms=1`
- Current conclusion: the avoidable Docker stdin overhead is reduced, and the remaining variance is
  overwhelmingly inside `claude --dangerously-skip-permissions --model sonnet -p ...` within the
  guarded container. Pool identity, queueing, readiness, image checks, and workflow dispatch are no
  longer the dominant warm-path cost. Pipeon should benefit automatically because it calls the same
  shared DorkPipe provider-pool contract and receives the same timing metadata through the MCP bridge.

Follow-up stream-worker experiment:

- `claude --bare` is not viable for the current guarded lane because it rejects the copied
  subscription/OAuth-style host auth and reports `Not logged in`; the help text says bare mode uses
  API-key/helper auth only.
- `claude --safe-mode` preserved current auth but did not materially improve prompt latency; a simple
  latency probe took about 31 seconds.
- Claude's machine stream mode is viable for a real warm worker:
  `claude --dangerously-skip-permissions --model sonnet -p --input-format stream-json
  --output-format stream-json --include-partial-messages --replay-user-messages --verbose` accepted
  multiple JSONL user messages on one process.
- In the two-turn proof, the first turn paid initialization and returned `first-turn` with
  `time_to_request_ms=22293`, `ttft_ms=28550`, and `duration_ms=28571`; the second turn in the same
  Claude process returned `second-turn` with `time_to_request_ms=44`, `ttft_ms=2657`, and
  `duration_ms=2731`.
- Next implementation direction: keep the existing DorkPipe provider-pool/MCP contract as the public
  surface, but replace the sleeping guarded Claude container plus per-prompt `docker exec claude -p`
  with a session-affine in-container Claude stream process managed by DorkPipe. MCP can keep the
  provider session addressable and route prompts, but the latency win comes from keeping the Claude
  stream process alive behind that contract.

Stream-worker implementation pass:

- The generic provider-pool public path remains unchanged: CLI workflows, MCP, and Pipeon still call
  `dorkpipe provider-pool prompt --json` through `dorkpipe.provider_pool_chat`. No Claude-specific
  public MCP tool or DockPipe core logic was added.
- Claude direct prompts now default to a session/model-affine stream worker inside the guarded
  container. The worker is addressed by generic provider-pool fields (`provider`, `session_id`,
  `worker_id`, `worker_mode`, `prompt_turn_id`) and launches:
  `claude --dangerously-skip-permissions --model <model> -p --input-format stream-json
  --output-format stream-json --include-partial-messages --replay-user-messages --verbose`.
- The stream worker is managed by DorkPipe inside the existing warm container using a
  container-local Unix socket. Each prompt is sent as one JSONL user turn, and DorkPipe reads stream
  events until Claude emits `type=result`.
- The previous one-shot `docker exec claude -p` path remains as the explicit fallback. Set
  `DORKPIPE_PROVIDER_POOL_CLAUDE_STREAM_WORKER=single_prompt` to force it, or
  `DORKPIPE_PROVIDER_POOL_CLAUDE_SINGLE_PROMPT_FALLBACK=1` to fall back after a stream-worker error.
- Prompt JSON now includes the requested stream timing fields where available:
  `queue_wait_ms`, `status_ms`, `worker_start_ms` when a worker container starts,
  `stream_start_ms`, `stream_ready_ms`, `time_to_request_ms`, `time_to_first_token_ms`,
  `provider_turn_ms`, and `total_ms`. It also includes `provider_session_id`,
  `provider_request_id`, `prompt_turn_id`, `prompt_count`, `stream_reused`, and
  `stream_restart_reason`.
- Direct validation on the Windows core-dev machine:
  - first streamed direct prompt after daemon restart returned `stream-smoke` with
    `stream_reused=false`, `stream_start_ms=3355`, `stream_ready_ms=484`,
    `time_to_request_ms=1`, `time_to_first_token_ms=32043`, `provider_turn_ms=32283`, and
    `total_ms=36856`
  - second direct prompt on the same provider-pool session returned `stream-smoke-2` with
    `stream_reused=true`, `time_to_request_ms=1`, `time_to_first_token_ms=3266`,
    `provider_turn_ms=3302`, and `total_ms=4784`
  - `dockpipe --package dorkpipe --workflow orchestrator -- --provider claude --json` reused the
    same stream worker and returned `orchestrator-stream-smoke` with `stream_reused=true`,
    `provider_turn_ms=2885`, `provider_prompt_ms=3516`, and provider-pool `total_ms=4082`
- Pipeon should pick up the fast path automatically: the extension call sites still read
  `dorkpipe.provider_pool_catalog` and send direct chat through `dorkpipe.provider_pool_chat`, which
  invokes the same `provider-pool prompt --json` implementation.
