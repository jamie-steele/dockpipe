# TASK-003 Managed Session Volume Cleanup And Retention

## Current State

- Core DockPipe now auto-cleans managed session base volumes after successful workflow completion
  and after `dockpipe session publish` when the session uses `workspace.storage: volume`.
- Cleanup is runtime-owned and gated by preflight checks: managed worktree exists, local session
  branch exists and matches the workspace branch, local session branch metadata still exists, and
  no active worker lease still depends on the volume.
- Cleanup writes `session.volume.cleanup.preflight` and `session.volume.cleanup` operation-result
  lines/session events and records cleaned volume metadata in the session record.
- `DOCKPIPE_SESSION_VOLUME_AUTOCLEANUP=false` disables the default cleanup behavior when a user
  intentionally wants to keep the runtime-owned volume around.

## Still Open

- Add explicit inspect/list/prune CLI behavior for retained or stale managed session volumes and
  related workspace artifacts.
- Broaden cleanup evaluation to more failure paths so runtime-owned cleanup is not limited to the
  current successful workflow completion and publish paths.
