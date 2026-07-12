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
correlation, and schema/capability gate. It retains only safe provider-version, identity-class,
and configuration-warning classifications. Malformed envelopes, response mismatch, provider
errors, request deadline, transport loss, child exit, and reroute indications are all one safe
`disconnected` state event. Protocol data remains package-local; the neutral contract and Pipeon
receive no raw payloads or provider types.

Thread/turn operations, normalized provider events, approval relay, interruption, persistence,
audit, hardening, and Pipeon wiring remain deferred to CAS-05+.
