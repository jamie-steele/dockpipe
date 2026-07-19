# backlog.remote

`backlog.remote` is the offline first slice of TASK-015. It resolves exactly one explicit entry from
`docs/agents/task-index.yaml`, validates its exact linked task document and a one-line bounded slice,
compiles reviewable immutable request artifacts, and records fixture dispatch identity. It never
invokes Codex Cloud, polls status, retrieves a diff, applies work, commits, pushes, or publishes.

Run it from the consumer repository root with every authority-bearing input explicit:

```bash
dockpipe --package dorkpipe --workflow backlog.remote --workdir . \
  --var DORKPIPE_BACKLOG_TASK_ID=TASK-015 \
  --var 'DORKPIPE_BACKLOG_SLICE=Implement only the offline inspect, compile, and fixture dispatch proof.' \
  --var DORKPIPE_BACKLOG_BASELINE=0123456789abcdef0123456789abcdef01234567 \
  --var DORKPIPE_BACKLOG_ENVIRONMENT_REF=codex-environment-id \
  --var DORKPIPE_BACKLOG_BRANCH_REF=js/dev \
  --var 'DORKPIPE_BACKLOG_ALLOWED_PATHS_JSON=["packages/dorkpipe","docs/agents/tasks/backlog-driven-remote-tasks.md"]' \
  --var 'DORKPIPE_BACKLOG_HARD_BOUNDARIES_JSON=["No src/lib or src/cmd changes","No live provider invocation"]' \
  --var 'DORKPIPE_BACKLOG_REQUIRED_VALIDATION_JSON=["go test ./packages/dorkpipe/lib/orchestrationhelper"]' \
  --var 'DORKPIPE_BACKLOG_ROUTED_SOURCES_JSON=["docs/agents/packages/package-authoring.md","docs/agents/workflows/yaml-workflows.md"]' --
```

The workflow writes under the normal `backlog-remote` artifact scope:

- `backlog-selection.json` records the exact open task, linked path, bounded slice, baseline, and
  source digests. A rejected inspection writes the same contract with a deterministic rejection code.
- `remote-request.json` and `remote-request.md` bind the explicit target, allowed paths, hard
  boundaries, validation, and exact source file digests under one request fingerprint.
- `remote-task.json` records one opaque fixture task ID, that fingerprint, the target references,
  deterministic fixture time, and adapter identity with `provider_invoked: false`.

`orchestrate-helper backlog-followup <artifact-root>` validates and recovers identity using only
`remote-task.json`, `remote-request.json`, and `remote-request.md`. It does not reread the repository.

The canonical backlog has no standardized readiness or ownership fields. Package test fixtures use
an optional `dispatch_state` (`blocked`, `external_active`, or `closed`) only to prove deterministic
rejection. The canonical index is unchanged; a future `--next` selector remains out of scope until
that metadata contract is decided.

The installed Codex CLI currently documents `codex cloud exec --env <id> --branch <branch> [query]`,
but its help does not expose a machine-readable submission receipt or stable task-ID response schema.
The default adapter therefore remains fixture-only instead of parsing undocumented terminal text.
