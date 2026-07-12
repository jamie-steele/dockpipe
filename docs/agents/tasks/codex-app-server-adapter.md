# TASK-013 Codex App Server adapter for top-level orchestrators

## Epic

### Problem statement

Pipeon and future top-level DockPipe orchestrators use host codex exec, buffered output and transcript-file discovery. That preserves native workspace sandboxing but cannot represent live turn state, native approvals, interruption completion or connection loss reliably. This is separate from successful disposable codex exec workflow workers.

### Expected value

- replace transcript/timestamp discovery with typed thread ownership;
- give Pipeon live progress, follow-up, user-input, approval, cancellation and recovery states without raw provider protocol;
- improve audit evidence with correlated provider events and decisions;
- preserve Codex sandbox/escalation and DockPipe user approval authority;
- retain a provider-neutral contract and CLI fallback.

### Decision status

**Prototype before deciding.** Canonical research: docs/research/codex-app-server-top-level-orchestrators-2026-07.md.

Repository has no ADR convention; this is the proposed-decision record, not an accepted ADR.

### Scope

- Codex App Server adapter for host-resident long-running top-level sessions;
- generic session/state/event/approval contract usable by other providers;
- Pipeon as first feature-flagged consumer;
- process supervision, persistence, audit, cancellation and recovery;
- controlled tests using installed Codex.

### Non-goals

- replacing bounded codex exec workflow workers;
- Codex-specific engine changes under src/lib or src/cmd;
- WebSocket/remote-control transport;
- automatic approval, auto-review, full access, thread shell-command or raw protocol in Pipeon;
- production abstraction during the spike.

### Architectural constraints

- App Server runs on host; Codex native sandbox remains active.
- Codex decides whether escalation is needed; DockPipe presents, records, grants or denies it.
- Host operations are never silently approved.
- Adapter owns provider JSON-RPC; generic contracts expose provider-neutral events only.
- Crash/disconnect is never reported as safe continued execution.
- codex_exec remains available and no active prompt is replayed into it.
- Pipeon uses the generic contract and receives first migration advantage.

### Security constraints

- stdio only initially; no external listener;
- explicit workspace-write/declared roots; reject full access and thread shell-command;
- human reviewer only; no auto-review;
- approval uses process/thread/turn/item/request correlation and one-time persistence;
- no credential copying; redact sensitive raw payloads;
- default deny on timeout, disconnect, stale event, schema mismatch or malformed message;
- append-only event/approval audit with gap detection and reconciliation.

### Migration and rollback

1. Complete constrained protocol spike for selected Codex version.
2. Add provider-neutral contracts and selectable codex_app_server adapter.
3. Move one Pipeon direct top-level session behind an opt-in feature flag.
4. Run contract/integration/security evidence review.
5. Migrate remaining consumers only after maintainer decision; retain codex_exec and bounded-worker behavior.

Rollback disables adapter for new sessions. Existing App Server sessions become Disconnected until explicitly reconciled; never replay active turn. Retain audit records and offer user-guided resume/fork only after recovery checks.

### Dependencies and unresolved questions

- Stable-enough schema/capabilities for selected Codex version.
- Genuine approval-producing sandbox test on Windows and supported hosts.
- Effective-config/policy inspection needed to prove resumed sandbox equivalence.
- Existing ChatGPT auth reuse without adapter credential access.
- Event retention/redaction and user-visible reconnect policy.
- Maintainer decision on ADR process and default provider.

### Epic acceptance criteria

- Spike proves launch, initialization, start/resume, stream, approval+denial, interrupt, clean exit, child death, native sandbox and fail-closed recovery.
- Codex types do not leak into generic orchestration/Pipeon APIs.
- Approval cannot replay or cross-apply.
- Pipeon opt-in and CLI fallback work together.
- Bounded worker codex exec remains unchanged.
- Schema gates, audit/security tests and operations/migration docs pass.

## Child backlog items

| ID | Type | Task | Acceptance criteria |
| --- | --- | --- | --- |
| CAS-01 | Research | App Server protocol spike | Launch existing-auth stdio; initialize; start/read/resume; stream events; record stable schema/version; no production abstraction. |
| CAS-02 | Architecture | Provider-adapter contracts | Define provider-neutral session/state/event/approval interfaces; prove Codex types do not leak into Pipeon/generic layer. |
| CAS-03 | Implementation | Process supervision | Own child/job, JSONL I/O, startup/shutdown/liveness deadlines and fail-closed Disconnected. |
| CAS-04 | Implementation | Protocol client and initialization | Correlate requests; initialize/initialized; schema/capability gate; capture version, identity and config warnings. |
| CAS-05 | Implementation | Thread and turn lifecycle | Implement start/read/resume/follow-up/steer policy, ownership records and no duplicate turn guarantee. |
| CAS-06 | Implementation | Structured event normalization | Convert thread/turn/item/error/warning/token stream to ordered safe generic events; retain restricted raw audit payload. |
| CAS-07 | Implementation | Approval relay | Persist/correlate command/file/permission/MCP/user-input requests; require user decision; test denial and replay rejection. |
| CAS-08 | Implementation | Cancellation/interruption | Implement cancel intent, turn interrupt, terminal wait, bounded kill escalation and background-process risk report. |
| CAS-09 | Implementation | Persistence/resumption | Persist policy/thread/turn/process/event cursor; reconcile through fresh server without claiming work survived. |
| CAS-10 | Implementation | Audit/observability | Add redacted RPC journal, operation-result projection, progress/latency and event-gap alert. |
| CAS-11 | Security | Hardening | Enforce stdio, no shell/full-access/auto-review, policy validation, transport isolation, redaction and MCP allow-list. |
| CAS-12 | Testing | Contract tests | Fixture-test schema, state, duplicate/reorder/malformed messages, approval replay and policy mismatch. |
| CAS-13 | Testing | Controlled Codex integration | Test existing auth, stream, approve/deny, interrupt, sandbox, clean exit and process death. |
| CAS-14 | Migration | First Pipeon migration | Feature-gate one Pipeon top-level direct session; render normalized status/approval/recovery; retain CLI. |
| CAS-15 | Migration | Remaining top-level orchestrators | Inventory/migrate only compatible consumers after Pipeon evidence review. |
| CAS-16 | Migration | CLI fallback | Make adapter choice, safe fallback, no-replay rules and rollback telemetry explicit/tested. |
| CAS-17 | Documentation | Operations guidance | Document policy, approval, recovery, supported versions, diagnostics, Pipeon UX and rollback. |

### CAS-01 current evidence

On 2026-07-11, three workspace-sandboxed materialization probes reached a correlated `turn/completed` terminal event classified as `failed`; none sent `thread/resume`. All repeated the same terminal diagnostics: nine retriable `responseStreamDisconnected` errors, one `other` error, one warning, and terminal error kind `other`. A failure-gated, redacted `thread/read` reconciliation returned a result, proving that the App Server control plane remained responsive after the failed provider stream. Metadata-only probes on the recorded `codex-cli 0.144.1` baseline also completed `account/read`, `model/list`, `thread/start`, and `thread/read` successfully, retaining only the account-read result class, selected model/effort, catalog-verification flag, and sanitized CLI version. A credential-free TCP baseline then showed `api.openai.com:443` unreachable from the workspace sandbox but reachable from the host. One narrowly approved host-network materialization probe, with the identical pinned `gpt-5.6-terra` / `high` and workspace-write/network-disabled/user-review turn policy, reached correlated `turn/completed: completed` in 6.2 seconds and then returned `thread/resume: result`. This proves that the App Server must be host-resident so it can reach the provider, while the model turn itself remains restricted by its native sandbox policy; safe successful resume is proven in that host-resident shape. The harness validates and pins the selected model and effort on thread and turn start, and halts if a model-reroute event occurs. Its static checks assert the complete policy envelope, reroute correlation and fail-closed state, and rejection of full access, shell-command, auto-review, and empty model/effort inputs. No raw account, catalog, command, or RPC payload was retained.

### CAS-02 current decision

CAS-02 adds the unused `packages/dorkpipe/lib/providersession` contract package and its package-owned documentation. It defines provider-neutral session identity, lifecycle states, contiguous event sequencing, safe normalized events, user-input and approval requests, cancellation intent, recovery reference, and an adapter interface. `Disconnected` is fail-closed and can return to `Ready` only through verified recovery; every human approval requires the complete process-incarnation, connection, session, interaction, activity, request, and one-time decision tuple. The contract carries references and summaries only—no provider protocol unions, raw payloads, credentials, or provider-specific error types. Its source-boundary test rejects provider protocol identifiers, and focused tests cover ordering/duplicate/stale/gap rejection, approval correlation, user-input/cancellation references, and recovery transitions. CAS-03+ retain protocol lifecycle execution, event normalization, approval delivery, persistence, audit, and Pipeon wiring.

### CAS-03 current decision

CAS-03 adds the unused package-local `appserversupervisor` foundation. It directly owns one
host-resident child with private stdio (no listener, socket, shell, fallback process, credentials,
or raw-payload storage), observes JSONL framing only, and bounds startup, liveness, graceful
shutdown, and kill escalation. Startup failure, child exit, closed or malformed stdout, transport
loss, and deadline expiry each produce exactly one provider-neutral `providersession` state event:
fail-closed `Disconnected` with a safe reason class. A stopped supervisor cannot start again, so it
does not retry, resume, replay, or fall back. Native turns remain deferred: CAS-04+ must still set
the pinned `gpt-5.6-terra`/`high` and the workspace-write, declared-root, network-disabled,
user-review policy; host process placement itself grants none of those capabilities.

### CAS-04 current decision

CAS-04 extends only the package-local `appserversupervisor` with a private JSONL protocol client.
The one host-resident direct child now receives a bounded `initialize` request followed by the
`initialized` notification. Request IDs start at one, advance monotonically, and only the active
response can satisfy the request. The initialization gate requires an explicit accepted stable
schema version and required capability set; it retains only an allow-listed projection of the
provider version, `codex_app_server` identity class, bounded configuration-warning classes, and
the pinned `gpt-5.6-terra` / `high` policy configuration. It neither calls the model catalog nor
starts a thread or turn.

Malformed JSONL/envelopes, correlation mismatches, provider errors, rejected initialization,
schema/capability mismatch, model-reroute indications, request deadline expiry, transport loss,
or child exit produce the single safe `Disconnected` projection and stop the child. The protocol
client keeps frames, provider errors, IDs, prompts, account data, commands, stderr, and
credentials private and transient; none crosses into `providersession` or Pipeon. It adds no
retry, resume, replay, reconnect, fallback process, listener, socket, shell, credential access,
or persistence.

Deferred to CAS-05+: all thread/turn start/read/resume/follow-up/steer work; normalized provider
events; approvals/user input; interruption; persistence/resumption; audit; additional hardening;
Pipeon migration; and CLI fallback. Future turns must still explicitly enforce workspace-write,
declared writable roots, network disabled, and user review; host process placement grants none of
those capabilities.

### CAS-05 current decision

CAS-05 extends only the package-local `appserversupervisor` lifecycle client. After the completed
CAS-04 initialization gate it makes bounded `thread/start`, `thread/read`, `thread/resume`,
`turn/start`, and `turn/steer` requests. Provider thread and turn IDs remain private and are
projected only as opaque `providersession.SessionRef` and `Correlation` values. A single
supervisor owns one active steerable turn; duplicate/concurrent starts, stale lifecycle references,
correlation mismatches, non-steerable steering, bad response shapes/states, policy changes, and
any failure leave it safely `Disconnected`.

Every lifecycle request reconstructs the pinned `gpt-5.6-terra` / `high` native-turn envelope:
workspace-write with explicitly declared writable roots, network disabled, and untrusted policy
reviewed by the human user. Full access, shell commands, automatic review, fallback models or
providers, empty roots, and policy/model changes are rejected. The host App Server child remains
host-resident only for provider-stream reachability; that placement grants no additional turn
capabilities. No raw payload, prompt, account, command, error body, credential, retry, replay,
reconnect, fallback, or persistence is retained or introduced.

Deferred to CAS-06+: provider-event normalization and terminal turn state; approval/user-input
relay; interruption; persistence/reconciliation; audit; further hardening; Pipeon migration; and
CLI fallback.

### CAS-06 current decision

CAS-06 extends only the package-local `appserversupervisor` notification path. It accepts a closed
allow-list of correlated thread, turn, item, progress, warning, error, token-usage, and terminal
turn notifications after CAS-05's initialization/thread/turn gates. Raw JSONL frames, token text,
item content, provider messages, error bodies, command/file data, credentials, and provider IDs
remain private and transient. The supervisor projects only contiguous, supervisor-owned neutral
`providersession.Event` progress summaries and opaque `SessionRef`/`Correlation` references.

One private item may be active within the one private active turn. Duplicate, stale, reordered,
cross-thread, cross-turn, cross-item, malformed, uncorrelated, unsupported, rerouted, transport,
or child-exit notifications fail closed through the single `Disconnected` projection. A validated,
exact terminal turn notification alone releases the active-turn invariant; it does not recover,
resume, replay, retry, or cancel work. `providersession` and Pipeon remain free of App Server
protocol/raw-payload types.

Deferred explicitly to CAS-07+: approval and user-input relay (CAS-07), interruption/cancellation
delivery (CAS-08), persistence/reconciliation/recovery (CAS-09), audit journal (CAS-10), broader
hardening (CAS-11), contract/integration expansion (CAS-12/13), Pipeon migration (CAS-14/15), CLI
fallback changes (CAS-16), and operations guidance (CAS-17).

### CAS-07 current decision

CAS-07 extends only the package-local `appserversupervisor` request path. It accepts the stable
command-execution, file-change, declared-permission, and tool user-input request notifications
only when they exactly match the current private process/connection/thread/turn/item invariant.
It projects neutral approval or user-input events with supervisor-generated opaque request and
one-time decision references. Command text, patches, paths, prompt/question text, provider request
IDs, messages, credentials, raw payloads, policy amendments, network requests, and session grants
remain private and transient. Only the bounded neutral one-turn approve/deny operation is exposed:
it maps to private command/file decisions, while permission requests are deny-only because the
neutral contract carries no grant subset. User-input requests have no answer operation in the
existing neutral contract and expire fail-closed.

Duplicate, stale, reordered, cross-thread, cross-turn, cross-item, uncorrelated, malformed,
unsupported, expired, transport-loss, child-exit, provider-error, and reroute conditions deny or
disconnect fail-closed. A successful private response still requires the exact matching
`serverRequest/resolved` notification before the supervisor returns to `running`; no terminal,
retry, replay, reconnect, cancellation, recovery, persistence, audit journal, or Pipeon behavior
is introduced. CAS-08+ remains explicitly deferred.

## Likely impact map

- packages/dorkpipe/lib: provider-neutral contracts, adapter package, state and tests;
- packages/dorkpipe/mcp/mcpbridge: normalized host session/approval operations;
- packages/dorkpipe/lib/cmd/dorkpipe/provider_pool.go: adapter selection retaining exec;
- packages/dorkpipe/resolvers/dorkpipe/assets/provider-pools/catalog.yml: capability policy;
- Pipeon extension: normalized session/event UI;
- DorkPipe/Pipeon tests and docs.

Do not modify those production areas for this research task.
