# TASK-014 Native Dev Container Discovery And Lifecycle

## Goal

Let Pipeon recognize a repository-owned `.devcontainer` definition and offer a governed way to
prepare, start, attach to, inspect, and stop that environment. The same lifecycle must be available
through a CLI/MCP contract so Pipeon is a consumer of the capability, not a second Dev Container
runtime.

## Current Context

The package-owned `packages/ide/resolvers/devcontainer` resolver now discovers standard, legacy, and
direct root repository definitions; fails closed on multi-definition selection; reports normalized
status from captured adapters; verifies live `read-configuration` through pinned Dev Container CLI
`0.87.0`; and defines an approved managed `up` contract through fixtures.

Pipeon already owns a separate local stack and provider-pool lifecycle. Native Dev Container support
must compose with that stack without treating the Dev Container as Pipeon's private state or silently
replacing a user's existing container session.

No live `up`, lifecycle hooks, `exec`, attach/editor action, stop/remove, reconciliation, Pipeon
consumer, or provider-pool/runtime integration exists yet. The managed `up` slice proves only the
approval, adapter-result, ownership-record, and event contract.

## Remaining Questions

- What is the bounded cross-platform live `up` adapter and reconciliation contract after the current
  pinned read-only CLI verification and fixture-only managed result?
- How do Docker Compose-based definitions, features, mounts, `remoteUser`, forwarded ports, and
  rebuild requirements map to bounded DockPipe operation-result events?
- How should Pipeon expose readiness, build progress, logs, container identity, attach targets, and
  repair actions while preserving the CLI as the execution authority?
- Can the DorkPipe provider pool safely use a ready Dev Container as a declared execution location,
  or must provider workers and the Dev Container remain separate until an explicit resolver contract
  exists?

## Product Shape

1. Discovery and status are package-owned, read-only, deterministic, and available through the
   package workflow and generic MCP execution path. Finding a definition never starts it, and
   multiple definitions require explicit selection.
2. The managed `up` contract requires explicit intent, risk-bound approval, a pinned adapter result,
   and exact ownership evidence. Live execution, rebuilding, stopping, or attaching remains a future
   governed action using the same operation-result contract.
3. The CLI/MCP surface remains package-owned, provider-neutral, and lifecycle-oriented. Do not add a
   `dockpipe devcontainer` engine subcommand or guess among multiple definitions.
4. Pipeon will consume that contract. The first UX only offers availability, selection, and status;
   `Use Dev Container`, logs, attach/open, rebuild, and stop arrive only with matching CLI/MCP
   operations. It stores only UI selections and drafts locally; the repository's `.devcontainer`
   files remain the durable source of truth.
5. The lifecycle operation returns an opaque environment/session reference plus normalized state and
   artifact/log pointers. It does not expose raw Docker or Dev Container command payloads to other
   app layers.

## Safety And Boundary Rules

- Keep Dev Container-specific resolution, CLI integration, and Docker behavior package/resolver
  owned unless research identifies a genuinely generic DockPipe primitive.
- Never auto-run a discovered configuration. Builds, pulls, feature installation, Compose changes,
  stop/remove, rebuild, and host-editor launch require explicit intent and applicable approval.
- Respect the user's existing containers and labels. Do not stop, remove, or rebuild a container
  not proven to belong to the selected definition and requested DockPipe session.
- Do not copy repository contents into a Pipeon volume when the Dev Container contract already owns
  workspace mounting. Do not infer editor attachment state from unsupported host process heuristics.
- Treat secrets only as existing Dev Container references or governed secret references; never read
  or serialize resolved secret values into Pipeon state, artifacts, or events.
- Keep Pipeon UI, CLI, and MCP on one structured event/approval contract. No extension-only
  lifecycle implementation or durable Pipeon-specific Dev Container configuration.

## First Research Deliverables

- Compatibility matrix for Dev Container CLI, Docker Desktop, Docker Compose, and host editor
  attachment across supported host platforms.
- Inventory of the existing `packages/ide/resolvers/` flows, Pipeon stack lifecycle, and their
  overlap/conflicts with repository-owned `.devcontainer` definitions.
- Proposed normalized lifecycle state machine, operation-result schema, approval classes, ownership
  labels, and cleanup/recovery rules.
- CLI/MCP contract proposal with multi-definition selection and non-interactive fail-closed behavior.
- Pipeon UX wireflow showing discovery, explicit start, progress, attach, error/repair, and teardown.
- A minimal vertical-slice recommendation with tests that use fixture Dev Container definitions and
  no live image pull by default.

## Open Decisions

- The first live `up` adapter and reconciliation design. Read-only configuration currently uses the
  pinned installed CLI; direct Docker remains limited to captured status evidence and must not become
  an alternate lifecycle implementation.
- Whether a started Dev Container becomes an eligible generic workflow runtime/resolver target or is
  initially limited to Pipeon/editor attachment and explicit CLI exec.
- Recovery for a live start that creates resources before its managed record is persisted. The
  decided managed-stop policy retains its record; stop implementation, remove/down, and automatic
  recovery still require separate authorization.
- Which editor attachments are supported first: VS Code, Cursor, Pipeon code-server, or a
  container-only status/exec surface.

## Research Update — First Design Slice (2026-07-13)

### Local Evidence And Precise Gap

| Existing flow | What it owns | Gap to a repository Dev Container |
| --- | --- | --- |
| `packages/ide/resolvers/vscode` | A disposable `dockpipe-base-dev` container mounted at `/work`, then a host VS Code Dev Containers URI. | Does not read `.devcontainer` or use its image, Compose service, features, mounts, lifecycle commands, or `remoteUser`. |
| `packages/ide/resolvers/cursor-dev` | The same DockPipe-authored base image/container and best-effort editor-attachment/idle heuristics. | It is not a native Cursor/Dev Container lifecycle and its attachment heuristics must not be reused as an ownership signal. |
| `packages/pipeon/resolvers/pipeon-dev-stack` | A Pipeon-scoped Compose control plane, code-server container, host MCP bridge, state directory, labels, and teardown. | It is Pipeon's product stack, not the workspace environment. It must neither replace nor be torn down with a discovered Dev Container. |
| DorkPipe provider pools | Bounded, DorkPipe-owned provider workers exposed through CLI/MCP and consumed by Pipeon. | A ready Dev Container is not a provider-pool worker or generic runtime target without a later explicit resolver contract. |

The exact missing capability is therefore read-only discovery and status of a *repository-owned
definition*, followed only by an explicit, governed request to use it. Today no package reads a
selected definition or reports the corresponding container identity/state. Existing Pipeon stack
labels and cleanup apply only to Pipeon resources; existing IDE containers are DockPipe-authored,
disposable compatibility sessions. Neither is evidence that a user's native Dev Container is
DockPipe-owned.

### Upstream CLI And Docker Evidence

Use the reference [`@devcontainers/cli`](https://github.com/devcontainers/cli) as the initial
adapter boundary, pinned and compatibility-tested by the future package. Its documented/sourced
operations are:

| Operation | Use in this design | Machine-readable fact |
| --- | --- | --- |
| `read-configuration` | Read a selected definition before any action. | Structured configuration result; it can resolve the selected file. |
| `up` | Future explicit prepare/start only. | Final JSON includes `outcome`, container id, remote user, and remote workspace folder; `--log-format json` makes progress parseable. |
| `run-user-commands` | Future deliberate lifecycle-hook operation, never an incidental status check. | Final JSON outcome/result; hooks may execute repository-defined commands. |
| `exec` | Future explicit command operation against an already selected/running environment. | Final JSON outcome; the CLI applies Dev Container user/environment settings. |
| `build` | Future prebuild/rebuild action. | Final JSON outcome/image name. |

The current upstream command surface does **not** list a supported `stop` or `down` command (they
remain unchecked in its status list). Do not promise either as a Dev Container CLI operation. A
future stop/remove implementation may use Docker only after exact managed-session proof; Docker
supports label filtering and JSON `container inspect`, which is appropriate for bounded status and
recovery, not ownership guessing. Sources: [CLI README](https://github.com/devcontainers/cli),
[current CLI options/source](https://github.com/devcontainers/cli/blob/main/src/spec-node/devContainersSpecCLI.ts),
[Docker label filters](https://docs.docker.com/reference/cli/docker/container/ls/), and
[Docker inspect JSON](https://docs.docker.com/reference/cli/docker/container/inspect/).

`--workspace-folder` defaults only to the standard `.devcontainer/devcontainer.json`, then
`.devcontainer.json`; `--config` is the supported explicit alternative path. `--id-label` both
sets labels and queries for an existing container. This supports a stable adapter contract across
Windows, macOS, and Linux where Docker plus the CLI are available, but the upstream install script
is documented only for Linux/macOS; Windows installation/version support must be an explicit
package prerequisite and fixture-tested before it is claimed as turnkey.

### Discovery, Selection, And Status

Discovery is a filesystem-only scan of the workspace root:

1. Include the standard `.devcontainer/devcontainer.json` and legacy `.devcontainer.json` when
   present.
2. Enumerate other JSON/JSONC definitions directly under `.devcontainer/` as candidates, but never
   treat a file as selected merely because it appears first.
3. For every candidate, record a workspace-relative definition reference, display name (if safely
   readable), and content fingerprint. Do not resolve `${localEnv:...}`, secrets, Compose state, or
   lifecycle commands during discovery.
4. Zero candidates is `unavailable`; one candidate becomes the proposed selection; two or more is
   `selection_required`. Non-interactive CLI/MCP calls fail closed with the candidate list until a
   workspace-relative `definition_ref` is supplied.

Status requires an explicit definition reference, then combines read-only selected-definition facts
with Docker inspection of containers whose labels match the workspace/definition identity. Return
`not_created`, `created`, `running`, `stopped`, `ambiguous`, or `unavailable`; do not infer an
editor attachment state. Docker labels are discovery hints, not sufficient ownership proof.

### Ownership, Approval, Cleanup, And Recovery

| Classification | Rule | Allowed automatic action |
| --- | --- | --- |
| `external` | Any discovered/user-started container, including one that happens to carry Dev Container labels but lacks an exact DockPipe session record and DockPipe session label. | Read-only status/log reference only; never stop, remove, rebuild, or adopt for cleanup. |
| `managed` | A future `up` result whose exact container id, selected definition fingerprint, workspace identity, and DockPipe session label were recorded together. | Read-only reconciliation only. |
| `orphan_candidate` | A Docker-labeled prior managed container whose local session record is missing or mismatched. | Report repair options; no automatic cleanup. |
| `ambiguous` | More than one matching container, a changed definition fingerprint, or any missing proof. | Fail closed and require a user selection/repair action. |

Future managed starts should pass a namespaced label such as `com.dockpipe.devcontainer.session`
through the CLI's `--id-label`, while preserving Dev Container labels. The local record must bind
the opaque container id, workspace identity, definition reference/fingerprint, and session id.
Never add a DockPipe label to an existing container solely to “adopt” it. “Use existing” initially
means read-only status and, only after a later explicit product decision, explicit `exec` without
cleanup authority.

Approval classes are: no approval for discovery/status; explicit intent plus approval for image
pull/build, feature installation, Compose create/start, and lifecycle hooks; separate approval for
rebuild; explicit reviewed intent for `exec`; and explicit approval for host editor launch. Stop is
an explicit managed-session action. Remove/down is destructive and requires a stronger confirmation
after exact managed-session proof. On cancellation or crash, retain the session record and report
reconcile/status; only an exact managed record may offer stop/remove repair. No action ever starts a
container during discovery or status.

### Recommended First Vertical Slice

Implement **read-only discovery plus selected-definition status only**. It proves the product seam
without pulling an image, starting Docker Compose, changing a lockfile, or launching an editor.

- Put Dev Container-specific scanning, `read-configuration` adaptation, Docker-label inspection,
  and normalization in a new `packages/ide/resolvers/devcontainer` resolver/package-owned assets.
  Do not add a `src/lib` or `src/cmd` special case.
- Expose its package workflow/command as the sole CLI execution path; the exact friendly command
  name can be chosen later, but it must accept `--workspace` and `--definition-ref` and fail closed
  on multi-definition discovery.
- Add one package-local CLI/MCP operation-result envelope, for example
  `devcontainer.lifecycle.v1`: `request_id`, `workspace_ref`, `definition_ref`,
  `definition_fingerprint`, `operation` (`discover` or `status` in this slice), normalized `state`,
  `ownership`, opaque `environment_ref` when known, safe `summary`, `log_ref`, and
  `next_actions`. Stream only ordered `discovered`, `selection_required`, `status`, `progress`,
  `approval_required`, `completed`, and `failed` events. No raw Docker/CLI command text, secret,
  resolved configuration, or editor-process heuristic crosses the boundary.
- Surface that same package operation through the DorkPipe host MCP bridge. Pipeon maps
  `unavailable` to no card, `selection_required` to a picker, `not_created` to “Dev Container
  available”, and `running`/ownership to status/repair UI. Start, logs, attach, rebuild, and stop
  controls remain disabled or absent until their matching CLI/MCP operations exist. The extension
  must not call Docker or the Dev Container CLI directly.
- Keep provider pools separate. A later resolver contract must prove how a managed Dev Container
  becomes an execution location before provider workers can use it.

Fixture-only validation: standard/legacy/alternate/multiple/malformed definitions; stable candidate
ordering and explicit-selection failures; captured `read-configuration` and Docker inspect/label
JSON for each normalized status/ownership outcome; changed-fingerprint, duplicate-container, and
lost-session recovery cases; CLI/MCP event sequence and redaction assertions; and Pipeon UI mapping
against recorded events. No test may invoke Docker, pull/build an image, run hooks, or require an
editor/account.

### Lifecycle Decisions

1. **Adapter distribution — decided (2026-07-13):** require an installed/pinned Dev Container CLI.
   The first lifecycle contract fixture-verifies the installed version against its pin; it does not
   yet execute the CLI live.
2. **Existing environments — decided (2026-07-13):** first release is managed-only. External or
   user-started containers remain status-only; no `exec`, adoption, labeling, cleanup, or mutation
   is permitted.
3. **Cleanup policy — decided (2026-07-13):** Pipeon close explicitly requests a stop only for an
   exactly proven managed container. The stopped container and its managed session record are
   retained for reuse; remove/down remains a separate destructive action requiring stronger
   confirmation. This is a Dev Container lifecycle request, never an incidental side effect of
   Pipeon stack teardown.
4. **First attachment:** VS Code, Cursor, Pipeon code-server, or status/exec-only; attach remains a
   host action and not proof of lifecycle ownership.
5. **Definition scope:** whether recursive/nonstandard definitions beyond the root and direct
   `.devcontainer/*.json` candidates are a supported product feature; the first slice should not
   guess.

## Implementation Update — Read-Only Discovery/Status Slice (2026-07-13)

Implemented the recommended first vertical slice in the package-owned
`packages/ide/resolvers/devcontainer` resolver only.

- The `devcontainer` package workflow provides deterministic, filesystem-only discovery for the
  standard, legacy, and direct root `.devcontainer/*.json` / `*.jsonc` definitions. It records a
  workspace-relative reference, safe display name when parsable, and content fingerprint. Multiple
  candidates always fail closed until the caller supplies `--definition-ref`; no selection is
  guessed.
- `status` requires that explicit reference and accepts captured `read-configuration`, Docker
  inspect, and optional managed-session JSON fixtures. It deliberately rejects absent live
  adapters: no code in this resolver invokes Docker, the Dev Container CLI, hooks, an editor, or a
  provider. The resulting `devcontainer.lifecycle.v1` NDJSON stream normalizes
  `unavailable`, `selection_required`, `available`, `not_created`, `created`, `running`,
  `stopped`, and `ambiguous` states plus `external`, `managed`, `orphan_candidate`, and
  `ambiguous` ownership.
- CLI execution is the package workflow; DorkPipe's existing tiered generic `dockpipe.run` MCP
  bridge invokes that same workflow and returns the same recorded event stream. There is no
  Pipeon Docker/Dev Container CLI path, no provider-pool integration, and no engine change.
- Fixture tests cover standard/legacy/alternate/malformed definitions, stable ordering,
  multi-definition refusal, adapter absence, `not_created` / external / managed / orphan / changed
  fingerprint / duplicate-container status, event sequencing, and identifier/workspace redaction.

## Implementation Update — Approved Managed `up` Contract Slice (2026-07-13)

Implemented the next lifecycle contract slice in `packages/ide/resolvers/devcontainer` only.

- `up` requires an explicit workspace-relative `--definition-ref` and first emits an
  `up_requested` event. Without an approval record bound to the request id, workspace identity,
  selected definition reference/fingerprint, and all pull/build/Compose/features/hooks risks, it
  emits `approval_required` and fails before an adapter result is read or a session record exists.
- The fixture adapter accepts only a successful installed/pinned Dev Container CLI result whose
  installed version equals its pin. A successful result must bind the container id, session id,
  selected workspace/reference/fingerprint, and
  `com.dockpipe.devcontainer.session`. The resolver persists that exact managed record only to an
  explicit workspace-relative output path, while the event stream exposes an opaque environment
  reference rather than the container id.
- The existing `devcontainer.lifecycle.v1` NDJSON and generic `dockpipe.run` MCP stdout path are
  unchanged. Pipeon remains an event consumer; provider pools remain separate. This contract adds
  neither a live CLI/Docker invocation nor any external-container execution, adoption, cleanup,
  attach, build, stop/down/remove, or Pipeon control.
- Fixture tests cover unapproved and incomplete approval failure, approved pinned-adapter result,
  session-record persistence, record-backed managed status, redaction, and the prior discovery and
  status cases. No test invokes Docker, a Dev Container CLI, hooks, or an editor.

Remaining recovery/cleanup risks: a crash after a real future CLI creates a container but before
the session record is persisted will be an orphan candidate; failed/cancelled real starts need a
later reconciliation/repair contract. The retention policy is decided: Pipeon close stops only an
exact managed container and retains its record for reuse. Remove/down and automatic recovery remain
unauthorized until their separate destructive/recovery contracts exist.

## Implementation Update — Pinned Live Read-Configuration Verification (2026-07-13)

The package-owned resolver now has an explicit `--live-read-configuration` status adapter. It
requires the installed `@devcontainers/cli` to equal package pin `0.87.0`, then invokes only
`read-configuration` for the selected definition and still consumes Docker status exclusively from
the existing captured fixture. Windows resolves the npm command shim to the package JavaScript entry
point under the current Node executable, without a shell. Missing, unpinned, timed-out, malformed,
or identity-mismatched output fails closed and retains no raw adapter output. Dev Container CLI
`0.87.0` performs its own label-filtered, read-only `docker ps` during this operation; that is the
sole Docker exception for the slice. No direct Docker call, `up`, hook, editor, provider, `exec`,
stop, remove, or Pipeon lifecycle path was added.
