# backlog.remote

`backlog.remote` is the offline TASK-015 path. It resolves exactly one explicit entry from
`docs/agents/task-index.yaml`, validates its exact linked task document and a one-line bounded slice,
compiles reviewable immutable request artifacts, preflights the Codex Cloud CLI contract from narrow
package-owned help fixtures, records fixture dispatch identity, and ingests one explicitly bound
completion-candidate fixture plus later fixture-backed status and diff observations as untrusted
evidence. It never invokes Codex Cloud, live-polls status or diff, retrieves a result, validates or
applies remote work, commits, pushes, or publishes.

Run it from the consumer repository root with every authority-bearing input explicit:

```bash
dockpipe --package dorkpipe --workflow backlog.remote --workdir . \
  --var DORKPIPE_BACKLOG_TASK_ID=TASK-015 \
  --var 'DORKPIPE_BACKLOG_SLICE=Implement only the offline completion-candidate, status, and diff-evidence proof.' \
  --var DORKPIPE_BACKLOG_BASELINE=0123456789abcdef0123456789abcdef01234567 \
  --var DORKPIPE_BACKLOG_ENVIRONMENT_REF=codex-environment-id \
  --var DORKPIPE_BACKLOG_BRANCH_REF=js/dev \
  --var 'DORKPIPE_BACKLOG_ALLOWED_PATHS_JSON=["packages/dorkpipe","docs/agents/tasks/backlog-driven-remote-tasks.md"]' \
  --var 'DORKPIPE_BACKLOG_HARD_BOUNDARIES_JSON=["No src/lib or src/cmd changes","No live provider invocation"]' \
  --var 'DORKPIPE_BACKLOG_REQUIRED_VALIDATION_JSON=["go test ./packages/dorkpipe/lib/orchestrationhelper"]' \
  --var 'DORKPIPE_BACKLOG_ROUTED_SOURCES_JSON=["docs/agents/packages/package-authoring.md","docs/agents/workflows/yaml-workflows.md"]' \
  --var DORKPIPE_BACKLOG_COMPLETION_FIXTURE=/reviewed/path/completion-candidate.json \
  --var DORKPIPE_BACKLOG_STATUS_FIXTURE=/reviewed/path/remote-status.json \
  --var DORKPIPE_BACKLOG_DIFF_FIXTURE=/reviewed/path/remote-diff.json --
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
- `remote-status.json` records one status observation/replay identity bound to the full accepted
  candidate fingerprint and candidate identity plus the immutable task/request/dispatch/adapter/
  environment/branch identity. Its canonical observation time must be later than both dispatch and
  candidate observation times. The fixture's `completed` status is explicitly untrusted and
  non-authoritative; the artifact remains at `state: completion_candidate`, with review, diff/result
  retrieval, validation, apply, commit, push, and publication false.
- `remote-diff.json` records one diff observation/replay identity bound to the canonical accepted
  status and candidate fingerprints plus the immutable task/request/dispatch/adapter/environment/
  branch identity. Its observation time is later than dispatch, candidate, and status times. It
  records the exact patch SHA-256 and byte count, package-owned fixture provenance, and only
  `state: completion_candidate`; review, result retrieval, semantic and allowed-path verification,
  validation, apply, commit, push, and publication remain false.
- `remote-diff.patch` contains the exact adjacent fixture patch bytes. They are opaque and untrusted:
  retrieval checks only the declared SHA-256 and does not parse paths, infer authorization, apply the
  patch, or infer lifecycle completion.

`orchestrate-helper backlog-followup <artifact-root>` validates and recovers identity using only the
immutable request, compatibility, and dispatch artifacts. Completion ingestion uses those same
artifacts and never rereads the repository. A candidate observed at or before dispatch is stale;
wrong bindings, duplicate candidate IDs, replayed replay IDs, malformed fixtures, and tampered
immutable artifacts fail before `completion-candidate.json` is written. Once accepted, later
duplicate or replay rejection leaves both the accepted candidate and dispatch bytes unchanged.

Status retrieval revalidates the immutable request, compatibility, dispatch, and the complete
accepted candidate artifact without rereading the repository. An observation at or before the
candidate time is stale. Wrong candidate/task/request/dispatch/adapter/target bindings, duplicate
observation IDs, replayed replay IDs, malformed fixtures, tampered evidence, and tampered immutable
artifacts fail before `remote-status.json` is written. Rejection cannot create review, diff, result,
validation, or apply artifacts and cannot alter the accepted candidate or dispatch identity.

Diff retrieval revalidates the immutable request, compatibility, dispatch, complete candidate, and
complete status artifacts without rereading the repository. An observation at or before status is
stale. Wrong status/candidate/task/request/dispatch/adapter/target bindings, duplicate observation
IDs, replayed replay IDs, malformed or missing metadata, missing patch bytes, checksum-tampered patch
bytes, and tampered immutable artifacts fail before either accepted diff artifact is written. The
two outputs use temporary files and a rollback-on-rename-failure pair write. Clean-chain rejection
cannot create review, result, validation, or apply artifacts; duplicate or replay rejection cannot
change the accepted diff, status, candidate, or dispatch bytes.

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
no `remote-task.json`. Completion, status, and diff fixtures are package-test evidence, not
undocumented provider responses or callback schemas.
