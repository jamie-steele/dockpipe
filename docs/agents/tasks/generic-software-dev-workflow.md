# TASK-007 Generic Software Dev Workflow

## Dependency Status

TASK-006 closed on 2026-07-17. The package-owned Example Brain baseline now seeds native guidance
deterministically, and materialized documentation fails closed on ambiguous runtime or host-path
identity. The TASK-007 contract design is unblocked; no TASK-007 implementation is included in that
baseline slice.

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

## Proposed Architecture

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

## First Slices

### Slice 1

Use the new `example.brain` workflow as the first task-pack-style example running on the general
contract.

Target:

- prove required-artifact floor plus inferred extra artifacts
- prove repo-native wording rules
- prove source-precedence rules

### Slice 2

Refactor `brain.optimize`-style consumer workflows so they look like repo task packs invoking the
same main governed orchestration behavior instead of carrying a large custom workflow body.

### Slice 3

Add agent-designed planning for the soft layer only:

- proposed task list
- proposed dependency graph
- proposed extra artifacts
- proposed follow-up tasks

All proposals remain schema-validated and verifier-reviewed before apply/publish.

## Current Direction

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

## Recommendation

Implement the generic workflow as a package-owned contract with YAML task packs as the first-class
extension point.

Use this hierarchy:

- package-owned generic software-dev workflow
- repo-owned task pack for standing roles, rules, artifact floors, and prompt skeletons
- per-run planner artifact for current task split and inferred extras
- optional promoted repo-local wrapper when a verified recurring pattern deserves durability

This lets simple repos invoke the generic workflow directly while still allowing advanced repos or a
future master agent to generate richer workflows on top of the same governed contract.

## Still Open

- Design the package-owned generic software-dev workflow contract and decide its durable public YAML
  surface.
- Define the repo task-pack contract for required artifacts, roles, task definitions, and repo rules.
- Add a bounded agent-designed soft layer for tasks, dependencies, and inferred artifacts while
  keeping security, mounts, approval, and publish settings fixed.
- Implement planner-promotion checks for graduating per-run artifacts into durable repo config.
- Implement the CLI-first governed MCP/host bridge and the Phase 0 versioned guarantee contract and
  Linux offline probes from the host-sandbox decision; provider-broker implementation remains later
  work.
- After the Linux security and performance baselines stabilize, evaluate signed DockPipe-owned native
  launchers behind the same `host-sandbox` contract to reduce external runtime dependencies and
  high-frequency startup overhead; require conformance parity and measured benefit before migration.
- Decide whether `example.brain` becomes a pure task-pack example on top of the generic workflow or
  remains a slightly thicker starter wrapper.
