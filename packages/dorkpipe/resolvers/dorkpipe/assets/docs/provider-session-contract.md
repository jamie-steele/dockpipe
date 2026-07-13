# Provider-neutral top-level session contract

`providersession` is the CAS-02 contract for long-running, top-level provider sessions. It is a
pure type and validation package: it owns no process, transport, persistence, provider client, UI,
or approval delivery behavior.

## Public shape

- `SessionRef` identifies a provider and an opaque provider session.
- `State` is `ready`, `running`, `waiting_for_approval`, `waiting_for_user_input`, `completed`,
  `cancelled`, `failed`, or `disconnected`.
- `Event` is a bounded normalized state/progress/request/cancellation/recovery record. It carries
  safe references and summaries, never raw provider payloads or credentials.
- `ValidateNextSequence` requires contiguous event order and rejects duplicate, stale, and gapped
  records before a future adapter applies them.
- `Correlation` is the one-time decision tuple: process incarnation, connection, session,
  interaction, activity, request, and decision identity.
- `RecoveryRequest` binds an opaque bounded recovery-evidence reference to the exact session;
  adapter-local persistence and reconciliation decide whether that evidence is safe.
- `Adapter` describes future start, send, decide, cancel, and recover operations without choosing a
  provider implementation.

## Safety semantics

`disconnected` is fail-closed. It can return to `ready` only through verified recovery; a terminal
state cannot restart. A human decision requires the complete correlation tuple, preventing replay
or cross-session application.

## Future adapter mapping

A future App Server adapter maps its provider-specific thread to `SessionRef`, turn to
`InteractionID`, item to `ActivityID`, and approval request to `RequestID`. It owns all protocol
parsing and raw-payload handling; Pipeon receives only `providersession.Event` values.

CAS-03 adds a package-local, host-resident child supervisor. CAS-04 adds its private JSONL
initialization client: a bounded `initialize` request, `initialized` notification, monotonic
correlation, and schema/capability gate. CAS-05 adds bounded private thread/read/resume and
turn/start/steer lifecycle requests after that gate. It maps provider identifiers only into opaque
`SessionRef` and `Correlation` values, permits one active steerable turn, and rejects stale or
mismatched lifecycle references. Every request pins `gpt-5.6-terra` / `high`, workspace-write,
declared roots, network disabled, and human user review; protocol data, prompts, credentials, and
provider error bodies remain package-local and transient. Malformed envelopes, response mismatch,
provider errors, lifecycle/policy rejection, request deadline, transport loss, child exit, and
reroute indications are all one safe `disconnected` state event.

CAS-06 adds package-local structured notification normalization. It emits contiguous supervisor-owned
progress events with opaque thread/turn/item correlation and bounded allow-listed summaries only;
raw frames, token text, item content, messages, error bodies, commands, files, and credentials stay
private and transient. A correlated terminal turn event can release the private active-turn gate,
but does not implement recovery or replay. Approval relay, interruption, persistence, audit,
additional hardening, and Pipeon wiring remain deferred to CAS-07+.

CAS-07 adds package-local approval and user-input request relay. It creates opaque request and
one-time decision references only after exact process, connection, thread, turn, and item
correlation. It projects closed action classes and safe scope labels, never command text, patches,
paths, question text, provider request IDs, raw payloads, or permission/policy data. The neutral
contract now bounds approval decisions to one-turn `approve` or `deny`: command/file requests can
use both; declared-permission requests are deny-only because a granted subset would need a new
neutral contract surface. User-input requests are delivered as opaque references but have no answer
operation yet. Expiry and every stale, duplicate, malformed, unsupported, transport, child-exit,
provider-error, or reroute condition fail closed.

CAS-08 adds cancellation only through the existing neutral `CancellationIntent`: `user_requested`,
`safety_stop`, or `deadline_exceeded`. The package-local supervisor requires the exact active
process/connection/session/turn correlation, projects the opaque intent, and privately requests
the active turn interrupt. The accepted response is not cancellation completion; only the exact
correlated terminal `interrupted` notification projects `cancelled`. Timeout, transport loss,
child exit, response mismatch, malformed/provider-error/reroute input, a missing or
non-interrupted terminal, and any ambiguity disconnect and invoke the existing bounded shutdown
path. A background-process indication is reduced to the neutral
`background_process_risk_possible` summary only. CAS-09+ persistence, resumption,
reconciliation/recovery, audit, hardening, testing expansion, and Pipeon work remain deferred.

CAS-09 adds bounded package-local idle-session snapshots and an explicit validated
`RecoveryRequest`. The snapshot retains only safe session/policy/incarnation references, a
contiguous event cursor, and closed lifecycle/summary classes. Recovery launches a fresh
initialized supervisor, performs one exact private idle reconciliation read, and emits `ready` only
then. It never reconnects to a prior child or resumes/replays an active, pending, cancelled,
failed, unknown, or non-idle turn. Corrupt/stale evidence, policy/cursor mismatch, response or
transport ambiguity, child exit, provider error, reroute, and timeout remain bounded
`recovery_required`/`disconnected`.

CAS-10 adds an appserversupervisor-local, bounded, versioned audit journal. It stores only safe
operation/event outcome classes, opaque neutral session/correlation references, contiguous event and
journal cursors, and coarse progress/latency buckets. The journal is atomically replaced as bounded
append-only segments and is never an adapter operation surface: it cannot replay, resume, retry,
approve, deny, steer, cancel, or recover a turn. Raw frames/payloads, timestamps, prompts, questions,
commands, patches, paths, credentials, token text, provider IDs/error bodies, account/config data,
and process details are excluded. Missing, corrupt, oversized, stale, gapped, cross-session, or
unsafe audit evidence fails closed. A recovered idle session must match its retained audit cursor;
that evidence still never claims prior active or unknown work survived.

CAS-11 closes the remaining supervisor-local hardening gaps. Only the direct `codex app-server --stdio`
child shape is accepted; bounded constructor, policy, reference, snapshot, and audit values are required.
The policy stays pinned to `gpt-5.6-terra` / `high`, workspace-write, declared in-workspace roots,
network disabled, and human review. Unknown or duplicate initialization, event, server-request, or
MCP-progress extensions fail closed without retaining raw content. Disconnect and rejected recovery clear
private transport and active/pending state before bounded child cleanup. The journal stays descriptive
only and has no replay, retry, resume, recovery, decision, dispatch, or export operation.

CAS-12 contract-test expansion, CAS-13 controlled integration, and CAS-14+ migration and operations work
remain deferred.
