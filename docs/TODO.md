# Docs TODO

Cross-cutting follow-ups that should not live only inside one feature doc.

## Operations And Results

Current state:

- Core DockPipe now has a shared Go `OperationResult` contract in `src/lib/infrastructure` and it
  is the canonical source for unit name, status, timing, IDs, CLI rendering, and event-field
  mapping.
- Runtime-owned session volume work now uses the pattern for `session.volume.preflight`,
  `session.volume.create`, `session.volume.seed`, `session.volume.sync_in`, and
  `session.volume.sync_out`.
- Workflow host setup and workflow checkpointing now use the same operation-result rendering in the
  main CLI path.
- Runtime-owned helper containers now use stable DockPipe helper names and labels instead of
  leaving random Docker-generated names as the only operator clue.
- DorkPipe orchestration scripts do not reference core internals directly, but the main package
  shell path now mirrors the same `unit=... status=... duration_ms=...` pattern through shared
  helpers in `orchestrate-common.sh`.

Still open:

- Expand the core operation-result contract into the remaining important runtime actions such as
  publish, broader session creation lifecycle, auth discovery outside the current DorkPipe shell
  path, and other long-running runtime/bootstrap work that still prints one-off lines.
- Continue migrating package-owned scripts and package workflows that still use bespoke status
  wrappers, especially older `dev-stack` and optimizer-style logging, onto the same unit/result
  vocabulary.
- Expose a cleaner public CLI/SDK surface for package-owned scripts that want canonical
  operation-result emission without reimplementing helper formatting in shell.
- Push structured event usage further so session metadata, orchestration artifacts, and future
  machine-readable output depend on the shared result contract instead of handwritten event shapes.

## Orchestration And Validation

- Expand deterministic verification around orchestration apply/publish paths so missing sources,
  broken references, and contradictory validation claims fail before writes.
- Add package-owned deterministic source walkers for broad mounted roots and external corpora so
  cheap/local lanes consume bounded fact packets instead of pretending they performed source-root
  discovery on their own.

Managed session volume cleanup current state:

- Core DockPipe now auto-cleans managed session base volumes after successful workflow completion
  and after `dockpipe session publish` when the session uses `workspace.storage: volume`.
- Cleanup is runtime-owned and gated by preflight checks: managed worktree exists, local session
  branch exists and matches the workspace branch, and no active worker lease still depends on the
  volume.
- Cleanup writes operation-result lines and session events and records cleaned volume metadata in
  the session record.
- `DOCKPIPE_SESSION_VOLUME_AUTOCLEANUP=false` disables the default cleanup behavior when a user
  intentionally wants to keep the runtime-owned volume around.

Still open:

- Add explicit inspect/list/prune CLI behavior for retained or stale managed session volumes and
  related workspace artifacts.
- Broaden cleanup evaluation to more failure paths so runtime-owned cleanup is not limited to the
  current successful workflow completion and publish paths.
