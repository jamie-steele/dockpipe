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
provider-error, or reroute condition fail closed. CAS-08 cancellation/interruption and all later
CAS-09+ persistence, recovery, audit, hardening, testing expansion, and Pipeon work remain
deferred.
