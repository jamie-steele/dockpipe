# Git Runtime Sessions

Read when changing Git lifecycle, workspace ownership, autonomous edit sessions, checkpointing,
managed workspaces, bind-mount policy, or recovery/publish behavior.

## Canonical Design

Use `docs/git-runtime-sessions.md` as the source of truth for the long-term architecture.
Use `docs/operation-results.md` for the broader unit-of-work result/logging pattern that runtime
session work should follow.

## Hard Rules

- AI agents must not run arbitrary Git commands for session lifecycle.
- Git operations are runtime lifecycle actions, not resolver or package-script behavior.
- Every autonomous edit session should run on a runtime-owned session branch.
- Checkpoint commits are recovery/collaboration artifacts, not final human-approved history.
- Human review remains the final authority before merge.
- Workflow authors describe workspace intent; runtimes decide clone, worktree, volume, bind mount,
  branch, checkpoint, and cleanup mechanics.
- For `workspace.storage: volume`, treat `/work` as the worker editing surface and keep Git
  lifecycle actions in runtime-owned helper tools.
- Provider/auth mount rules live in `docs/agents/git-runtime-auth.md`.

## Routing

| Work type | Read with |
| --- | --- |
| Runtime/session primitives | `docs/agents/engine-boundary.md`, `docs/agents/architecture.md` |
| Provider detection or auth mounts | `docs/agents/git-runtime-auth.md`, `docs/agents/engine-boundary.md` |
| Workspace storage or path movement | `docs/agents/path-scopes.md`, `docs/agents/core-package-model.md` |
| Workflow YAML fields | `docs/agents/yaml-workflows.md`, `src/lib/infrastructure/schema/workflow.schema.json` |
| Agentic orchestration behavior | `docs/agents/model-escalation.md`, `docs/agents/docs-generation.md` |
| Generated state, recovery logs, MCP/session metadata | `docs/agents/artifacts-and-mcp.md` |

## Implementation Guidance

- Keep the public model provider-neutral: `workspace_id`, `session_id`, worker leases, lifecycle
  requests, and metadata events.
- Keep branch/session logic in runtime infrastructure or a runtime-owned service boundary.
- Do not add DorkPipe-specific session behavior to generic engine code unless it is a reusable
  runtime primitive.
- Do not require workflows to know host paths, Docker volume names, Git commands, or worktree paths.
- Prefer one unit-of-work result/logging contract for runtime actions such as session create, volume
  seed/sync, worker lease, checkpoint, sync, and publish; do not add more ad hoc status strings per
  code path.
- Prefer `workspace.mode: managed` as the future default; keep `workspace.mode: bind` explicit.
- Prefer `workspace.storage: volume` for container-facing managed sessions; reserve
  `workspace.storage: worktree` for local debugging and inspection when needed.
- Start with serialized write leases before introducing worker branches or parallel worktrees.
- Current lifecycle primitives are `CreateSessionBranch`, `CheckpointSession`, `SyncSession`,
  `PublishSession`, `ArchiveSession`, `CreateWorkerLease`, and `ReleaseWorkerLease`.
- Public local CLI commands are `dockpipe session list`, `dockpipe session inspect <id>`,
  `dockpipe session switch <id>`, and `dockpipe session publish <id>`.
- `dockpipe session switch` should hand the human to the managed worktree; it cannot mutate the
  parent shell's current directory.
- `dockpipe session publish` should checkpoint first, then push the session branch. It must not
  merge into the user's current branch.
- `workspace.storage: volume` should mount a runtime-owned volume at `/work`.
- AI workers edit files in `/work` only. They do not clone, branch, checkpoint, or publish.
- Codex/Claude worker containers should receive a runtime-staged skills directory that preserves
  the user's provider skills and overlays curated DorkPipe skills for deterministic script/task
  routing guidance.
- Non-AI runtime helper tools may clone/fetch/checkout/checkpoint/publish against the volume
  workspace as part of DockPipe runtime behavior.
- Keep session metadata and audit logs under `bin/.dockpipe/sessions/...`.

## Validation

- For docs-only routing changes: `git diff --check`.
- For workflow surface changes: update schema, docs, language support, and run workflow validation.
- For runtime/session code: run `go test ./src/lib/...` and `make build`.
