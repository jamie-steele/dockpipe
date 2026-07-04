# TASK-001 Operation Results Contract Rollout

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
- DorkPipe package source builds now emit per-tool `package.source.tool` units for the package-owned
  `dorkpipe`, `mcpd`, `skills-render`, and `orchestrate-helper` binaries.
- DorkPipe orchestration auth checks now emit `orchestrate.auth.preflight` start/done/fail units and
  host-login recovery after worker auth failures now emits `orchestrate.auth.recovery` units with
  provider, auth status, login policy, retry status, and duration.
- DorkPipe orchestration planning now emits `orchestrate.plan` start/done/fail units with workflow,
  normalized orchestration root, follow-up mode, plan path, and duration.
- DorkPipe orchestration approval now emits `orchestrate.approval` start/done/fail units with
  workflow, approval mode, decision, approved flag, approval artifact path, and duration.
- DorkPipe task hard failures now rely on the canonical `orchestrate.task status=fail` line instead
  of printing a second bespoke required-worker failure line.
- DorkPipe optimizer child iterations now emit `orchestrate.optimize.iteration` units, and target
  refresh reruns after applied optimizer output emit `orchestrate.optimize.refresh` units.
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
- Core Git session lifecycle now uses operation-result units across creation, checkpoint, sync,
  publish, volume cleanup, and worker leases. These events are mirrored both to normal
  operation-event sinks and to the runtime-owned session `events.jsonl` file.
- Session creation units now include `session.create.preflight`, `session.create.workspace`,
  `session.create.branch`, and `session.create.metadata`.
- Session checkpoint/sync/publish units now include `session.checkpoint.status`,
  `session.checkpoint.commit`, `session.checkpoint.metadata`, `session.sync.fetch`,
  `session.sync.merge`, `session.publish.preflight`, and `session.publish.push`.
- Session archive now emits `session.archive.metadata`.
- Worker lease units now include `worker.lease.preflight`, `worker.lease.metadata`,
  `worker.lease.volume`, `worker.lease.branch`, `worker.lease.apply`,
  `worker.lease.release.metadata`, and `worker.lease.cleanup`.
- DorkPipe orchestration scripts do not reference core internals directly, but the main package
  shell path now delegates operation-result emission through `dockpipe result` from the shared
  helpers in `orchestrate-common.sh`, with a text fallback for older binaries.
- DorkPipe dev-stack logged operations now use the same `dockpipe result` adapter through
  `dev-stack-lib.sh`, while keeping the existing spinner/log-tail UX.
- DorkPipe optimizer actions now emit canonical `orchestrate.optimize` start/done/fail results with
  duration, target workflow, result artifact path, and action status; the package test suite covers
  the cheap single-pass optimizer path and Windows absolute artifact paths.
- Session lifecycle tests now cover successful and failed operation-result events for create,
  checkpoint, sync, publish, cleanup, and worker lease paths.
- `dockpipe package compile` now emits canonical `package.compile.*` operation-result units for the
  workflow, core, resolver, full-store, workflow-batch, and dependency-closure paths, including
  compiled-vs-skip/noop outcomes, output paths, batch counts, and stale-prune summaries instead of
  relying on one-off `compiled ...`, `skip ...`, or `compile all:` status lines.
- Compile internals now emit `package.compile.hook` units for staged `compile_hooks` execution and
  `package.compile.source_build` units for core `build.source.script` execution instead of bespoke
  `compile_hooks[...]` and `core source build:` lines.
- Compile-closure discovery now emits `package.compile.dependency` skip units for missing `inject`,
  `depends`, and nested delegate workflow references instead of bespoke `compile for-workflow:
  warning: ... skip` lines.
- `dockpipe package build core` and `dockpipe package build store` now emit canonical
  `package.build.core` and `package.build.store` units with version, output/manifest paths, slice
  selection, tarball counts, and built results instead of bespoke `[dockpipe] wrote ...` summaries.
- `dockpipe install core` now emits canonical `install.core`, `install.core.resolve`,
  `install.core.download`, and `install.core.extract` units across project/global install flows with
  source URLs, destination paths, mode/version metadata, checksum status, and installed results
  instead of bespoke install/checksum summary lines.
- `dockpipe package test` and `dockpipe workflow test` now emit canonical batch and per-target
  `package.test.*` / `workflow.test.*` units, including noop outcomes for unmatched selectors,
  target names, scripts, counts, and durations instead of bespoke `package test:` /
  `workflow test:` status lines.
- `dockpipe install core --dry-run` now emits a canonical `install.core.dry_run` unit while
  preserving the descriptive plan text for fetch/manifest/extract behavior.
- `dockpipe run` now emits canonical informational `run.*` units for shared runtime/bootstrap
  context including worktree-base inference, prepared workspace sessions, resolved runtime/resolver
  profiles, host-isolate selection, `/work` mount context, and container branch/detached context
  instead of one-off status lines in those paths.
- Docker runtime startup and recovery paths now emit canonical `run.container.start`,
  `run.container.detach`, `run.container.hint`, and `run.host_commit` units for attached/detached
  container launch, post-launch detach metadata, structured Docker mount/daemon hints, and
  host-checkpoint commit/skip behavior instead of bespoke startup, detach, mount-warning, and
  skip-commit lines.
- Compile-root config warnings now emit canonical `config.compile_path` skip units from the
  application layer, and the `domain` compile-root resolver now returns missing-path metadata
  instead of printing raw config warning lines directly.

## Still Open

- Expand the core operation-result contract into remaining runtime actions outside the current
  covered session/build/DorkPipe-shell paths and other long-running runtime/bootstrap work that still
  prints one-off lines.
- Continue rolling the same result contract deeper into compile/package subcommands that still emit
  ad hoc detail lines beneath the now-normalized build/package units, especially validation and
  install/bootstrap detail lines that still print bespoke text.
- Continue migrating any newly added package-owned scripts that do meaningful long-running work onto
  `dockpipe result` or a shared package wrapper instead of adding new bespoke status formats.
- Add a rebuildable Postgres projection over operation-event JSONL and JSON/YAML indexes for
  PipeDeck, dashboards, search, and cross-run history.
- Push structured event usage further so session metadata, orchestration artifacts, host-action
  approval requests, and future machine-readable output depend on the shared result contract instead
  of handwritten event shapes.
