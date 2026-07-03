# Operation Results

This document defines a repo-wide pattern for DockPipe operations that do work, take time, and
produce a result.

It is broader than Git runtime sessions. The same contract should apply to engine/runtime work,
CLI-visible actions, package/build flows, helper-container setup, DorkPipe orchestration support
steps, and later UI/event surfaces.

## Goal

DockPipe should stop inventing a new status string, log phrase, or ad hoc return payload for each
code path.

Any meaningful operation should be modeled as one unit of work with:

- a stable unit name
- a consistent result shape
- a consistent status vocabulary
- one human-log rendering contract
- one machine/event mapping contract

That keeps "did a thing, took time, produced a result" as the shared mental model across Go, CLI,
and shell helpers.

## Scope

This pattern is intended for operations such as:

- session creation, workspace bootstrap, volume seed/sync, worker lease, checkpoint, sync, publish
- container start/stop, Docker helper runs, image bootstrap, package/tooling preparation
- package compile/build/install/release operations
- workflow plan/merge/verify/apply support work
- any future long-running action surfaced in the CLI, logs, events, or UI

Tiny pure functions do not need this contract. Meaningful operational work does.

Invisible infrastructure work still counts as meaningful operational work. If the user would care
whether it happened, how long it took, or why it failed, it should be modeled as a unit.

Examples:

- seeding a managed workspace volume from Git
- preparing a helper container before worker start
- resolving auth material for a runtime-owned Git action
- discovering or validating required workspace/session state before mutation

## Canonical Ownership

Go should own the canonical contract.

- core type definition lives in Go
- core status vocabulary lives in Go
- CLI rendering derives from the Go result
- structured events/JSON derive from the Go result
- bash helpers mirror or adapt the contract when needed; they do not define a parallel one

That means bash can have helper wrappers, but bash should not be the architectural source of truth
for operation status semantics.

## Canonical Shape

Prefer a concrete Go struct plus helpers over a pure interface-only design. Interface layering can
sit on top if a package needs it later.

Conceptual baseline:

```go
type Result struct {
    Unit       string
    Status     string
    Message    string
    StartedAt  time.Time
    FinishedAt time.Time
    DurationMs int64
    IDs        map[string]string
    Data       map[string]any
}
```

The point is not the exact field names. The point is that every meaningful operation should have the
same operational envelope.

### Required Fields

- `Unit`: stable dotted or slash-free operation name such as `session.create`,
  `session.volume.seed`, `container.run`, `package.compile.workflow`
- `Status`: stable small vocabulary
- `Message`: short human summary, optional but preferred
- duration/timing fields
- identifiers relevant to the unit
- optional details/data payload for debugging or downstream consumers

## Status Vocabulary

Keep the status vocabulary small and stable.

Recommended baseline:

- `start`
- `progress`
- `done`
- `fail`

Not every operation needs to emit `progress`, but every surfaced operation should be able to map to
the same lifecycle.

Domain payloads such as "published", "archived", or "conflict" can still exist as operation data or
domain-state results. They should not force each code path to invent a different log lifecycle.

Example:

- lifecycle status: `done`
- domain state inside `Data`: `publish_status=published`

## Logging Contract

Human-facing stderr logging should render from the same result envelope.

Example shape:

```text
[dockpipe] ts=2026-07-02T21:15:10Z unit=session.volume.seed status=done duration_ms=1824
```

Failure shape:

```text
[dockpipe] ts=2026-07-02T21:15:11Z unit=session.volume.seed status=fail duration_ms=640 error="git clone failed"
```

Rules:

- operation-result lines should carry an explicit timestamp
- unit name first
- same `status=` vocabulary everywhere
- identifiers are flat and bounded
- failures include the actionable error
- no one-off phrasing for important side effects when the same information can be rendered from the
  result
- hidden prerequisite work should emit its own unit instead of only surfacing the downstream
  failure

If a command depends on preparatory work such as auth discovery, repo validation, branch lookup, or
volume bootstrap, those steps should not disappear behind a later generic error. They should show
up as named units with their own success/failure result.

## Preflight

Important operations should separate preflight from mutation where that boundary improves operator
clarity.

Typical pattern:

1. preflight unit validates assumptions and discovers required inputs
2. work unit performs the actual mutation or long-running action
3. result units make both phases visible in logs and structured events

Examples:

- `session.volume.preflight` verifies repo identity, base ref, target branch, auth inputs, and
  helper prerequisites before `session.volume.seed`
- `session.publish.preflight` verifies remote, branch, auth, and dirty/checkpoint policy before
  `session.publish`

Preflight does not need to become bureaucracy. It exists to catch bad assumptions before DockPipe
does expensive or destructive work and to make failure location obvious.

## Structured Events

Structured events should map from the same result contract rather than being authored independently.

The canonical observed-state event log is append-only JSONL. When `DOCKPIPE_EVENT_LOG` is set,
operation results are mirrored to that file. The file is the durable runtime ledger for the current
run/session; Postgres or another database may index it later, but should be treated as a rebuildable
projection rather than the source of truth.

Current event schema:

```json
{
  "schema": "dockpipe.operation_event.v1",
  "type": "operation_result",
  "ts": "2026-07-02T21:15:10Z",
  "unit": "session.volume.seed",
  "status": "done",
  "started_at": "2026-07-02T21:15:08Z",
  "finished_at": "2026-07-02T21:15:10Z",
  "duration_ms": 1824,
  "ids": {
    "workspace": "unitehere",
    "session": "run-1842"
  }
}
```

Rules:

- JSONL files are canonical for observed runtime facts.
- YAML files are canonical for desired state and configuration.
- Postgres indexes JSONL/YAML as a rebuildable projection for PipeDeck, dashboards, search, and
  cross-run queries.
- Event writers should append; they should not rewrite earlier observed facts.
- Structured events must derive from the Go result contract instead of being hand-authored in
  parallel.

CLI inspection:

```bash
dockpipe runs events --event-log <path>
dockpipe runs events --event-log <path> --json
```

When `--event-log` is omitted, `dockpipe runs events` reads `DOCKPIPE_EVENT_LOG`.

## Bash Adapter Rules

Shell helpers may still need to emit operational status, especially for package-owned scripts and
DorkPipe orchestration support code. When they do:

- reuse the same unit names and status vocabulary
- keep key/value rendering aligned with the Go renderer
- do not invent a separate shell-only lifecycle taxonomy
- prefer shell wrappers that adapt an upstream Go-owned result contract over freehand `echo`
  conventions

In practice:

- good: shell emits `unit=devstack.up status=start`
- bad: one script says `starting`, another says `bootstrapping`, another says `doing setup now`
  with no shared shape

## CLI Implications

The CLI should treat operation results as the standard way to summarize real work.

That means:

- long-running commands should surface named units
- nested work should compose child results rather than only printing freehand lines
- JSON or machine-readable command modes should be able to expose the same result data without
  scraping text
- interactive human CLI mode should render in-flight work with the existing DockPipe loading
  animation/spinner rather than printing a final-looking `status=start` line and then leaving it in
  the scrollback as if it were the result

Recommended interactive pattern:

1. while a unit is active, show the spinner/loading row for that unit
2. when the unit finishes, replace or finalize that row with the stable result line
3. include duration and key identifiers in the completed or failed rendering

When a unit cannot safely use the live spinner because child work is already writing verbose output,
the CLI should still prove liveness with periodic `status=progress duration_ms=...` heartbeat lines
until the final `done` or `fail` result is emitted.

Example:

```text
[dockpipe] seeding session workspace volume... unitehere / run-1842
[dockpipe] ts=2026-07-02T21:15:12Z unit=session.volume.seed status=done duration_ms=1824 workspace=unitehere session=run-1842 volume=dockpipe-ws-unitehere-run-1842
```

This keeps the nice live CLI experience while preserving one canonical result contract after the
work completes.

## Testing Implications

Tests should be able to assert:

- unit names
- status transitions
- required identifiers
- expected success/failure messages

This is stronger than string-matching arbitrary log lines and makes regressions easier to catch.

## Migration Order

Recommended implementation order:

1. Define the shared Go result type and renderer.
2. Apply it to one vertical slice first:
   session create -> volume seed/sync -> worker lease -> checkpoint -> publish.
3. Map the same results into structured session events.
4. Expand outward to container runs, package/build/install flows, and other CLI-visible work.
5. Align bash helpers to the same vocabulary and unit naming as thin adapters.

## Non-Goals

This contract does not replace:

- domain result payloads
- workflow/task result artifacts
- typed business objects returned by higher-level packages

It is the operational envelope around meaningful work, not a replacement for all domain models.
