# TASK-007 Generic Software Dev Workflow — Closed

Completed: 2026-07-18

## Shipped

- Added package-owned `software.dev`, selected by an exact consumer-repo-relative workflow path and
  step id without a new DockPipe schema or engine primitive.
- Kept access ceilings, deny rules, budgets, approval, apply mechanics, and absent publish/sync in
  DorkPipe's hard layer. Repo task packs and planner proposals can narrow but cannot widen it.
- Added deterministic task-pack loading, field-class normalization, strict proposal parsing, and a
  two-phase compiler. Planner tasks cannot execute until the bootstrap proposal validates and
  compiles without partial executable artifacts.
- Preserved request, plan, graph, per-task, merge, verification, approval, apply, usage, halt, and
  proposal evidence under the normal orchestration artifact root.
- Preserved ordered required-output floors while allowing unique inferred materialized outputs.
  Normal apply requires verification and approval and writes only below the repo-selected target.
- Kept Example Brain behavior unchanged while proving its durable-guidance baseline maps into the
  generic contract only for eligible tasks.
- Added offline promotion candidate evaluation, deterministic patch generation, a separately
  authored digest-bound approval, transactional application, rollback, and idempotent reapply.
- Documented copy/paste-ready static and planner direct invocation plus the supported thin
  `workflow: software.dev` / `package: dockpipeproject` repo wrapper.
- Proved on temporary consumer copies that direct selection and the thin wrapper compile identical
  request, normalized plan, task graph, per-task artifacts, approval/apply target and floor,
  publish/sync state, task-pack path, and selected step identity.

## Consumer Contract

The canonical invocation, artifact, approval/apply, wrapper, and promotion command documentation is
[`packages/dorkpipe/workflows/software.dev/README.md`](../../../../packages/dorkpipe/workflows/software.dev/README.md).
Planner promotion policy and approval ownership remain canonical in
[`docs/agents/workflows/planner-promotion-model.md`](../../workflows/planner-promotion-model.md).

Direct invocation is the default. A thin repo-owned wrapper is justified only to pin durable path,
step, and planner-mode defaults or provide a stable short command. `software.dev` currently exports
raw workflow vars, so wrappers use `vars`; they must not invent a duplicate typed surface.

A package-level `brain.optimize`-style wrapper was rejected. It would duplicate authored surface and
add a handoff without improving breadth, safety, cost, validation, rerunability, or traceability over
direct `software.dev` invocation. No new wrapper workflow was added.

## Boundaries Preserved

- No changes under `src/lib`, `src/cmd`, workflow schema, or language support.
- No provider pools, host bridges, remote execution, cloud/model calls, automatic promotion,
  approval creation, approval UI, publish, or sync behavior.
- Example Brain migration remains a separate future decision, not unfinished TASK-007 work.
- Governed host-bridge and remote-task work remains tracked outside TASK-007.

## Closure Evidence

The package-local offline proof covers static, planner, seed, invalid-proposal, normal apply,
promotion candidate, patch, deny/stale approval, transactional apply, and consumer invocation
equivalence paths. Focused helper tests, both workflow validations, package workflow compilation,
repository searches, and diff checks were run for closure. No genuine TASK-007 implementation slice
remains.
