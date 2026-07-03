# TODO-001 Operation Results Contract Rollout

## Current State

- Core DockPipe now has a shared Go `OperationResult` contract in `src/lib/infrastructure`.
- It is the canonical source for unit name, status, timing, IDs, CLI rendering, and event-field
  mapping.
- Runtime-owned session volume work now uses the pattern for `session.volume.preflight`,
  `session.volume.create`, `session.volume.seed`, `session.volume.sync_in`, and
  `session.volume.sync_out`.
- Workflow host setup and workflow checkpointing now use the same operation-result rendering in the
  main CLI path.
- `dockpipe build` now surfaces stable operation-result units for compile, package source builds,
  image artifact materialization, and clean-path behavior instead of relying on one-off build/image
  status strings.
- Operation results can now mirror to append-only JSONL when `DOCKPIPE_EVENT_LOG` is set. The
  canonical event schema is `dockpipe.operation_event.v1`, implemented in
  `src/lib/infrastructure`.
- Workflow runs now default `DOCKPIPE_EVENT_LOG` to `<artifact_root>/events.jsonl` and
  `DOCKPIPE_EVENT_INDEX` to `<artifact_root>/events-index.json`; the parent process env is set while
  the run is active so Go-side host setup/checkpoint events and child steps share the same ledger.
- `dockpipe get event_log`, `dockpipe get event_index`, and workflow `dockpipe scope` now expose the
  resolved operation-event ledger/projection paths so callers do not need to know the artifact
  layout.
- `dockpipe session inspect <id|latest> [--json]` now exposes the runtime-owned session metadata
  event log as `storage.event_log`.
- `dockpipe runs events --event-log <path> [--json]` can inspect the JSONL operation event ledger
  without requiring Postgres or PipeDeck.
- `dockpipe runs events --event-log <path> --index [<path>] [--json]` can rebuild a
  `dockpipe.operation_event_index.v1` JSON projection from the JSONL ledger for fast summaries and
  future UI bootstrap; omitting the index path uses `DOCKPIPE_EVENT_INDEX`.
- `dockpipe result --unit <name> --status <status> ...` now gives package-owned scripts and shell
  helpers a public core adapter for canonical operation-result rendering and JSONL mirroring.
- Runtime-owned helper containers now use stable DockPipe helper names and labels instead of
  leaving random Docker-generated names as the only operator clue.
- DorkPipe orchestration scripts do not reference core internals directly, but the main package
  shell path now delegates operation-result emission through `dockpipe result` from the shared
  helpers in `orchestrate-common.sh`, with a text fallback for older binaries.
- DorkPipe dev-stack logged operations now use the same `dockpipe result` adapter through
  `dev-stack-lib.sh`, while keeping the existing spinner/log-tail UX.

## Still Open

- Expand the core operation-result contract into remaining runtime actions such as publish, broader
  session creation lifecycle, auth discovery outside the current DorkPipe shell path, and other
  long-running runtime/bootstrap work that still prints one-off lines.
- Continue rolling the same result contract deeper into compile/package subcommands that still emit
  ad hoc detail lines beneath the now-normalized top-level build units.
- Continue migrating any newly added package-owned scripts that do meaningful long-running work onto
  `dockpipe result` or a shared package wrapper instead of adding new bespoke status formats.
- Add a rebuildable Postgres projection over operation-event JSONL and JSON/YAML indexes for
  PipeDeck, dashboards, search, and cross-run history.
- Push structured event usage further so session metadata, orchestration artifacts, host-action
  approval requests, and future machine-readable output depend on the shared result contract instead
  of handwritten event shapes.
