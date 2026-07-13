# Native Dev Container resolver (managed-up contract slice)

`devcontainer` is a package-owned resolver for one intentionally narrow capability:
filesystem-only discovery, fixture-adapted status, and a fixture-proven explicitly approved
managed `up` contract for a selected repository Dev Container definition. It is not a generic
Docker lifecycle manager, runtime, provider-pool worker, external-container executor, or editor
attachment flow.

## CLI and MCP entrypoint

The package workflow is the sole execution path:

```bash
dockpipe --workflow devcontainer --package ide --workdir . -- discover --workspace .
dockpipe --workflow devcontainer --package ide --workdir . -- status --workspace . \
  --definition-ref .devcontainer/devcontainer.json \
  --read-configuration-fixture fixtures/read-configuration.json \
  --docker-inspect-fixture fixtures/docker-inspect.json
dockpipe --workflow devcontainer --package ide --workdir . -- up --workspace . \
  --definition-ref .devcontainer/devcontainer.json --request-id request-123 \
  --approval-fixture fixtures/up-approval.json \
  --up-result-fixture fixtures/up-result.json \
  --managed-session-output artifacts/devcontainer-session.json
```

DockPipe passes the arguments after `--` to this resolver through `DOCKPIPE_ARGS_JSON`; direct
invocation of `assets/scripts/devcontainer-lifecycle.sh` accepts the same normal argv. The
DorkPipe host MCP bridge exposes the exact same package output through its existing generic,
tiered `dockpipe.run` tool (exec tier):

```json
{
  "workflow": "devcontainer",
  "package": "ide",
  "workdir": ".",
  "result_mode": "stdout",
  "argv": ["discover", "--workspace", "."]
}
```

Use `result_mode: "stdout"` so the MCP content is the unchanged NDJSON event stream rather than
the bridge's human-oriented CLI summary wrapper.

Pipeon must consume the recorded NDJSON events from that bridge. It must not invoke Docker or the
Dev Container CLI, and this resolver contains no Pipeon controls.

## `devcontainer.lifecycle.v1` event contract

Every stdout line is one JSON event with these common fields:

| Field | Meaning |
| --- | --- |
| `contract_version` | Always `devcontainer.lifecycle.v1`. |
| `request_id`, `sequence` | Per-operation correlation and strictly increasing event order. |
| `event` | `discovered`, `selection_required`, `status`, `up_requested`, `approval_required`, `up_result`, `completed`, or `failed`. |
| `workspace_ref`, `definition_ref`, `definition_fingerprint` | Safe workspace-relative identity; no resolved configuration is emitted. |
| `operation` | `discover`, `status`, or `up`. |
| `state` | `unavailable`, `selection_required`, `available`, `not_created`, `created`, `running`, `stopped`, `ambiguous`, `approval_required`, or `failed`. |
| `ownership` | `external`, `managed`, `orphan_candidate`, or `ambiguous`. |
| `environment_ref` | Opaque, deterministic reference only when one container is known. |
| `approval_id`, `session_id`, `session_label` | Present only for the approved managed-up result; no raw container id is emitted. |
| `summary`, `log_ref`, `next_actions` | Safe renderable status; `log_ref` is absent in this read-only slice. |

Discovery scans only the workspace root for `.devcontainer/devcontainer.json`, `.devcontainer.json`,
and direct `.devcontainer/*.json` or `*.jsonc` files. Candidate ordering is lexical by
workspace-relative reference. A multi-definition workspace emits `discovered`,
`selection_required`, then `failed` and exits non-zero; no selection is guessed. `status` always
requires an explicit workspace-relative `--definition-ref`.

`status` accepts captured JSON adapter fixtures by default:

- `--read-configuration-fixture` — JSON object with `definition_ref` (or `config_file`) matching
  the selected reference.
- `--docker-inspect-fixture` — array of Docker inspect objects, or an object with `containers`.
  Matching uses `devcontainer.local_folder` plus `devcontainer.config_file` labels only.
- `--managed-session-fixture` (optional) — `{ "sessions": [...] }`; a managed record must bind
  `container_id`, `session_id`, `workspace_ref`, `definition_ref`, and
  `definition_fingerprint`, and Docker must carry `com.dockpipe.devcontainer.session`.

For an explicit, read-only installed-adapter verification, replace
`--read-configuration-fixture` with `--live-read-configuration`. The resolver checks that the
installed `@devcontainers/cli` is exactly the package pin (`0.87.0`), then invokes only
`read-configuration --workspace-folder <workspace> --config <selected-definition>`. Docker
inspection remains a captured fixture in this slice. On Windows, the npm command shim is resolved
to its installed JavaScript entry point and run through the current Node executable without a shell.
An absent, unpinned, timed-out, malformed, or mismatched adapter fails closed without exposing raw
paths, command text, or adapter output. Dev Container CLI `0.87.0` itself performs a
label-filtered, read-only `docker ps` while reading configuration; that indirect query is the sole
Docker exception for this slice. The package does not directly invoke Docker or broaden this into a
Docker preflight.

`up` requires an explicit workspace-relative `--definition-ref`. It first emits `up_requested`.
Without an approval fixture bound to the request id, workspace, selected reference/fingerprint,
and every possible lifecycle risk (`image_pull`, `build`, `compose_create_start`, `feature_install`, and
`lifecycle_hooks`), it emits `approval_required`, then fails before loading any up adapter fixture
or writing a record.

An approved `up` currently accepts only `--up-result-fixture`. That captured result must prove the
chosen **installed/pinned Dev Container CLI** adapter by supplying equal installed and pinned
versions, and bind the selected workspace/reference/fingerprint, container id, session id, and
`com.dockpipe.devcontainer.session` label. The resolver writes the resulting record only to an
explicit workspace-relative `--managed-session-output` path. The persisted record contains the
raw container id for later exact reconciliation; the event stream exposes only its opaque
`environment_ref` plus the session identity and label name.

Without either a read-configuration fixture or explicit live-read flag, `status` fails closed with
`state: unavailable`. The resolver never directly invokes Docker or a Dev Container lifecycle command.
External/user-started containers remain status-only under the selected managed-only policy.

`external` is read-only even if it has Dev Container labels. A matching session label without an
exact session record is `orphan_candidate`; an identity mismatch or multiple containers is
`ambiguous`. This slice never labels, adopts, stops, rebuilds, removes, starts, or otherwise
changes a container.
