# TASK-007 Generic Software Dev Workflow

## Dependency Status

TASK-006 closed on 2026-07-17. The package-owned Example Brain baseline now seeds native guidance
deterministically, and materialized documentation fails closed on ambiguous runtime or host-path
identity. The TASK-007 contract design is unblocked; no TASK-007 implementation is included in that
baseline slice. The contract baseline below is now settled; substantial package implementation
remains.

## Goal

Design one main governed software-development orchestration workflow that can be reused across repos
while letting each repo define its own tasks, durable outputs, and source-of-truth rules.

The reusable unit should be the orchestration engine and task contract, not a giant fixed
repo-specific workflow.

## Desired Model

### Fixed hard layer

These should remain governed by DockPipe/DorkPipe and should not be agent-designed at runtime:

- auth handling
- mounts and path boundaries
- access policy
- approval requirements
- publish/sync behavior
- destructive-action gates
- security-sensitive runtime settings
- budget ceilings and escalation policy
- logging and operation-result behavior
- artifact/result contracts

### Soft repo-defined layer

These should be declared by the repo or proposed inside a bounded contract:

- task list
- role definitions
- prompts and brief structure
- required durable artifacts
- repo-specific quality bars
- source-of-truth rules
- follow-up and repair task routing

### Agent-designed layer

Inside the hard envelope, allow a strong agent to propose or refine:

- bounded task decomposition
- task dependencies
- extra inferred artifacts beyond the required floor
- index/router structure
- doc splitting or consolidation
- targeted follow-up tasks after review or validation

The agent should not control the hard layer.

## Architecture

### Main workflow

Create one package-owned workflow family for general software-development orchestration.

Core stages:

- request intake
- planning
- bounded task execution
- merge
- verify
- approval
- apply
- optional publish/sync

### Repo task pack

Each repo should provide a task pack or orchestration declaration that plugs into the main workflow.

That pack should define:

- repo rules
- source roots and briefing hints
- required artifacts
- role/task definitions
- repo-specific validator expectations

The repo owns what work is being done. The main workflow owns how governed orchestration runs.

### Inferred output model

The workflow should require a stable minimum artifact set while allowing inferred additional outputs.

Pattern:

- `required_artifacts` declares the floor
- role workers use `materialize_outputs`
- apply infers the full bundle from materialized outputs
- verifier checks quality, duplication, and boundary compliance
- human review can push back on over-fragmentation or bad file shape

## Settled Contract Baseline

This contract-design slice settles the first implementation shape. It does not add the generic
workflow, change authored YAML, or refactor `example.brain`.

### Existing package capabilities to reuse

TASK-007 does not need a new DockPipe core primitive. The DorkPipe package already owns the reusable
orchestration mechanics:

| Capability | Existing package contract |
| --- | --- |
| Workflow declaration | `steps[].agent` and `agent.orchestration` already declare startup context, access, request, plan, shared collectors, agents, tasks, merge, verify, and apply. |
| Reusable roles | A sibling or bounded parent `agents.yml` supplies role defaults; inline `agent.orchestration.agents` overrides or adds workflow-local roles. |
| Task graph | `tasks[].id`, `agent`, `depends_on`, `goal`, `brief`, `context`, `constraints`, `expected_output`, and `max_cloud_tokens` materialize into `task-graph.json` and per-task artifacts. |
| Source briefing | Shared collectors, `context.required_artifacts`, `seed_paths`, and `source_roots` create bounded briefing artifacts without replacing mount and access boundaries. |
| Lane governance | Package-owned lane catalogs and `model_policy` select workers, enforce cloud budgets, record actual lanes, and emit halt and usage artifacts. |
| Durable output | `materialize_outputs` extracts reviewable multi-file bundles from worker responses. |
| Verification | Merge and verify artifacts cover graph shape, value-bar evidence, failures, rerun targets, and deterministic apply coherence. |
| Apply | Explicit `apply.outputs`, or `apply.target_root` plus `apply.required_artifacts`, support approval-gated writes and inferred output bundles. |
| Follow-up | Existing follow-up selection reruns named tasks and downstream dependents while reusing unaffected results. |
| Invocation | A package workflow is directly invokable, and repo workflows can call it through the existing `workflow:` plus `package:` step form with `vars` or typed `inputs`. |

The package workflow and helper scripts remain DorkPipe-owned. DockPipe core continues to provide
generic workflow, runtime, resolver, scope, and packaged-call behavior only.

### Four contract layers

| Layer | Owner | Durable | Contract |
| --- | --- | --- | --- |
| Hard layer | DorkPipe package | yes | Fixed workflow stages, mounts and access ceilings, deny rules, auth, lane catalog, budget ceilings, destructive gates, operation results, approval, apply mechanics, and publish/sync policy. Repo input may narrow authority but cannot widen it. |
| Task pack | Consumer repo | yes | Repo rules, source-of-truth order, prompt skeletons, reusable roles, recurring task templates, context seeds, quality bars, durable target roots, and required artifact floors. |
| Planner proposal | Run session | no | Exact request decomposition, dependency graph, role selection, inferred extra outputs, and bounded repair/follow-up tasks. The normalized proposal is an artifact, not executable authority. |
| Promoted config | Consumer repo | only after promotion | A small reviewed patch containing reusable soft-layer improvements. It cannot change the hard layer. |

The hard layer compiles and validates the other layers before workers run. A rejected task pack or
planner proposal fails closed; it never falls back to broader access or unapproved publish behavior.

### First task-pack representation

The first task pack is a thin repo-local workflow YAML file, not a new top-level schema. Its packaged
workflow step uses the existing `agent` and `agent.orchestration` fields as the task-pack body and
points the package-owned workflow back to that file and step through normal `vars` or typed `inputs`.
A sibling `agents.yml` remains the preferred home for reusable repo roles.

Proof shape:

```yaml
name: repo.software-dev
steps:
  - id: software_dev
    workflow: software.dev
    package: dockpipeproject
    vars:
      DORKPIPE_SOFTWARE_DEV_TASK_PACK: workflows/software-dev/config.yml
      DORKPIPE_SOFTWARE_DEV_TASK_PACK_STEP: software_dev
    agent:
      startup_prompt: Apply this repo's source-of-truth and quality rules.
      include_agents_md: true
      orchestration:
        request:
          text: Implement the bounded user request.
        plan:
          goal: Produce a verified, reviewable repo change.
        shared: []
        tasks: []
        merge: {}
        verify: {}
        apply:
          target_root: .
          required_artifacts: []
```

`software.dev` is the first package workflow name and `dockpipeproject` is its packaged workflow
namespace. Direct invocation passes the same repo-relative task-pack path and step id with `--var`;
a thin wrapper is only for durable repo defaults and convenience. The package implementation must
validate the referenced workflow file through the existing workflow schema, then extract only the
named step's task-pack fields. It must not interpret arbitrary sidecar YAML or use `imports:` to pull
consumer configuration into the package tree.

```bash
./src/bin/dockpipe --package dorkpipe --workflow software.dev --workdir . \
  --var DORKPIPE_SOFTWARE_DEV_TASK_PACK=workflows/software-dev/config.yml \
  --var DORKPIPE_SOFTWARE_DEV_TASK_PACK_STEP=software_dev --
```

The task pack may leave `tasks` empty only when planner mode is enabled and produces a valid run
proposal. Otherwise it must declare at least one task, matching the current orchestration floor.

### Deterministic precedence

Precedence is field-class-specific; it is not an unrestricted deep merge.

| Field class | Precedence and merge rule |
| --- | --- |
| Hard authority | Package policy always wins. Repo and planner values may only narrow read/write roots, worker authority, cost, or publish intent. Deny rules are an ordered union. Any widening request is a contract error. |
| Soft scalars | Per-run proposal, then repo task pack, then package default. Proposal values affect only the current normalized plan. |
| Roles and named templates | Merge by id: package defaults first, repo definitions replace soft fields with the same id, and planner definitions are ephemeral. Hard access, budget, approval, and publish fields are removed from this merge class. |
| Constraints and quality rules | Ordered union: package, repo, then proposal additions, with stable de-duplication. Later layers cannot remove earlier rules. |
| Required artifact floors | Ordered union of package and repo floors. A proposal may add artifacts but cannot remove or rename the floor. `context.required_artifacts` remains an input prerequisite; `apply.required_artifacts` is the durable output floor. |
| Run tasks and dependencies | A valid planner proposal supplies the exact run graph; otherwise repo tasks are used, then package seeds. Task ids are unique, dependencies must resolve, cycles fail, and every required output must have one producer. |
| Inferred outputs | Declared `materialize_outputs` plus planner-proposed extras form the candidate bundle. The durable floor must exist, duplicate relative paths fail, and all inferred files stay below the repo-owned target root. |
| Apply target | The repo task pack selects a target inside the package-approved mutable surface. The planner cannot change the target root. |
| Publish/sync | Disabled by default and package-owned. Neither task-pack nor planner precedence can enable it; an explicit package boundary and approval are required. |

Environment precedence for selecting the task-pack path continues to use normal DockPipe behavior,
including `--var` as an explicit invocation override. That selects the input contract; it does not
change field-level contract precedence inside the compiled plan.

### Stage and artifact boundaries

1. **Invoke:** run `software.dev` directly with a repo-relative task-pack path and step id, or run a
   thin repo wrapper that calls `workflow: software.dev` with `package: dockpipeproject`.
2. **Load:** validate the repo workflow YAML, locate the named step, load its sibling `agents.yml`,
   and reject paths outside the consumer repo or ambiguous step identity.
3. **Plan:** materialize request and shared artifacts. If planner mode is enabled, run one bootstrap
   planner task that emits a structured proposal artifact using the existing task, dependency,
   context, and `materialize_outputs` shapes.
4. **Compile:** combine package defaults, repo task pack, and proposal with the precedence table;
   validate ids, dependencies, access narrowing, artifact producers, target roots, and budget floors;
   then write the normalized `plan.json`, `task-graph.json`, and per-task artifacts.
5. **Execute and merge:** run only compiled bounded tasks. Preserve normalized worker results, merge
   evidence, cloud usage, halt state, and rerun targets.
6. **Verify:** run deterministic schema, path, required-artifact, reference, duplication, forbidden
   term, and repo validator checks before qualitative verification. A failure blocks apply. A review
   result may write a concrete workspace diff but must set `requires_human_review` and block publish.
7. **Approve and apply:** approval is mandatory. Explicit outputs are allowed, but the default is
   `target_root` plus `required_artifacts`, with the full unique bundle inferred from
   `tasks/*/materialized/*`. Apply writes only to the current governed workspace.
8. **Optional publish:** publish/sync is a separate, off-by-default package/runtime boundary after a
   passing verification and approved apply. The first implementation may omit publish entirely; it
   must not hide publish inside apply or let a planner request it.
9. **Promote:** a planner proposal remains under the run artifact root. Promotion creates a small
   patch only inside an explicit repo-owned task-pack/workflow surface and follows
   `docs/agents/workflows/planner-promotion-model.md`; verification alone does not grant mutation
   authority.

### Example Brain proof sketch

`example.brain` proves the data shapes without being refactored in this task:

- its host steps, budgets, access policy, lane selection, auth, approval, apply, and teardown map to
  the package-owned hard layer
- `startup_prompt`, shared repo collectors, sibling roles, tasks, merge labels, verify guidance,
  `apply.target_root`, and the four required output paths map to a repo task pack
- `planner_brain` demonstrates a bounded planning artifact consumed by downstream tasks, but it does
  not yet prove same-run dynamic graph compilation
- `rules_writer` and `inventory_writer` prove multi-file `materialize_outputs`; the four
  `apply.required_artifacts` prove the floor, while apply inference admits additional unique files
- the target `docs/agents/brain` makes this a durable consumer-guidance task pack

The completed Example Brain baseline applies only when a task pack generates durable consumer
guidance. Such tasks must include the package `example_brain_baseline` collector, and the compiler
must place that artifact before repo-specific context for each eligible task. Code changes, tests,
build artifacts, run reports, and other non-guidance outputs do not receive the baseline. Mixed task
packs apply it only to the durable-guidance tasks, never globally to every worker.

### Genuine package gap before implementation

No DockPipe core, schema, or language-support primitive is missing. DorkPipe does need one
package-owned two-phase contract compiler: the current helper reads one authored graph and plans all
tasks before any planner worker runs, so a planner response cannot yet become a validated same-run
task graph. The compiler must load the repo task-pack step, run and parse the bootstrap proposal,
apply the precedence rules, and then materialize the executable graph.

One existing apply handoff must also be repaired in that package slice: `planOrchestration` currently
copies `apply.require_approval` and `apply.outputs` into `plan.json` but drops authored
`apply.target_root` and `apply.required_artifacts`, even though downstream apply inference already
consumes those fields. This is a package helper defect, not a reason to add schema fields.

## Remaining Implementation Slices

1. Add focused package tests for task-pack loading, step identity, precedence, authority narrowing,
   graph validation, output floors, and the dropped inferred-apply fields.
2. Implement the DorkPipe task-pack loader and two-phase proposal compiler in package code and
   scripts, preserving the existing orchestration artifact contract.
3. Add the package-owned `software.dev` workflow with hard policy, direct invocation variables or
   typed inputs, planner bootstrap, execute/merge/verify/approval/apply stages, and no implicit
   publish.
4. Add a minimal repo task-pack fixture that proves static tasks and planner-proposed tasks, inferred
   extra outputs, required floors, deterministic verification, and approval-gated apply.
5. Run `example.brain` as an unchanged proof sketch against the contract tests; do not migrate or
   replace it until the generic workflow is stable.
6. Implement proposal promotion as a separate package-owned slice with explicit mutable surfaces,
   small reviewable patches, and rollback-safe verification.
7. Document consumer invocation and only then evaluate thin wrappers for `brain.optimize`-style
   workflows. Provider pools, host bridges, remote execution, and TASK-008 remain out of scope.

## Supporting Decisions

### Task-pack format

Default to YAML task packs.

Rationale:

- YAML is already the authored workflow surface.
- It keeps the contract inspectable and easy for consumers to override.
- A richer typed model or PipeLang layer does not currently show enough advantage to justify another
  authored surface.

PipeLang can remain an optional future layer if a clear reuse or composition benefit appears later,
but it should not be required for the first generic software-dev workflow design.

### Repo invocation model

Support both levels, but optimize for the simplest consumer entrypoint.

Preferred shape:

- the package-owned generic workflow should be directly invokable
- a repo can wrap it in a thin local workflow when it wants local defaults, prompts, or convenience
- the simple path should not force a consumer to author a large wrapper just to use it

This should also support a higher-level agent-generated layer:

- consumers can start with a simple direct invocation
- stronger agents can later generate more sophisticated repo-local workflows
- those local workflows should still run on top of the same governed orchestration contract

### Validator expectations

Keep the verifier generic and make repo quality bars mostly prompt- and rule-driven.

Direction:

- consumer repos should express strong expectations through prompts, repo rules, required artifacts,
  and durable guidance docs
- the shared verifier should enforce generic contract quality, boundaries, and artifact integrity
- repos should not need to rewrite verifier logic just to express a stronger quality bar

### Master-agent layer

Expect a future master-agent layer that can be backed by Codex, Ollama, Claude, or another configured
model lane.

The safety requirement is the execution boundary, not the provider:

- host-side master executors need sandbox and escalation semantics
- containerized master executors can plan through any approved model lane
- privileged host actions must go through a governed DockPipe MCP or host bridge
- the bridge should ask the user or policy to approve, deny, or escalate structured host-action
  requests through the CLI first
- approved host actions should return operation-result events/artifacts with status, duration, IDs,
  and mutation summary
- future UI surfaces should render the same bridge requests and operation-result stream instead of
  adding a second approval protocol

Executor modes:

- bridge mode is for Docker/container or other non-host-sandboxed planners; host mutation goes
  through capability-scoped bridge calls
- native sandbox mode is for executors already constrained by an OS-enforced DockPipe runtime; a
  local offline agent may run entirely inside that runtime, while a cloud-backed agent uses a
  governed provider control plane and sends every tool action through the sandboxed executor
- both modes must use the same policy language and operation-result events so the CLI and future UI
  do not fork behavior

That layer should orchestrate the generic workflow; it should not replace the underlying governed
contract.

## Host Sandbox Runtime Decision

Resolved direction: prototype the generic `host-sandbox` runtime on Linux first, with no implicit
fallback to unrestricted `kind: host` execution. The authoritative architecture and audit decisions
are in
[host-sandbox-runtime-design-decision-2026.md](../../research/host-sandbox-runtime-design-decision-2026.md)
and
[host-sandbox-runtime-audit-addendum-2026.md](../../research/host-sandbox-runtime-audit-addendum-2026.md).

The first implementation is an explicitly opted-in preview with a versioned guarantee contract,
active enforcement probes, canonical workspace roots, offline networking, inherited descendant
restrictions, structured approvals, and complete teardown. Windows remains a narrower technical
preview; required macOS guarantees fail closed. A cloud model uses the governed split
controller/executor topology until a narrow in-sandbox provider broker exists.

## Planner Promotion Decision

Resolved direction: planner output starts as a session artifact and graduates into durable repo
configuration only through the promotion model in `docs/agents/workflows/planner-promotion-model.md`.

Summary:

- hard runtime config remains DockPipe/DorkPipe-owned and off-limits to agent-designed mutation
- soft repo config can be proposed by agents when it improves future runs
- exact task splits, lane choices, inferred extras, and experimental graph rewrites remain per-run
  artifacts until verification and promotion checks pass
- strong verification is required, but it does not grant authority to widen runtime permissions

This keeps useful AI-designed workflow knowledge from staying ephemeral forever while blocking the
dangerous version where a planner quietly changes mounts, auth, approval gates, publish behavior, or
budget policy.

## Promotion Model

Persist these by default when they improve results and pass verification:

- reusable role definitions
- recurring task templates
- required artifact floors
- source-of-truth rules
- prompt skeletons and quality bars
- stable dependency graph patterns
- follow-up repair task templates

Keep these as session artifacts by default:

- exact task decomposition for one user request
- exact dependency graph for one run
- selected model lanes or provider details
- one-off inferred artifact splits
- repair plans and validation failures
- generated richer workflows that have not passed promotion checks

Generated workflows can exist as artifacts first. They become repo-local workflow patches only when
the target path is inside an allowed mutable workflow surface, the schema passes, the value bar is
met, and the patch does not alter hard runtime authority.

## Acceptance Criteria

TASK-007 implementation is complete only when all of these are true:

- `software.dev` is package-owned and keeps mounts, access ceilings, auth, lane/budget policy,
  approval, apply mechanics, and publish/sync outside repo and planner control.
- A consumer task pack is a schema-valid repo workflow step using current `agent.orchestration`
  fields plus an optional sibling `agents.yml`; no new DockPipe authored surface is required.
- Direct package invocation and a thin `workflow:` plus `package:` wrapper compile the same normalized
  contract from an unambiguous repo-relative task-pack path and step id.
- The compiler implements the documented precedence, rejects authority widening, preserves ordered
  floors and constraints, and emits deterministic normalized plan, graph, task, and proposal
  artifacts.
- Planner mode is two-phase: the bootstrap proposal is bounded and validated before its tasks can
  execute, and its output remains session-only unless separately promoted.
- Static and planner-generated graphs both enforce unique ids, valid dependencies, no cycles, one
  producer per required output, bounded access, and package budget ceilings.
- Verification covers required floors, inferred extras, duplicate paths, target containment,
  Markdown/YAML references, repo validators, value-bar evidence, and publish eligibility.
- Apply requires approval, preserves `target_root` and `required_artifacts` in `plan.json`, and writes
  only the verified explicit or inferred bundle into the governed workspace.
- Publish is absent or a distinct off-by-default approved boundary; apply alone never publishes.
- The Example Brain baseline is deterministically injected only for durable consumer-guidance tasks,
  including eligible tasks in a mixed pack, and never for general implementation outputs.
- `example.brain` remains behaviorally unchanged until the generic contract passes focused package
  tests and a proof fixture; any later migration is a separate reviewed slice.
- Package tests, workflow validation, compile checks, and documentation prove the contract without
  changes under `src/lib`, `src/cmd`, schema, or language support.

## Still Open

- Implement and test every slice under `Remaining Implementation Slices`; this contract-design slice
  deliberately closes none of that implementation work.
- Implement planner-promotion checks for graduating verified reusable soft-layer proposals into
  durable repo config.
- Decide whether to migrate `example.brain` to a pure task pack only after the generic workflow is
  stable; it remains the unchanged proof sketch for now.
- Implement the CLI-first governed MCP/host bridge and the Phase 0 versioned guarantee contract and
  Linux offline probes from the host-sandbox decision; provider-broker implementation remains later
  work and is not a prerequisite for the package-only task-pack compiler.
- After the Linux security and performance baselines stabilize, evaluate signed DockPipe-owned native
  launchers behind the same `host-sandbox` contract to reduce external runtime dependencies and
  high-frequency startup overhead; require conformance parity and measured benefit before migration.
