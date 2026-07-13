# Codex App Server for Top-Level Orchestrators

Research date: 2026-07-10
Status: proposed direction; no production integration in this change

## Executive summary

**Recommendation: prototype before deciding.** Codex App Server is a viable first-class, provider-specific adapter candidate for long-running top-level Codex orchestrators. It has structured threads, turns, items, approvals, interruption and status that the current Pipeon/DorkPipe host path reconstructs from codex exec output and transcript files.

Keep codex exec as the default for bounded DorkPipe worker reps: they are disposable, scoped tasks that produce artifacts then terminate. Prototype App Server only for the host-resident master/orchestrator use case. Pipeon is the first migration candidate because it needs durable direct-chat sessions but currently has no typed approval or lifecycle stream.

The spike should supervise one App Server stdio child per top-level session. It must retain Codex native sandboxing, use the user as approval reviewer, relay every request through DockPipe, persist an audit record and fail closed on disconnect. Do not use WebSocket in production: official documentation calls it experimental/unsupported. Local codex-cli 0.143.0 also labels app-server experimental.

## Sources and evidence

Primary sources inspected on 2026-07-10:

| Source | Evidence |
| --- | --- |
| [Official App Server README](https://github.com/openai/codex/blob/main/codex-rs/app-server/README.md) | Transport, initialization, v2 methods, lifecycle, approvals, MCP, auth and schemas. |
| [v2 protocol source](https://github.com/openai/codex/blob/main/codex-rs/app-server-protocol/src/protocol/v2.rs) | Wire types for sandbox, approval, permission, error and configuration. |
| [Protocol registration](https://github.com/openai/codex/blob/main/codex-rs/app-server-protocol/src/protocol/common.rs) | Method registration and experimental gates. |
| [App Server source tree](https://github.com/openai/codex/tree/main/codex-rs/app-server) | Server wiring where the README is incomplete. |
| [Codex repository guidance](https://github.com/openai/codex/blob/main/AGENTS.md) | Source-derived: active API work is v2 and schemas/fixtures regenerate; not a public compatibility promise. |
| [Codex license](https://github.com/openai/codex/blob/main/LICENSE) | Apache-2.0 source license; binary/service terms still need review. |

Documented means stated by the README. Source-derived is based on official source layout/guidance. Prototype required means not yet proven locally.

Read-only local checks found codex-cli 0.143.0. Help confirms stdio, Unix socket, WebSocket, daemon/proxy, per-invocation configuration and WebSocket auth options. A local schema-generation attempt could not create its requested C:\tmp output directory in the current sandbox, so it is not used as evidence.

## Official capability findings

### Transport and initialization

- **Documented:** App Server is bidirectional JSON-RPC 2.0; stdio is default and sends JSONL. It is the only first-release DockPipe transport.
- **Documented:** Unix socket uses WebSocket framing below CODEX_HOME and proxy can bridge it to stdio. It is out of initial scope.
- **Documented:** WebSocket provides ready/health probes but is experimental/unsupported. Do not expose it in Pipeon/PipeDesk.
- **Documented:** bounded queues return JSON-RPC -32001 overload; clients should retry with jittered backoff.
- **Documented:** each connection sends initialize then initialized; response includes codexHome/platform, and pre-init/repeated init is rejected.
- **Documented:** clientInfo identifies an integration for OpenAI compliance logs. DockPipe needs a stable truthful identity/version in audit records.
- **Source-derived:** stable v2 types live under app-server-protocol/src/protocol/common.rs and v2.rs. Generate/pin stable schema from the selected binary; do not hand-copy provider types.

### Configuration, sandbox and auth

- **Documented:** configuration is global/effective Codex config, thread start/resume override, or turn start override. Turn overrides become later-turn defaults except output schema; environments can be per turn.
- **Design:** do not write user config.toml for normal orchestration. Provide explicit thread/turn workspace-write and approval settings, record policy/config warnings and avoid config-write APIs.
- **Documented:** approval and sandbox policy plus experimental permission profiles can be thread/turn scoped. User is default approval reviewer; auto-review delegates to a subagent.
- **Security requirement:** explicitly choose user; never auto-review. Never call thread shell-command because official docs say it is unsandboxed and ignores thread policy.
- **Documented:** account read/notifications show API key, ChatGPT, PAT and supported managed auth; ChatGPT managed auth persists/refreshes in Codex.
- **Prototype required:** prove current user ChatGPT auth is reused without DockPipe reading/copying auth material.
- **Documented:** skills are listable/reloadable/watched; normal CODEX_HOME launch uses existing config, skills and MCP settings. Do not isolate CODEX_HOME without explicit complete profile provisioning.

### Thread, turn, events and approvals

- **Documented:** thread start/resume/fork/read/list/loaded-list create and recover persisted conversations. Thread state includes notLoaded, idle, active and systemError.
- **Documented:** turn start accepts typed text/image input; turn steer accepts follow-up input for a steerable active turn.
- **Documented:** turn started, item deltas/tool/file events, warnings, token usage and turn completed give structured progress. Terminal state is completed, interrupted or failed; error may precede failed completion.
- **Documented:** turn interrupt requests cancellation; only terminal interrupted confirms it. Background terminal cleanup is experimental and excluded.
- **Documented:** command/file approvals are server-to-client JSON-RPC requests carrying thread, turn, item, reason and action context. Server request resolved plus terminal item state establish outcome. Permission response grants only a subset; omitted permission is denied.
- **Design:** lost stream/process, invalid message or missing terminal state is Disconnected, never completed/failed until recovery proves it.
- **Documented:** MCP startup/tool/resource/OAuth/config reload are exposed; startup is starting, ready, failed or cancelled. Record tool/server identity but keep DockPipe MCP capability-scoped/audited.
- **Documented:** LOG_FORMAT=json gives JSON tracing on stderr. Redacted RPC plus trace can improve audit evidence.

### Stability, licensing and CLI gaps

- **Documented:** generated TypeScript/JSON schema is binary-version-specific. Stable output excludes experimental surface; experimental API has no compatibility guarantee.
- **Conclusion:** target stable v2 only; version capability matrix and schema gate are required; retain CLI fallback.
- **Documented/local:** WebSocket is unsupported and local CLI marks App Server experimental. No semver/backward-compatibility claim is safe.
- **Documented:** source is Apache-2.0. Invoke user-installed Codex, not a bundled fork, pending service/distribution review.
- **Gap:** App Server is not a terminal/TUI replacement. Pipeon can render protocol controls but must not expose unsafe shell shortcuts.

## DockPipe current-state trace

### Bounded workers: working and retained

packages/agent/resolvers/codex and DorkPipe worker scripts execute bounded resolver/container work. packages/dorkpipe/lib/workers/workers.go uses exec.CommandContext and captures output. The DorkPipe orchestration contract stores provider sessions as trace-only metadata and owns artifacts, budget/halt, merge/verify and later approval. This model remains default.

### Top-level sessions

| Component | Current behavior | Works today | App Server gain |
| --- | --- | --- | --- |
| Pipeon extension | Calls host MCP direct chat with Pipeon session id. | Thin UI, provider selection and safe status. | Live typed turn/approval display. |
| DorkPipe MCP | Exposes provider-pool chat and host-Codex chat. | Control-plane auth/tiering and path checks. | Generic adapter without JSON-RPC in UI. |
| provider_pool.go | Runs codex exec with workspace-write then exec resume. | Session affinity, readiness/auth/queue state and leases. | Typed thread ID/state instead of transcript discovery. |
| mcpbridge/exec.go | Runs 30-minute command, output-last-message, scans user Codex JSONL, persists Pipeon-to-Codex binding. | Existing auth/config model selection and native new-session sandbox. | Removes timestamp/file race; adds native approval/interrupt. |
| structured trace/Pipeon contract | Projects safe progress/artifacts. | UI hides raw command/patch; model text is not execution authority. | Same contract receives richer events. |

The host bridge stores bindings in bin/.dockpipe/packages/dorkpipe/host-bridge/codex-sessions.json. Provider-pool has package state. latestCodexSessionID scans user session JSONL by workdir/time: concurrent runs can race and it cannot represent active work/pending approval.

| Question | Current handling | Gap |
| --- | --- | --- |
| Completed/failed | Exit code, output-last-message, string heuristics. | No typed terminal/error taxonomy. |
| Approval | Noninteractive metadata says exec_noninteractive; RequiresApproval false. | Native approval is not relayed. |
| Architecture/user question | Ordinary prose. | No typed waiting-user-input state. |
| Unresponsive | 30-minute context. | No progress deadline/reconnect state. |
| Cancel | Context/process cancellation. | No verified graceful interrupt. |
| Sandbox | Explicit workspace-write on new exec; resume inherits prior session. | No normalized resumed-policy proof. |

This current implementation is not wrong: it is simple, preserves native sandbox on new sessions and prevents Pipeon direct host execution. App Server addresses lifecycle facts that process parsing cannot safely express.

## Direct comparison

| Area | Current CLI | App Server | DockPipe impact | Status |
| --- | --- | --- | --- | --- |
| Process startup | One exec per prompt/resume. | Supervised process can own many threads/turns. | Less churn. | Likely; prototype. |
| Authentication | Existing CLI/auth discovery. | Effective account/config plus account read. | No credential copy. | Confirmed API; reuse prototype. |
| Session creation | Spawn then scrape transcript ID. | Thread start returns ID. | Remove heuristic. | Confirmed. |
| Session resumption | Exec resume. | Thread resume/read/loaded list. | Explicit recovery. | Confirmed. |
| Streaming | Buffered output. | Typed item/tool/file/plan/token events. | Safe live UI/audit. | Confirmed. |
| Structured state | Exit/string interpretation. | Thread/turn state and typed error. | Accurate state machine. | Confirmed. |
| Approvals | Not relayed. | Server request/response/resolution. | Native approve/deny audit. | Confirmed; policy spike. |
| Cancellation | Kill/cancel child. | Turn interrupt then terminal event. | Graceful cancel. | Confirmed. |
| Failure detection | Exit/text. | Failed/error; supervisor still needed. | Better classification. | Confirmed. |
| MCP | Separate DockPipe MCP only. | MCP status/tool/resource events. | Event correlation. | Confirmed. |
| Sandbox | New exec explicit; resume inferred. | Thread/turn policy override. | Persist/verify policy. | Confirmed API; Windows spike. |
| Auditability | Output plus binding. | IDs/events/JSON log. | Stronger ledger. | Confirmed; persistence needed. |
| Protocol stability | CLI output/flags. | Schema and stable/experimental split. | Version gate/tests. | Likely, not guaranteed. |
| Testing/maintenance | Low volume, brittle scrape. | Adapter/schema/process fixtures. | More initial work, less ambiguity. | Likely. |

## Proposed design

Architecture:

Pipeon, PipeDesk or CLI sends only provider-neutral session operations into DockPipe. The generic contract owns provider/session ref, workspace/policy envelope, start/resume/follow-up/cancel/decision, normalized states/events and opaque correlation. A Codex-specific DorkPipe adapter owns JSON-RPC, schemas, thread/turn IDs, raw items and approval unions. The adapter runs a supervised host App Server stdio child, retains native sandbox/escalation, and projects approval/audit into DockPipe. Pipeon never parses provider protocol.

Normalized lifecycle:

Starting leads to Ready, then Running. Running can wait for approval or user input, and ends Completed, Cancelled, Failed or Disconnected. Waiting states return to Running or terminal. Disconnected becomes Ready only after verified recovery, otherwise Failed. Ready means initialized plus known idle thread; Cancelled requires interrupted terminal notification or auditable process termination; Disconnected is fail-closed.

Supervisor responsibilities:

1. Resolve absolute Codex binary/version and reject unsupported schema.
2. Spawn stdio child with normal user CODEX_HOME, least environment, workspace policy and no hidden full-access override; own process/job/streams.
3. Initialize with DockPipe identity, no experimental opt-in by default; validate returned codexHome.
4. Start/resume with workspace-write and user reviewer; record policy fingerprint and warnings.
5. Normalize RPC/events/parser failures to ordered journal. Restrict/redact raw provider log; Pipeon receives safe events only.
6. Persist session/workspace/binary/process-incarnation/thread/turn/state/event-cursor/pending-approval/outcome in DorkPipe package state. Never persist tokens.
7. Cancel through turn interrupt, wait terminal, kill only after deadline and record path.
8. On exit/transport loss mark active work Disconnected, deny pending approval, then permit user-guided reconcile with fresh process plus thread read/resume. Never replay an active prompt to CLI fallback.

Approval relay:

1. Persist request before UI render with process incarnation, connection, thread, turn, item, RPC request id, event hash, normalized intent/scope and safe preview.
2. Pipeon renders DockPipe record only; it cannot call App Server directly.
3. User choice must exactly match the active tuple, is single-use and has send/result audit.
4. Do not resolve until server request resolved and terminal item event. Timeout, disconnect, restart, duplicate/mismatch means decline/cancel.
5. Prototype supports one-shot accept/decline/cancel only. Session-wide/amendment approval remains disabled.

Migration:

Keep codex_exec named legacy adapter. Feature-gate codex_app_server for one Pipeon session/top-level run. Fallback is allowed only before a turn or after explicit Disconnected; never replay a mutation prompt. Pipeon session maps to verified App Server thread rather than scraped transcript. The same normalized contract serves PipeDesk/CLI later.

## Focused security review

| Risk | Mitigation |
| --- | --- |
| Sandbox disabled/broader host access | Reject full access/thread shell-command; validate policy fingerprint every turn. |
| Config mismatch | Record binary version, codexHome, policy, roots, warnings/layers; recheck on resume. |
| Approval spoof/replay/cross-session | Atomic one-time tuple plus nonce/hash; strict thread registry; deny mismatch. |
| Unauthorized transport | Private stdio only; no listener. Later socket needs OS permission/auth proxy. |
| Token/auth exposure | Inherit CODEX_HOME without reading/copying auth; redact account/RPC fields and restrict logs. |
| Malformed protocol | Size limits, JSON-RPC/schema validation, defensive parser, quarantine/disconnect. |
| Process compromise | Absolute binary/version, least environment, child ownership; treat as privileged local dependency. |
| Untrusted MCP | Inventory/allow-list server, audit tool identity, approval-gate elicitation. |
| Connection loss while Codex runs | Disconnected, block approvals, warn user, reconcile before resume/retry. |
| Dropped/reordered events | Append-only sequence/hash journal, dedupe, thread-read reconciliation and gap alarm. |
| Auto-review bypass | Explicit user reviewer, forbid auto-review, test denial remains denied. |

## Prototype recommendation

A narrow protocol spike is required before migration.

Acceptance criteria:

- launch existing user App Server via stdio without credential copying;
- initialize and record binary version, codexHome and client identity;
- create/read/resume thread; start turn; receive typed item/turn stream;
- provoke native approval and prove exact-correlation acceptance and denial; prove denial remains denied;
- interrupt running turn and observe terminal interrupted;
- test clean shutdown/forced child death and mark active work Disconnected;
- prove workspace-write sandbox and absence of full-access/shell shortcut;
- pin tested stable schema and separately record unsupported/experimental features.

Non-goals: production abstraction; engine changes; default switch; WebSocket/remote control; background terminals; auto-review; session-wide approvals; config writes; bounded-worker replacement; automatic replay after disconnect.

## Future repository impact map

No production area was changed by this research.

| Area | Likely future change |
| --- | --- |
| packages/dorkpipe/lib | Provider-neutral top-level contracts, state/events/persistence and dedicated adapter package. |
| packages/dorkpipe/mcp/mcpbridge | Normalized host session/approval operations while retaining tier/path control. |
| provider-pool and catalog | Adapter selection/capability policy while retaining exec. |
| Pipeon extension | Normalized lifecycle/approval/recovery UI; never raw protocol. |
| DorkPipe/Pipeon tests/docs | Contract fixtures, controlled integration tests and operations guidance. |

## ADR status and final recommendation

Repository search found no ADR directory, naming convention or index. Do not introduce an isolated ADR format. The proposed decision is captured in TASK-013 as **prototype before deciding**, not accepted.

Proceed with the spike. If it proves native sandboxing, real approval/denial, interruption and fail-closed disconnect recovery on the selected Codex version, migrate Pipeon top-level path behind a flag. Retain CLI integration through migration and retain codex exec for bounded workers.
