# TASK-015 Backlog-Driven Remote Tasks And Multi-Machine Execution

## Goal

Provide package-owned DorkPipe workflows for two related but distinct remote-execution paths:

1. execute one explicitly selected, decision-ready backlog item as a bounded remote Codex task;
2. execute one DorkPipe task graph across user-owned DockPipe nodes and compatibility surfaces.

The first path turns an item from `docs/agents/task-index.yaml` and its linked task document into a
bounded remote Codex task. The second schedules implementation, validation, repair, and aggregation
without turning DockPipe into a cluster scheduler. They share immutable dispatch/result artifacts and
approval discipline, but neither path depends on the other.

The remote Codex task is the authority for asynchronous task state. DorkPipe retains only immutable
dispatch and result artifacts; it must not depend on a recurring master process, prose continuity
memory, or a shared-worktree inference loop to resume work after interruption.

## Why This Is A Separate Task

TASK-007 shipped the generic software-development workflow and repo task-pack model. This task
applies that model to the DockPipe standardized backlog and a remote-task adapter. TASK-013 remains
the separate host-resident App Server path for interactive top-level sessions; this task must not
couple remote Cloud task lifecycle to that adapter.

## Proposed Contract

### Backlog input

- `docs/agents/task-index.yaml` remains the single open-only entrypoint.
- The first workflow requires an explicit task ID and bounded slice reference. It reads only that
  index entry, its linked task document, `AGENTS.md`, and the docs routed for the selected task type.
- It must reject closed, absent, malformed, ambiguous, externally active, or decision-blocked items
  before any remote submission.
- A future `--next` selector is allowed only after the standardized backlog records deterministic
  readiness and ownership metadata. It must never infer readiness from historical prose updates.

### Remote execution

- A package-owned Codex Cloud adapter submits exactly one task through the installed CLI's remote
  task surface, with an explicitly selected Cloud environment and branch.
- The compiled request contains the baseline commit, allowed paths, hard boundaries, task slice,
  linked source-of-truth files, and required validation. It must not serialize local secrets,
  generated state, broad unreviewed workspace context, or a resolved prompt as durable repo config.
- DorkPipe records a stable dispatch artifact containing the remote task ID, request fingerprint,
  selected environment/branch reference, and safe submission metadata. Remote status, diff, and
  result retrieval are keyed by that ID instead of a local master-agent state file.
- Remote creation is explicit. Polling/status is read-only. Applying a remote diff, checkpointing,
  publishing, or starting a new item remains a separate governed action with the existing approval
  and Git lifecycle boundaries.

### Completion and reconciliation

- Treat a provider or agent terminal signal as an untrusted `completion_candidate`, never as an
  authoritative completion or permission to mutate the local checkout.
- A host-side package adapter records the candidate against the dispatched remote task ID, rejects
  stale, duplicate, replayed, or mismatched events, and deterministically verifies the expected
  result/status, allowed artifact references, required validation receipts, diff boundary, and
  absence of pending approval or halt state before emitting `ready_for_review`.
- For a host-resident App Server session, obtain the signal from the supervised provider adapter;
  do not give the agent an arbitrary MCP lifecycle-control tool. For a Cloud task, reconcile its
  remote task ID through the provider's status/diff/result surface (or a proven signed callback),
  without exposing the local MCP server to the remote worker.
- `ready_for_review`, local apply, checkpoint, and publish remain distinct operation-result and
  approval transitions. A completion candidate can never trigger apply or publish directly.

### Workflow shape

1. `backlog.inspect` resolves and validates one selected task/slice.
2. `backlog.compile` materializes a reviewable task request artifact.
3. `backlog.dispatch` submits one remote task and records its identifier.
4. `backlog.status` and `backlog.diff` reconcile only the recorded remote task.
5. `backlog.apply` requests explicit approval, applies the reviewed remote diff locally, and then
   delegates checkpoint/publish to the runtime-owned Git lifecycle.

The implemented vertical slice stops after `inspect`, `compile`, compatibility preflight,
fixture-backed `dispatch`, untrusted `completion_candidate` ingestion, and one fixture-backed remote
status observation. It does not create a scheduler, live-poll, retrieve diff/result evidence,
validate remote work, advance to `ready_for_review`, auto-apply, auto-commit, auto-push, or create a
cross-task orchestrator.

## Current Status (2026-07-19)

The first vertical slice is implemented as the package-owned `backlog.remote` workflow and dedicated
orchestration-helper commands:

- `backlog.inspect` requires one exact `TASK-NNN`, one trimmed single-line bounded slice, and one
  exact baseline commit. It strictly loads `docs/agents/task-index.yaml`, resolves one exact linked
  document, verifies that document's heading matches the selected task ID, and records source
  digests in `backlog-selection.json`.
- `backlog.compile` writes deterministic `remote-request.json` and `remote-request.md`. Their shared
  fingerprint binds the selected task/path/slice/baseline, explicit environment and branch refs,
  allowed paths, hard boundaries, required validation, and only `AGENTS.md`, the index, the linked
  task, and explicitly routed source files with digests.
- `backlog.dispatch` is fixture-only. It writes `remote-task.json` with one opaque task ID, request
  and compatibility fingerprints, environment/branch refs, deterministic fixture time, and adapter
  identity. The artifact records `provider_invoked: false` and no status, diff, result, apply,
  commit, push, or publication capability.
- `backlog.completion_candidate` ingests one strict fixture with separate candidate and replay
  identities, source adapter identity, remote task ID, request and dispatch fingerprints, explicit
  environment/branch refs, canonical observation time, and exactly one untrusted `completed` claim.
  Its reviewable `completion-candidate.json` has only `state: completion_candidate`, records the
  terminal claim as untrusted, and leaves every retrieval, validation, review, apply, commit, push,
  and publication transition false.
- Artifact-only follow-up and candidate ingestion validate the immutable request, compatibility,
  and dispatch artifacts; neither rereads the checkout or prose state.
- `backlog.status` ingests one strict fixture with separate observation and replay identities bound
  to the accepted candidate identity and full candidate fingerprint plus the immutable task,
  request, dispatch, adapter, environment, and branch identity. The canonical observation time must
  be later than both dispatch and candidate observation times. `remote-status.json` records only
  untrusted, non-authoritative fixture status evidence, remains at `state: completion_candidate`, and
  leaves `ready_for_review`, diff/result retrieval, validation, apply, commit, push, and publication
  false.
- Artifact-only status retrieval revalidates the immutable request, compatibility, dispatch, and
  complete accepted candidate artifact; it does not reread the checkout or backlog prose.

The package proof rejects absent, malformed, unknown, and ambiguous IDs; malformed index entries;
missing, escaping, mismatched, or closed linked task paths; empty, whitespace-padded, multiline, or
otherwise malformed bounded slices; invalid baselines; and explicitly blocked or externally active
fixture entries. Rejected inspection writes a deterministic rejection code but no request or
dispatch artifact. Temporary consumer copies prove repeated-run determinism, no consumer mutation,
no live provider invocation, no Git/SSH/network tool invocation, and no live status polling,
diff/result retrieval, apply, commit, push, or publication.

The canonical index remains unchanged and open-only. Package-owned fixtures use an optional
`dispatch_state` solely to represent `blocked`, `external_active`, and `closed` deterministically.
Standardizing readiness plus ownership metadata is still required before any future `--next`
selector; prose remains non-authoritative.

The installed CLI documents `codex cloud exec --env <id> --branch <branch> [query]`, but its help
does not expose a machine-readable submission receipt or a stable task-ID response schema. Parsing
undocumented terminal text would not be a safe resumable identity contract, so live submission
remains unimplemented.

The package now has a fail-closed compatibility preflight before fixture dispatch. Its narrow tracked
fixtures record the exact read-only `codex-cli 0.144.1` surfaces from `codex --version`,
`codex cloud --help`, and `codex cloud exec --help`. The resulting
`remote-adapter-compatibility.json` is bound to the immutable `remote-request.json` fingerprint and
explicit environment/branch refs. It records the required command surface, documented
`--env <ENV_ID>` and optional `--branch <BRANCH>` inputs, snapshot digests, receipt/task-ID support,
compatibility status, enabled adapter modes, and the exact fail-closed gap. That CLI version
documents neither a machine-readable submission receipt nor a safely recoverable stable opaque task
ID, so compatibility is `unsupported`, live submission is disabled, and fixture dispatch remains
the only enabled adapter.

Temporary consumer proofs show repeated compatibility and completion-candidate artifacts are
byte-for-byte deterministic; the preflight itself leaves no `remote-task.json`; and malformed
contracts emit deterministic operation-result failure evidence. Completion ingestion rejects an
observation at or before dispatch as stale; rejects wrong task, request, dispatch, adapter,
environment, or branch bindings; rejects duplicate candidate IDs and replayed replay IDs; and fails
closed on malformed fixtures or tampered request, compatibility, or dispatch artifacts. Rejection
writes no candidate, status, diff, result, validation, review, or apply artifact. A duplicate or
replay after acceptance leaves both the accepted candidate and dispatch bytes unchanged.

Remote-status proofs show byte-for-byte deterministic output across clean runs and artifact-only
restart after the consumer checkout is removed. Status ingestion rejects observations at or before
the accepted candidate time; wrong candidate, task, request, dispatch, adapter, environment, or
branch bindings; duplicate observation IDs; replayed replay IDs; malformed fixtures; untrusted
claims other than the one fixture `completed` status; and tampered request, compatibility, dispatch,
or candidate artifacts. Every rejection writes no status, review, diff, result, validation, or apply
artifact and leaves the accepted candidate and dispatch bytes unchanged. Operation-result evidence
records success and deterministic stale, duplicate, replay, mismatch, malformed, and tampered
rejection reason codes.

The complete offline workflow invokes no Codex, Git, SSH, network, live status polling, diff/result
retrieval, validation, apply, commit, push, or publication surface. Existing selection, request,
compatibility, fixture dispatch, follow-up, and completion-candidate artifacts remain byte-for-byte
deterministic, and software.dev/Example Brain behavior is preserved. The next bounded remote-task
slice is fixture-backed remote diff evidence retrieval bound to the accepted candidate and status
observation, still without result retrieval, validation-receipt reconciliation, diff verification,
or any `ready_for_review`, apply, or publication transition. A live Codex Cloud adapter remains
blocked until a future installed CLI documents a machine-readable receipt with a stable opaque task
ID.

## Boundaries

- Keep Codex Cloud CLI integration, backlog parsing, prompt compilation, and task artifacts inside
  DorkPipe package workflows/assets/resolvers. Do not add a `dockpipe backlog` engine command or
  Codex-specific behavior under `src/lib` or `src/cmd`.
- Treat remote task submission as a cloud-backed governed lane: declare cost/attempt policy, record
  selected lane and environment, and halt before unapproved spend or mutation.
- Preserve the existing task-index/task-document format as the source of truth. Add the smallest
  structured readiness field needed for a future automatic selector rather than creating a second
  queue or copying status into generated state.
- A remote task owns neither the local workspace nor Git publication. It returns a remote diff and
  result; local application, validation, checkpoint, and publication retain their existing explicit
  approval boundaries.
- Do not promise Desktop sidebar visibility as a workflow guarantee until the CLI-to-Desktop task
  identity mapping is proven on supported accounts and environments.

## Required Artifacts

- `backlog-selection.json`: selected task ID, linked task path, bounded slice, baseline, and
  deterministic rejection reason when not dispatchable.
- `remote-request.md` and `remote-request.json`: reviewable compiled request and safe metadata.
- `remote-adapter-compatibility.json`: inspected adapter/CLI contract, documented inputs and receipt
  capability, exact fail-closed reason, and immutable request/target binding.
- `remote-task.json`: remote task ID, request fingerprint, environment/branch reference, and
  submission timestamp, plus the compatibility and dispatch fingerprints.
- `completion-candidate.json`: candidate/replay identity, immutable task/request/dispatch/adapter/
  target binding, deterministic observation time, untrusted terminal claim, and only the
  `completion_candidate` lifecycle state.
- `remote-status.json`: status observation/replay identity, full accepted candidate fingerprint,
  immutable task/request/dispatch/adapter/target binding, deterministic later observation time, and
  only untrusted fixture status evidence at the `completion_candidate` lifecycle state.
- `remote-diff.patch` and `remote-result.json`: future fetched remote evidence; no raw credentials or
  hidden provider transcript.
- operation-result events for inspect, compile, compatibility, dispatch, completion-candidate
  ingestion/rejection, status, diff, apply, and failure.

## Acceptance Criteria

- A fixture task index plus linked task document deterministically compiles one explicit bounded
  request and rejects closed, unknown, malformed, ambiguous, active-external, and blocked entries.
- The adapter fixtures prove dispatch, completion-candidate, and status-observation parsing, record
  one opaque remote task ID, and never call a live provider by default. Diff/result parsing remains
  future work.
- The compatibility fixture proves the exact documented CLI submission surface, fails closed when a
  machine-readable receipt or stable opaque task ID is absent, and cannot create a task identity.
- Completion-candidate fixtures prove that stale, duplicated, replayed, mismatched, malformed, or
  tampered signals cannot create or advance beyond `completion_candidate`. Reconciled evidence is
  still required before a future slice can define `ready_for_review`.
- Remote-status fixtures prove that stale, duplicated, replayed, mismatched, malformed, or tampered
  observations cannot create an artifact or advance beyond `completion_candidate`; accepted status
  evidence remains explicitly untrusted and cannot authorize diff/result retrieval or review.
- A restart can recover identity solely from the immutable request, compatibility, and
  `remote-task.json` artifacts; it does not need the consumer checkout, a cron worker, or a mutable
  prose status record.
- Remote apply is impossible without an explicit reviewed action. Checkpoint and publish remain
  separate runtime-owned requests.
- The workflow is package-owned, uses standard artifact scopes, and introduces no engine or
  Pipeon-specific special case.
- An opt-in live compatibility test proves the installed CLI/environment contract and separately
  records whether submitted tasks become visible in the intended Codex remote UI.

## Open Decisions

- Which resolver/profile owns the configured Cloud environment and selected branch policy.
- The minimum standardized readiness/ownership fields needed before a safe `--next` backlog
  selector can replace explicit task selection.
- Whether remote task results should be applied only through the Codex CLI or through a
  provider-neutral DorkPipe remote-task adapter contract.
- Whether any provider callback can meet the correlation and signature requirements; otherwise
  status/diff/result reconciliation remains the sole completion source for remote tasks.
- Which user-facing surface first consumes task status: CLI only, Pipeon, or Codex Desktop mapping
  once the identity bridge is proven.

---

# Multi-Machine DockPipe Execution Extension

## Scope And Evidence

This extension is a DorkPipe scheduling concern, not a request to make every DockPipe installation a
network worker. It was assessed against the current monorepo surfaces:

| Area inspected | Existing evidence | Consequence for this task |
| --- | --- | --- |
| DockPipe engine (`src/lib/`, `src/core/runtimes/`) | Generic workflow execution, host/Docker/VM runtimes, QEMU Windows helper assets, WSL guidance, runtime-owned Git sessions, run-scoped cleanup. | A node already has the local execution/lifecycle boundary; core must stay generic. |
| DockPipe results/events (`docs/runtime/operation-results.md`, `src/lib/infrastructure/operation_event.go`) | Canonical `OperationResult` records can be emitted as JSONL operation events with IDs, status, timing, and errors. | Keep the inner event unchanged; add distributed correlation outside it. |
| DockPipe artifacts and sessions (`docs/runtime/artifacts.md`, `docs/runtime/git-runtime-sessions.md`) | Scoped artifacts, session metadata, worker leases, checkpoints, sync/publish lifecycle, and future distributed-session intent. | Exact-commit execution and cleanup can reuse runtime primitives; graph ownership must remain above them. |
| DorkPipe package (`packages/dorkpipe/`) | DAG parsing/validation, topological scheduling, bounded parallel task execution, dependency artifacts, follow-up reruns, repair, budgets, approval, merge, and verification. | DorkPipe is the natural owner of node selection, graph state, retries, and final aggregation. |

`packages/dorkpipe/` is a first-party package in this DockPipe checkout, not a separate Git checkout
here. Its package boundary is nevertheless the DorkPipe product boundary for this decision.

## Decision

Adopt this responsibility boundary:

```text
DorkPipe scheduler and graph state
  -> node-execution adapter / outer transport envelope
    -> optional DockPipe node endpoint or existing trusted transport
      -> DockPipe local workflow execution
        -> host | Docker | QEMU | WSL | future runtime
```

- **DockPipe executes one assigned contract on one node or runtime.** It owns local policy
  enforcement, approvals presented at that node, process-tree termination, runtime teardown,
  artifacts, local operation results, and capability observation.
- **DorkPipe decides where, when, and why work executes across nodes.** It owns the graph, placement,
  dependency state, fan-out, retries/repair, distributed approval state, budgets, aggregation, and
  final graph outcome.
- **Transport is replaceable.** The first version uses a user-managed trusted transport; a persistent
  endpoint is an optional later capability, not an implied DockPipe daemon.

This confirms the hypothesis. The location, availability, and scheduling concepts are orchestration
concepts; putting them in DockPipe core would couple a standalone local executor to a cluster-control
plane it does not need.

## Architecture Options

| Option | Layering and portability | Security and operations | Verdict |
| --- | --- | --- | --- |
| A. Optional DockPipe node service | Clean local-executor endpoint if it exposes only a narrow execution contract; preserves third-party use. | Requires service install, mTLS/enrollment, revocation, binding/firewall UX, reconnect semantics, and durable local request state. | Viable Phase 4+ option; do not make it the first dependency. |
| B. DorkPipe-owned worker service calling local DockPipe | Keeps DockPipe networking-free, but duplicates local execution wrapping, health, cancellation, and policy translation in DorkPipe. | DorkPipe becomes responsible for a privileged long-running host agent and risks bypassing DockPipe semantics. | Do not use as the permanent default. It is acceptable only as a thin transport adapter with no independent executor. |
| C. Shared protocol with separate implementations | Can avoid a DorkPipe-specific wire format and permit optional DockPipe or third-party endpoints. | A protocol package still needs versioning, identity, authorization, and replay protection; premature sharing can freeze an immature design. | Define a small versioned contract after the existing-transport slice proves it; keep it transport-neutral. |
| D. Existing trusted transport first | Maximally local-first and portable: invoke the installed local DockPipe CLI through SSH or another user-managed private path. | Reuses user-owned network/auth/firewall controls; requires a careful wrapper for event streaming, cancellation, and artifact retrieval. | Recommended first vertical slice. |

The default must not expose a public listener, require DockPipe-hosted infrastructure, or introduce a
generic remote shell. A later service, if justified, accepts only authenticated, allow-listed
DockPipe execution requests and cannot be a general command relay.

## Responsibilities

### DockPipe core

Keep or add only reusable local-node primitives:

- execute an assigned workflow/task contract locally through a selected host, Docker, QEMU, WSL, or
  future runtime;
- report observed host, runtime, guest, toolchain, and policy capabilities without making placement
  decisions;
- emit canonical operation results/events, logs, artifact references, run identity, cancellation
  outcome, and cleanup outcome;
- honour cancellation by terminating the local process tree and performing the same scoped teardown
  as a local run;
- support exact source revision/workspace preparation through runtime-owned Git lifecycle APIs;
- optionally expose these same local primitives through a narrowly scoped endpoint in the future.

DockPipe core must **not** gain node enrollment, scheduler persistence, task-graph state, dispatch
queues, leases between machines, retries, repair policy, health-based placement, cost/risk placement,
distributed approvals, artifact fan-in, coordinator hosting, or a DorkPipe-specific protocol.

### Optional DockPipe node component

Only after a transport-neutral contract is proven, an optional `dockpipe node` component may provide
an authenticated endpoint for local execution, capability snapshots, event streaming, cancellation,
artifact retrieval, and health. It should be an installable Windows service or systemd service, not
part of normal `dockpipe` CLI startup.

It owns local request deduplication and cleanup recovery for a request it accepted. It does not own
worker enrollment policy, global leases, task selection, graph persistence, or final success.

### DorkPipe scheduler/orchestration

DorkPipe must add the distributed layer:

- node inventory/configuration and availability state;
- target/capability matching, including separate host and guest facts;
- graph-level execution leases, idempotency keys, dispatch, cancellation propagation, and reconnect
  handling;
- fan-out, dependency tracking, result reconciliation, targeted retry/repair, and budget/policy
  decisions;
- exact-commit source plan and per-target checkout receipt;
- outer envelopes for event/result/artifact correlation and immutable graph-run audit records;
- graph-level approval gates and the final success/failure decision.

No two edit tasks may receive the same mutable workspace by default. Version one has one
implementation owner; all validation targets use the exact resulting commit. Concurrent edits need
explicit ownership, isolated branches/workspaces, and a reconciliation task before they are enabled.

### Shared protocol package (conditional)

Do not create a shared package in Phase 1. If a second transport or an optional DockPipe endpoint is
implemented, extract a small transport-neutral `node-execution.v1` contract owned jointly by the
projects. It contains capability snapshot, execution request/receipt, event envelope, cancellation,
artifact manifest, health, identity, and version negotiation. It contains no scheduler decisions,
provider/model details, or generic-shell command field.

## Target Model And Authoring Boundary

Placement belongs in a **DorkPipe scheduler extension over DockPipe task contracts**, not in generic
DockPipe workflow YAML. A DockPipe workflow specifies *what local work does*; DorkPipe specifies
which compatible surface should receive that local contract. This keeps existing local workflows
backward compatible and allows the same workflow to be placed on several nodes.

The target model must preserve the compatibility surface:

```yaml
# DorkPipe task/scheduler authoring; not a DockPipe runtime selector.
target:
  requires:
    host_os: linux
    runtime: qemu
    guest_os: windows
    capabilities: [qemu, windows-ci-image]
```

`host_os: windows, runtime: host`, `host_os: linux, runtime: qemu, guest_os: windows`, and
`host_os: windows, runtime: wsl, guest_os: linux` are distinct targets. `node:` is an explicit
pin; `target: local` is a DorkPipe shorthand for the scheduler host. Capability records also need
runtime version/profile, relevant guest image or snapshot identity, available toolchains, policy
class, and freshness timestamp.

Fan-out is likewise DorkPipe authoring:

```yaml
strategy:
  matrix:
    target:
      - requires: {host_os: linux, runtime: host}
      - requires: {host_os: windows, runtime: host}
      - requires: {host_os: linux, runtime: qemu, guest_os: windows}
task:
  workflow: test.cross-platform
```

It expands into separate graph nodes, each dispatching the unchanged local DockPipe workflow.
The syntax remains a proposal until DorkPipe's current task schema and package contract are extended
with fixtures, validation, docs, and migration rules.

## Source, Event, Result, And Artifact Contract

Use commit-based synchronization initially. The implementation task creates a reviewed checkpoint or
commit; each validation target fetches and checks out that immutable commit in a runtime-owned
workspace. A shared remote Git repository is the preferred transfer mechanism. Git bundles are a
fallback for air-gapped/private-LAN cases. Patch, network-file, and workspace-snapshot transfer are
not version-one defaults because they weaken reproducibility and authority boundaries.

Keep DockPipe's inner events unchanged. DorkPipe records an outer immutable envelope, for example:

```yaml
graph_run_id: dorkpipe-912
node_id: office-windows
dockpipe_run_id: dp-412
task_id: verify-windows-host
sequence: 184
event: # unchanged DockPipe operation event
  schema: dockpipe.operation_event.v1
  type: operation_result
  status: done
```

At dispatch time, generic correlation keys such as `run_id`, `request_id`, and `task_id` may be
injected into the DockPipe execution context and its existing ID map. `graph_run_id`, placement, and
other DorkPipe concepts stay in the outer envelope. Every terminal target receipt must record:

- DorkPipe graph/run/task IDs, node identity, local DockPipe run ID, and sequence boundaries;
- tested commit/ref and checkout verification receipt;
- physical node, host OS, runtime, guest OS, QEMU image/snapshot, tool/capability snapshot;
- policy and approval context, event log/artifact manifest integrity, cancellation/cleanup outcome;
- normalized status, failure classification, and safe diagnostic references.

Artifacts remain content-addressed or checksum-manifested where transferred. The scheduler preserves
the original node artifact manifest and records retrieval failures rather than silently treating a
remote path as an artifact.

## Security Boundary

The first transport inherits a user-managed private network (LAN, VPN, or overlay) and must bind no
new public listener. The scheduler authorizes a named node for an allow-listed contract, not an
arbitrary command. Requirements before a persistent endpoint is accepted include:

- separate node and scheduler identities, mutual authentication, enrollment/revocation, and scoped
  task authorization;
- request IDs, short leases, expiry, idempotency/deduplication, monotonic event sequence numbers,
  and replay rejection;
- capability claims treated as observed/signed evidence, not a reason to grant extra authority;
- secrets resolved only at the authorized local execution boundary and never serialized into graph
  artifacts, envelopes, or logs;
- cancellation authenticated and bound to the original request; local cleanup reports success or
  residue explicitly;
- least-privilege service accounts, private-interface binding, optional narrowly scoped firewall
  assistance, audit logs, and node quarantine/revocation after suspected compromise;
- artifact/event integrity checks and no remote provider callback that can apply, publish, or expand
  authority without local reconciliation and approval.

## Recommended First Vertical Slice

Use **SSH over a user-owned LAN/VPN/overlay to Windows OpenSSH** as the initial transport. It avoids
new public infrastructure and is available from a Linux scheduler without requiring WinRM firewall
and remoting policy setup. PowerShell may run inside the remote DockPipe Windows workflow; it is not
the control-plane transport. WinRM/PowerShell Remoting remains a later adapter for organizations
that standardize it.

Slice:

1. Linux DorkPipe receives one implementation commit and one manually configured `office-windows`
   target with a static, reviewed capability manifest.
2. DorkPipe opens a single SSH execution session, prepares a Windows runtime-owned workspace at the
   exact commit, and invokes the installed local DockPipe validation workflow in the foreground.
3. A thin package-owned remote adapter streams/collects DockPipe's existing JSONL operation events,
   stdout/stderr references, terminal result, and artifact manifest into a DorkPipe outer envelope.
4. Cancellation is sent against the same request; the Windows adapter proves foreground process-tree
   termination and reports the DockPipe cleanup receipt. A failed cleanup is a distinct failed or
   degraded terminal result, never a successful cancellation.
5. DorkPipe records the per-target receipt and does no automatic repair, retry, apply, commit, or
   publish in this slice.

The slice deliberately excludes a daemon, auto-discovery, enrollment, QEMU dispatch, dynamic
scheduling, and generic remote shell. It must use a fixture/local fake for transport and an opt-in
Windows compatibility test; no live remote machine is required in the default package test suite.

## Phased Backlog

1. **Architecture decision:** define `node-execution.v1` shapes as package-owned design fixtures,
   target capabilities, outer correlation, security gates, and SSH acceptance tests.
2. **One remote Windows target:** implement the recommended slice above with exact-commit checkout,
   canonical event collection, cancellation/cleanup verification, artifacts, and restart-safe receipts.
3. **Multi-target validation:** add concurrent Linux-host, Windows-physical-host, and Linux-QEMU
   Windows-guest targets; aggregate per-target results and dispatch only an explicit repair task.
4. **Persistent nodes (conditional):** prove whether SSH limitations justify an optional endpoint;
   then add enrollment, health, capability refresh, reconnect, revocation, and offline behavior.
5. **Installer/UX (conditional):** role selection, Windows service/systemd management, private
   binding, optional firewall assistance, diagnostics, node naming, and node-management commands.
6. **Advanced scheduling:** leases, availability/load/risk/cost placement, retries, quarantine,
   disposable VMs/cloud workers, Mac, GPU, and third-party endpoint adapters.

## Acceptance Criteria For This Extension

- Standalone, local-only DockPipe workflows retain their current behavior and require no service.
- DorkPipe owns graph and placement decisions; DockPipe receives only a local execution contract.
- Host/runtime/guest dimensions are separately matched and reported.
- Existing DockPipe operation events/results are reused inside an outer DorkPipe envelope.
- The first slice runs one exact commit on a configured physical Windows target through SSH, returns
  structured results/artifacts, and proves cancellation plus cleanup handling.
- Default execution needs neither DockPipe-hosted cloud infrastructure nor a public listener.
- A failure cannot silently duplicate a task, replay a stale cancellation, hide cleanup residue, or
  automatically publish a change.
- Persistent-service work cannot start until its added value over the SSH adapter is documented with
  concrete cancellation, reconnect, capability-refresh, or UX evidence.

## Open Decisions For The Extension

- Whether the current DockPipe process-runner/cancellation primitives need one small generic
  machine-readable cancel/status API before the SSH slice can make its cleanup guarantee.
- The exact target schema location and migration path in the DorkPipe orchestration contract.
- Whether the first Windows target requires a preinstalled system OpenSSH service, a user-launched
  SSH endpoint, or an organization-owned private overlay; all remain user-managed prerequisites.
- Artifact transfer limits, retention, and checksum/signature policy for large guest logs/images.
- When a second real transport is sufficient evidence to extract `node-execution.v1`, and whether an
  optional DockPipe endpoint or a DorkPipe adapter should implement it first.
