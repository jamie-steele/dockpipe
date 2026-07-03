# DockPipe engine (data flow)

**Terms:** **[architecture-model.md](architecture-model.md)**. This page is the current execution
shape of the Go CLI: how DockPipe resolves workflows, prepares runtime state, runs host/container
work, and applies lifecycle actions around it.

## Primitive

DockPipe still has one core action:

1. prepare runtime-owned state and optional host pre-work
2. run the requested step or command in the selected runtime
3. apply lifecycle follow-up such as outputs merge, checkpoint, cleanup, or post-action

For simple single-run flows that still read like run -> isolate -> act, the engine now also
owns managed workspace/session behavior, host setup, packaged workflow calls, and compiled package
resolution.

**Detach:** **`-d`** runs the container without attaching (stays up until the inner command exits).

## Components

| Piece | Role |
|-------|------|
| **`src/bin/dockpipe`** | Launcher → compiled binary or **`go run ./src/cmd`** during source checkout use. |
| **`src/cmd`**, **`src/lib/…`** | CLI parsing, workflow/package resolution, runtime orchestration, Docker/host execution, lifecycle actions. |
| **embedded bundle** | Ships **`src/core/`**, repo **`workflows/`**, package assets, **`assets/entrypoint.sh`**, and **`VERSION`** into the bundled/materialized runtime layout. |
| **`assets/entrypoint.sh`** | Container entrypoint: run command, then optional **`DOCKPIPE_ACTION`**. |
| **compiled package store** | Project-local compiled core/workflow/resolver tarballs under **`bin/.dockpipe/internal/packages/`**. |
| **session state** | Runtime-owned managed workspaces, Docker volumes, leases, checkpoints, and audit metadata under **`bin/.dockpipe/sessions/`**. |

**Paths:** profile/script/package resolution is centralized in **`src/lib/infrastructure/paths.go`**
and related package/store helpers. Do not treat old `templates/core` repo paths as the in-repo
source of truth.

## Workflow resolution and execution shape

Current execution can enter through several surfaces:

- named workflow from **`workflows/<name>/config.yml`**
- nested workflow roots listed in **`dockpipe.config.json compile.workflows`**
- bundled example workflow under **`src/core/workflows/<name>/`**
- compiled workflow tarball from the package store
- arbitrary **`--workflow-file <path>`**

After resolution, DockPipe may:

1. merge workflow defaults, step overrides, runtime/resolver selection, and strategy behavior
2. prepare managed workspace/session state when **`workspace.mode: managed`** is in play
3. run host setup or host steps
4. run container steps with `/work` mounted from the checkout or runtime-owned session storage
5. merge outputs, write manifests, checkpoint sessions, sync managed volume state, and run finally
   or after hooks

## Typical data flows

### Simple container run

```text
dockpipe --workflow run -- echo ok
  -> resolve workflow + runtime/resolver
  -> prepare isolate/image
  -> docker run with /work mounted
  -> optional DOCKPIPE_ACTION / host follow-up
```

### Managed session volume run

```text
dockpipe --workflow <name>
  -> resolve workflow.workspace
  -> create or resume runtime-owned session branch/workspace
  -> preflight/create/seed managed Docker volume when workspace.storage=volume
  -> run worker/container against /work in that volume
  -> sync back, checkpoint, cleanup, or publish via runtime lifecycle
```

### Multi-step workflow

```text
workflow config.yml
  -> resolve steps / finally / packaged workflow calls
  -> run host and container steps in order
  -> merge outputs and artifacts between steps
  -> run finally steps even on failure
```

## Extension points

1. **Core authoring tree** — **`src/core/runtimes/`**, **`src/core/resolvers/`**,
   **`src/core/strategies/`**, **`src/core/assets/`**. Installed/materialized core uses the same
   logical shape under the global or bundled layout.
2. **Package authoring trees** — repo/package **`workflows/`**, package **`resolvers/`**, package
   assets, and compiled package tarballs in the store.
3. **Images** — Dockerfiles under resolver/core/package **`assets/images/`** plus compiled image
   artifact manifests.
4. **Workflow YAML** — **`config.yml`**, **`steps:`**, **`workspace:`**, **`security:`**,
   **`agent:`**, and packaged workflow step forms.

## Runtime-owned lifecycle

The engine now owns more than process launch:

- managed Git session branches and workspaces
- session Docker volume seed/sync/cleanup
- worker lease and checkpoint lifecycle
- build/package/image preparation flows
- operation-result logging with timestamps, durations, and progress heartbeats for meaningful work

Those are engine/runtime responsibilities, not resolver-local shell conventions.

## Permissions

For normal Docker runs, DockPipe still prefers host-user file ownership semantics where possible so
files in **`/work`** map cleanly back to the host checkout. Exact user mapping can vary by runtime
profile or image, but workflow authors should continue to think in terms of governed `/work`
execution rather than ad hoc container flags.
