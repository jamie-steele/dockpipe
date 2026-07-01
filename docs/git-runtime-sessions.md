# Git Runtime Sessions

This proposal defines a Git-owned session lifecycle for long-running autonomous and semi-autonomous
DockPipe runs. It is a foundation document, not the full implementation plan.

## Problem

Today, workflows and agents can drift toward direct repository mutation:

- workers can see host paths and mounted checkouts
- package scripts can call `git` directly
- checkpoint behavior is ad hoc
- crashes can leave partial edits in the user's checkout
- distributed workers would need to reinvent clone, branch, sync, and cleanup behavior

That model does not scale to long-running sessions, remote execution, multiple workers, or safe
human review.

## Principles

- AI agents do not execute arbitrary Git commands.
- Git operations are runtime lifecycle actions owned by DockPipe or DorkPipe's orchestration runtime.
- Every autonomous edit session runs on a session branch.
- Checkpoint commits are recovery and collaboration artifacts, not human-approved history.
- Human review remains the final authority before merge.
- Workflow authors describe workspace intent; the runtime decides bind mount, managed volume, clone,
  worktree, and branch mechanics.
- The public API uses workspace and session identities, not host paths, Docker volume names, or raw
  Git commands.

## Architecture

DockPipe keeps the existing architecture split:

- workflow: what should happen
- runtime: where execution happens, including workspace lifecycle
- resolver: which tool performs work
- strategy: optional before/after wrapper

The Git session lifecycle belongs to runtime infrastructure, not resolver profiles. Codex, Claude,
Ollama, editors, and future workers can request lifecycle transitions through a controlled API, but
they do not run `git commit`, `git pull`, `git push`, or `git checkout`.

```text
workflow intent
  -> runtime creates/opens workspace
  -> runtime creates session branch
  -> workers attach to session workspace
  -> workers produce edits/artifacts
  -> runtime checkpoints/syncs/publishes
  -> human reviews final branch/PR
```

## Public Interface

The public interface should be provider-neutral. The local CLI now exposes the human-facing review
loop:

```bash
dockpipe session list
dockpipe session inspect <id|latest>
dockpipe session switch <id|latest>
dockpipe session publish <id|latest>
```

`switch` prints the managed worktree path and a shell `cd` command because a child process cannot
change the caller's current directory. `publish` creates a pre-publish checkpoint commit when the
session worktree is dirty, then pushes the session branch to the selected remote. It does not merge
or rewrite the user's current branch.

Names below are conceptual Go/service operations behind that shape.

```go
type GitRuntime interface {
    CreateSession(ctx context.Context, req CreateSessionRequest) (Session, error)
    AttachWorker(ctx context.Context, req AttachWorkerRequest) (WorkerLease, error)
    CheckpointSession(ctx context.Context, req CheckpointRequest) (Checkpoint, error)
    SyncSession(ctx context.Context, req SyncRequest) (SyncResult, error)
    SyncWorker(ctx context.Context, req SyncWorkerRequest) (SyncResult, error)
    PublishSession(ctx context.Context, req PublishRequest) (PublishedSession, error)
    ArchiveSession(ctx context.Context, req ArchiveRequest) (Archive, error)
    RecoverSession(ctx context.Context, req RecoverRequest) (Session, error)
    InspectSession(ctx context.Context, req InspectSessionRequest) (SessionStatus, error)
}
```

Required properties:

- operations are idempotent where possible
- every operation records metadata
- workers receive a workspace identity and mount/connection descriptor, not raw Git authority
- implementation may use Git CLI internally, libgit2 later, or remote provider APIs where useful
- operation results include enough data for audit and recovery without parsing terminal logs

Agents should request lifecycle actions using structured events:

```json
{
  "type": "checkpoint.requested",
  "session_id": "ai/run-1842-feature-comments",
  "worker_id": "editor-a",
  "reason": "completed comment form validation changes"
}
```

They should not emit shell snippets like `git commit -am ...`.

## Workflow Surface

Workflow authors should describe workspace intent without selecting storage mechanics:

```yaml
workspace:
  repo: biztraak
  mode: managed
  lifecycle:
    branch_prefix: ai
    branch: js/features/spnext/reporting/worktree-report-poc
    checkpoint: auto
    publish: review
```

Local development can opt into direct bind behavior:

```yaml
workspace:
  repo: ./local/path
  mode: bind
  lifecycle:
    branch_prefix: ai
    checkpoint: manual
```

Proposed fields:

| Field | Meaning |
| --- | --- |
| `workspace.repo` | Logical repository identity or source path/URL. |
| `workspace.mode` | `managed` by default; `bind` for explicit local fast path. |
| `workspace.lifecycle.branch_prefix` | Prefix for session branches, default `ai`. |
| `workspace.lifecycle.branch` | Exact session branch name for repos with required branch conventions; overrides branch-prefix derivation. |
| `workspace.lifecycle.checkpoint` | `manual`, `auto`, or `step`. |
| `workspace.lifecycle.publish` | `none`, `branch`, or `review`. |
| `workspace.base` | Optional base branch/ref, default current/default branch. |
| `workspace.storage` | Optional advanced override, e.g. `volume`, `worktree`, `clone`; not needed for most workflows. |

The initial runtime service now supports this authored surface for local sessions. The first
implementation uses managed Git worktrees for `mode: managed` and current-checkout branch
management for explicit `mode: bind`; later implementations can move the same public API to named
Docker volumes, isolated clones, or distributed workers.

## Session Lifecycle

```text
requested
  -> preparing_workspace
  -> branching
  -> active
  -> checkpointing
  -> active
  -> syncing
  -> active
  -> publishing
  -> published
  -> archived
```

Failure substates:

```text
active -> interrupted -> recovering -> active
active -> conflict -> waiting_for_resolution -> active
active -> failed -> archived
publishing -> publish_failed -> active|archived
```

State meanings:

| State | Runtime behavior |
| --- | --- |
| `requested` | Session record exists; no workspace is usable yet. |
| `preparing_workspace` | Runtime creates/opens clone, worktree, bind workspace, or volume. |
| `branching` | Runtime creates or verifies the session branch. |
| `active` | Workers may attach and mutate the session workspace. |
| `checkpointing` | Runtime serializes changes into a recovery commit or stash-like object. |
| `syncing` | Runtime updates from the base/session source and handles conflicts. |
| `conflict` | Runtime blocks unsafe progress until resolution policy succeeds or human intervenes. |
| `publishing` | Runtime pushes branch or opens review artifact/PR. |
| `published` | Review target exists; no automatic merge has occurred. |
| `archived` | Runtime has closed the session and retained metadata. |

## Metadata Model

Runtime metadata should live under project-local state for local runs and under a session service for
distributed runs. A local starting point:

```text
bin/.dockpipe/sessions/<session-id>/
  session.json
  events.jsonl
  workers/<worker-id>.json
  checkpoints/<checkpoint-id>.json
  recovery/
  logs/
```

`session.json`:

```json
{
  "schema": 1,
  "session_id": "ai/run-1842-feature-comments",
  "workspace_id": "biztraak",
  "repo": {
    "logical_id": "biztraak",
    "source": "git@github.com:org/biztraak.git",
    "base_ref": "main",
    "session_ref": "ai/session-2026-07-01-feature-comments"
  },
  "storage": {
    "mode": "managed",
    "backend": "docker_volume",
    "volume": "dockpipe-ws-biztraak-run-1842"
  },
  "status": "active",
  "created_at": "2026-07-01T00:00:00Z",
  "updated_at": "2026-07-01T00:30:00Z",
  "policy": {
    "checkpoint": "auto",
    "publish": "review",
    "allow_agent_git": false
  }
}
```

`events.jsonl`:

```json
{"ts":"2026-07-01T00:01:00Z","type":"session.created","actor":"runtime","session_id":"ai/run-1842-feature-comments"}
{"ts":"2026-07-01T00:10:00Z","type":"checkpoint.created","actor":"runtime","checkpoint_id":"cp-0003","commit":"abc1234"}
{"ts":"2026-07-01T00:20:00Z","type":"worker.failed","actor":"runtime","worker_id":"editor-a","reason":"container_exit_137"}
```

Checkpoint metadata:

```json
{
  "schema": 1,
  "checkpoint_id": "cp-0003",
  "session_id": "ai/run-1842-feature-comments",
  "worker_id": "editor-a",
  "commit": "abc1234",
  "parent": "def5678",
  "reason": "worker requested checkpoint after implementation pass",
  "dirty_before": true,
  "status": "created",
  "created_at": "2026-07-01T00:10:00Z"
}
```

## Workspace Storage

Default storage should become managed runtime workspaces, not host bind mounts.

Preferred default:

```text
repo source -> DockPipe managed clone/worktree -> named runtime volume -> session branch -> checkpoints
```

Supported modes:

| Mode | Default? | Role |
| --- | --- | --- |
| `managed` | Yes | Runtime owns clone/worktree, branch, checkpoint, cleanup, and volume. |
| `bind` | No | Explicit local fast path for developer workflows that intentionally operate on a mounted checkout. |

Managed local implementation:

- create a session workspace under `bin/.dockpipe/sessions/<id>/workspace` or a Docker named volume
- prefer Docker volume for container workers once volume clone/bootstrap is implemented
- expose the workspace to workers at a stable guest path, for example `/workspace/<workspace-id>`
- keep `/work` as compatibility only, not the conceptual identity

Bind implementation:

- require explicit `workspace.mode: bind`
- still create/check out a session branch before workers attach
- still run checkpoints through the runtime
- block direct agent Git by policy and prompt text
- warn when dirty host state exists before session start

## Checkpoint Strategy

Checkpoint commits are runtime-owned recovery artifacts.

Recommended defaults:

- create a checkpoint after each successful edit worker
- create a checkpoint before risky sync/publish operations
- create a checkpoint on worker timeout/interruption if there are filesystem changes
- coalesce checkpoints when no tree changes exist
- mark checkpoint commits with machine-readable trailers

Example commit subject:

```text
checkpoint(editor-a): generated validation form edits
```

Example trailers:

```text
DockPipe-Session: ai/run-1842-feature-comments
DockPipe-Checkpoint: cp-0003
DockPipe-Worker: editor-a
DockPipe-Reason: worker-requested
```

These commits are not final product history. Publish can either:

- preserve checkpoint commits for review transparency
- squash them into one review commit
- open a PR with checkpoint commits and recommend squash merge

The default should be preserve for recovery during development, squash at human merge time.

## Recovery Strategy

After container or worker failure:

1. Runtime detects worker lease loss or non-zero exit.
2. Runtime freezes new worker attachment for the affected workspace.
3. Runtime inspects workspace status.
4. If dirty changes exist, runtime creates a recovery checkpoint or recovery patch artifact.
5. Runtime records `worker.failed` and `checkpoint.created` or `recovery.patch.created`.
6. Runtime can restart the worker from the latest checkpoint if policy allows.
7. If conflict or corrupt workspace state is detected, runtime marks the session `conflict` or
   `failed` and requires human or higher-level orchestrator resolution.

For managed volumes, recovery should not depend on the failed container. A new runtime process can:

- read `session.json`
- reattach the Docker volume or session workspace path
- inspect the session branch
- resume from the latest checkpoint
- archive orphaned worker leases

For bind mode, recovery is weaker because dirty changes are in the host checkout. Runtime should
checkpoint immediately where possible and warn if manual cleanup is needed.

## Conflict Handling

The first implementation should avoid automatic conflict resolution beyond safe fast-forward/rebase
cases.

Policy:

- fast-forward sync: automatic
- clean rebase without conflicts: optional automatic
- conflicts: stop workers, record conflict metadata, require explicit resolution
- repeated conflicts: archive or fork a new session branch

AI can propose a conflict resolution patch, but the runtime still owns Git operations.

## Runtime And Workflow Changes

Required core primitives:

- workflow schema: add optional `workspace`
- domain model: parse `workspace` into typed config
- runtime/session service: create, inspect, recover, publish sessions
- state helpers: `SessionRoot`, `WorkspaceRoot`, `SessionEventLog`
- runner: create/open workspace before step execution when `workspace` is configured
- container runner: mount/attach managed workspace by identity, not raw host repo path
- SDK/control API: expose lifecycle requests without exposing raw Git commands
- policy: deny or flag direct Git command execution from agent workers
- docs/language support: document fields and editor validation

Package-level changes:

- DorkPipe orchestration tasks should receive `workspace_id`, `session_id`, and worker lease data.
- DorkPipe prompts should say workers may request checkpoint/sync/publish but must not run Git.
- Apply/publish stages should call runtime operations, not shell `git`.
- Existing artifact-first flows remain valid and can run without workspace sessions.

## Migration Plan

Phase 0: design and guardrails

- Publish this design.
- Add prompt policy that cloud workers must not run Git commands.
- Keep current bind behavior but make direct Git use visible in logs/tests.

Phase 1: local session branch runtime

- Add internal session metadata and `CreateSession`, `CheckpointSession`, `ArchiveSession`.
- Default implementation uses the current checkout with explicit session branch.
- Require clean or intentionally accepted dirty state before session start.
- Checkpoint through runtime only.

Phase 2: managed workspace path

- Add `workspace.mode: managed`.
- Create local managed clone/worktree under DockPipe state.
- Attach containers to that managed workspace instead of the source checkout.
- Keep bind mode opt-in.

Phase 3: Docker volume workspace

- Add named volume bootstrap from managed clone/worktree. Implemented for container runs as
  `workspace.storage: volume`: DockPipe creates a session volume, mounts it at `/work`, syncs the
  managed worktree into it before the run, and overlays changes back after the run for checkpoints.
- Make container workers attach to the volume. Implemented for generic `RunContainer` paths through
  runtime-owned `WorkdirVolume`.
- Store enough metadata to reattach after process crash. Implemented in session metadata:
  `storage.volume`, `storage.workspace`, `storage.metadata`, and event logs.

Phase 4: multi-worker isolation

- Add worker leases. Implemented as runtime metadata under
  `bin/.dockpipe/sessions/<id>/workers/<worker>.json`.
- Initially serialize write workers against one session branch.
- Later support worker worktrees behind the same API. Worker branch creation is available as an
  opt-in lease flag, but it is not the default because one shared checkout cannot safely run
  concurrent branch checkouts.

Phase 5: publish/review

- Add `dockpipe session list|inspect|switch|publish`. Implemented for local session metadata,
  worktree handoff, and checkpoint-before-push branch publishing.
- Add `PublishSession`. Implemented for branch push to a configured Git remote.
- Add `SyncSession`. Implemented as runtime-owned pre-sync checkpoint plus merge from the base ref,
  with conflict status recorded in session metadata.
- Add `ArchiveSession`. Implemented as metadata/event transition; destructive workspace cleanup
  should remain a separate explicit operation.
- Support push branch and optional PR provider adapters as resolver/package capabilities.
- Keep merge outside automatic runtime behavior unless explicitly approved.

Phase 6: distributed runtime

- Move session operations behind a service boundary.
- Use the same public lifecycle API for remote workers, Discord/control panel orchestration, and
  cloud execution.

## Risks And Tradeoffs

| Risk | Mitigation |
| --- | --- |
| Managed workspaces add startup cost. | Cache clones/volumes by workspace id and base ref. |
| Docker volumes are opaque to users. | Provide `dockpipe session inspect`, export, archive, and cleanup commands. |
| Checkpoint commits may clutter history. | Treat them as recovery commits and squash/rebase at publish/review. |
| Multi-worker writes can conflict. | Start with serialized write leases; add worker branches later. |
| Bind mode remains tempting. | Make managed the default and require explicit bind opt-in. |
| Git provider differences can leak upward. | Keep provider push/PR behind publish adapters; core session operations use local Git semantics first. |
| Agents may still try raw Git. | Prompt policy, command filtering where possible, and runtime audit warnings. |
| Dirty host repos can confuse sessions. | Require clean base for managed clone; warn/block for bind mode unless override is explicit. |

## Recommendation

Default implementation should be:

```text
managed clone/worktree + session branch + runtime checkpoint commits
```

Not direct bind mounts.

Initial local implementation can use a managed worktree on the host because it is simpler to debug
than Docker-volume-only storage. The long-term default for container workers should be a named
runtime volume backed by that managed workspace identity.

Branches should be the user-facing review unit:

```text
main -> ai/session-<date>-<slug>
```

Worktrees should be an implementation detail for local managed workspaces. Clones should be an
implementation detail for remote/distributed workers. Worker branches should come later, after the
session API and checkpoint ledger are stable.

In short:

- public default: session branch
- local implementation: managed worktree first
- container implementation: named volume attached to session
- future distributed implementation: isolated clone or worker branch behind the same API

This keeps workflows stable while letting the runtime evolve from local Docker to distributed
orchestration.
