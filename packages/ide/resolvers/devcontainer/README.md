# Native Dev Container resolver (read-only slice)

`devcontainer` is a package-owned resolver for one intentionally narrow capability:
filesystem-only discovery and fixture-adapted status of a selected repository Dev Container
definition. It is not a Docker lifecycle manager, runtime, provider-pool worker, or editor
attachment flow.

## CLI and MCP entrypoint

The package workflow is the sole execution path:

```bash
dockpipe --workflow devcontainer --package ide --workdir . -- discover --workspace .
dockpipe --workflow devcontainer --package ide --workdir . -- status --workspace . \
  --definition-ref .devcontainer/devcontainer.json \
  --read-configuration-fixture fixtures/read-configuration.json \
  --docker-inspect-fixture fixtures/docker-inspect.json
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
| `event` | `discovered`, `selection_required`, `status`, `completed`, or `failed` in this slice. |
| `workspace_ref`, `definition_ref`, `definition_fingerprint` | Safe workspace-relative identity; no resolved configuration is emitted. |
| `operation` | `discover` or `status`. |
| `state` | `unavailable`, `selection_required`, `available`, `not_created`, `created`, `running`, `stopped`, or `ambiguous`. |
| `ownership` | `external`, `managed`, `orphan_candidate`, or `ambiguous`. |
| `environment_ref` | Opaque, deterministic reference only when one container is known. |
| `summary`, `log_ref`, `next_actions` | Safe renderable status; `log_ref` is absent in this read-only slice. |

Discovery scans only the workspace root for `.devcontainer/devcontainer.json`, `.devcontainer.json`,
and direct `.devcontainer/*.json` or `*.jsonc` files. Candidate ordering is lexical by
workspace-relative reference. A multi-definition workspace emits `discovered`,
`selection_required`, then `failed` and exits non-zero; no selection is guessed. `status` always
requires an explicit workspace-relative `--definition-ref`.

`status` accepts captured JSON adapter fixtures only:

- `--read-configuration-fixture` — JSON object with `definition_ref` (or `config_file`) matching
  the selected reference.
- `--docker-inspect-fixture` — array of Docker inspect objects, or an object with `containers`.
  Matching uses `devcontainer.local_folder` plus `devcontainer.config_file` labels only.
- `--managed-session-fixture` (optional) — `{ "sessions": [...] }`; a managed record must bind
  `container_id`, `session_id`, `workspace_ref`, `definition_ref`, and
  `definition_fingerprint`, and Docker must carry `com.dockpipe.devcontainer.session`.

No fixture means no status adapter: the operation fails closed with `state: unavailable`. The
resolver intentionally contains no subprocess call to Docker or `devcontainer`; adapter
distribution and the policy for live external-container `exec` remain product decisions.

`external` is read-only even if it has Dev Container labels. A matching session label without an
exact session record is `orphan_candidate`; an identity mismatch or multiple containers is
`ambiguous`. This slice never labels, adopts, stops, rebuilds, removes, starts, or otherwise
changes a container.
