# backlog.remote

`backlog.remote` is the offline TASK-015 path. It resolves exactly one explicit entry from
`docs/agents/task-index.yaml`, validates its exact linked task document and a one-line bounded slice,
compiles reviewable immutable request artifacts, preflights the Codex Cloud CLI contract from narrow
package-owned help fixtures, records fixture dispatch identity, and ingests one explicitly bound
completion-candidate fixture as untrusted evidence. It never invokes Codex Cloud, polls status,
retrieves a diff or result, validates remote work, applies work, commits, pushes, or publishes.

Run it from the consumer repository root with every authority-bearing input explicit:

```bash
dockpipe --package dorkpipe --workflow backlog.remote --workdir . \
  --var DORKPIPE_BACKLOG_TASK_ID=TASK-015 \
  --var 'DORKPIPE_BACKLOG_SLICE=Implement only the offline fixture dispatch and completion-candidate proof.' \
  --var DORKPIPE_BACKLOG_BASELINE=0123456789abcdef0123456789abcdef01234567 \
  --var DORKPIPE_BACKLOG_ENVIRONMENT_REF=codex-environment-id \
  --var DORKPIPE_BACKLOG_BRANCH_REF=js/dev \
  --var 'DORKPIPE_BACKLOG_ALLOWED_PATHS_JSON=["packages/dorkpipe","docs/agents/tasks/backlog-driven-remote-tasks.md"]' \
  --var 'DORKPIPE_BACKLOG_HARD_BOUNDARIES_JSON=["No src/lib or src/cmd changes","No live provider invocation"]' \
  --var 'DORKPIPE_BACKLOG_REQUIRED_VALIDATION_JSON=["go test ./packages/dorkpipe/lib/orchestrationhelper"]' \
  --var 'DORKPIPE_BACKLOG_ROUTED_SOURCES_JSON=["docs/agents/packages/package-authoring.md","docs/agents/workflows/yaml-workflows.md"]' \
  --var DORKPIPE_BACKLOG_COMPLETION_FIXTURE=/reviewed/path/completion-candidate.json --
```

The workflow writes under the normal `backlog-remote` artifact scope:

- `backlog-selection.json` records the exact open task, linked path, bounded slice, baseline, and
  source digests. A rejected inspection writes the same contract with a deterministic rejection code.
- `remote-request.json` and `remote-request.md` bind the explicit target, allowed paths, hard
  boundaries, validation, and exact source file digests under one request fingerprint.
- `remote-adapter-compatibility.json` binds the inspected adapter/CLI contract to that request
  fingerprint and the explicit environment/branch refs. It records required commands, documented
  inputs, receipt/task-ID support, the compatibility status and exact fail-closed reason, enabled
  adapter modes, and whether live submission is enabled.
- `remote-task.json` records one opaque fixture task ID, that fingerprint, the target references,
  deterministic fixture time, compatibility fingerprint, and adapter identity with
  `provider_invoked: false`.
- `completion-candidate.json` records one candidate/replay identity, exact task/request/dispatch/
  adapter/environment/branch binding, deterministic observation time, and an untrusted terminal
  claim. Its only authoritative state is `completion_candidate`; every review, retrieval,
  validation, apply, commit, push, and publication transition remains false.

`orchestrate-helper backlog-followup <artifact-root>` validates and recovers identity using only the
immutable request, compatibility, and dispatch artifacts. Completion ingestion uses those same
artifacts and never rereads the repository. A candidate observed at or before dispatch is stale;
wrong bindings, duplicate candidate IDs, replayed replay IDs, malformed fixtures, and tampered
immutable artifacts fail before `completion-candidate.json` is written. Once accepted, later
duplicate or replay rejection leaves both the accepted candidate and dispatch bytes unchanged.

The canonical backlog has no standardized readiness or ownership fields. Package test fixtures use
an optional `dispatch_state` (`blocked`, `external_active`, or `closed`) only to prove deterministic
rejection. The canonical index is unchanged; a future `--next` selector remains out of scope until
that metadata contract is decided.

The checked package fixture records the exact read-only inspection of `codex-cli 0.144.1` through
`codex --version`, `codex cloud --help`, and `codex cloud exec --help`. The documented submit surface
is `codex cloud exec --env <ENV_ID> [--branch <BRANCH>] [QUERY]`; it exposes no machine-readable
submission receipt or stable opaque task-ID response contract. Compatibility is therefore
`unsupported`, live submission remains disabled, and fixture dispatch remains the only enabled
adapter. The preflight never parses submission terminal text, credentials, authentication state, or
environment listings. A malformed compatibility contract fails before fixture dispatch and leaves
no `remote-task.json`. Completion fixtures are package-test evidence, not undocumented provider
responses or callback schemas.
