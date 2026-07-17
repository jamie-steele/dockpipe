# Planner Promotion Model

Read when designing generic agentic workflows, repo task packs, or rules for promoting planner
output into durable repo configuration.

## Position

Planner output starts as a session artifact. It can become repo-owned configuration only when the
change is inside an allowed mutable surface, passes verification, and improves future runs without
widening runtime authority.

This keeps the useful part of agent-designed workflow shape while preserving DockPipe's governed
runtime boundary.

## Layers

| Layer | Owner | Durable by default | Examples |
| --- | --- | --- | --- |
| Hard runtime layer | DockPipe/DorkPipe | yes | mounts, access boundaries, auth, approval, publish/sync, budgets, destructive gates, operation-result logging |
| Soft repo layer | repo | yes, when reviewed or verified | role definitions, recurring task templates, required artifacts, source-of-truth rules, prompt skeletons, stable dependency shapes |
| Run planning layer | session | no | exact task split, lane choices, inferred extra artifacts, repair plan, experimental graph rewrites |

Agents may propose changes to the soft repo layer. They must not design or mutate the hard runtime
layer from inside a normal workflow run.

## Durable Config Defaults

These are usually worth storing in repo config or repo brain docs when they improve results:

- source-of-truth rules
- reusable role definitions
- recurring task templates
- required artifact floors
- prompt skeletons and quality bars
- stable dependency graph patterns
- follow-up repair task templates

These should usually remain per-run artifacts:

- exact task decomposition for one request
- exact dependency graph for one run
- selected model lanes or provider details
- inferred extra docs or one-off split decisions
- validation failures and repair attempts
- generated richer workflows that have not passed promotion checks

## Promotion Rules

A planner proposal may be promoted only when all of these are true:

- the target path is inside an explicit repo-owned mutable surface
- the change does not alter mounts, auth, secrets, access policy, approval gates, publish behavior,
  budget ceilings, or destructive-action policy
- verification passes for schema, boundaries, required artifacts, forbidden terms, and repo-specific
  quality bars
- the generated diff is small enough to review and has a clear reason to exist on future runs
- the proposal preserves repo-native language and does not leak runtime mount points or orchestration
  mechanics into durable consumer docs

Verification is necessary but not sufficient. Passing checks does not grant authority to widen the
workflow's runtime permissions.

## Review Candidate Boundary

`software.dev` now implements the first promotion boundary as a deterministic review artifact, not
as a repository patch. The package-local evaluator accepts the exact repo-relative task-pack path,
selected step id, and existing run artifact root. It requires one selected raw proposal, an identical
normalized proposal, consistent compiler metadata, and `verify/result.json` with `status: pass`.

The evaluator reuses the verifier's value-bar and direct-worker-baseline evidence. Weak value-bar
results, a baseline that prefers one direct worker, missing evidence, review, failure, and inconsistent
artifacts all fail closed. Passing evidence can produce a candidate only when the proposal contains a
meaningful reusable soft-layer delta.

The review artifact is `proposal/promotion-candidate.json`. Its mutable identity is the exact selected
task-pack file plus step id. An exact sibling `agents.yml` can be named as a possible role target only
when it is a regular repo-owned sibling with an `agents` mapping; parent or symlinked sidecars are not
promotion targets. Candidate generation is atomic and writes nothing to the consumer repository.

Promotable data is limited to reusable role wording and constraints, stable task-pack constraints,
required-artifact floor additions, and reusable startup, plan, merge, or verification guidance. Exact
tasks, dependencies, lane/provider/model choices, inferred output declarations, repair evidence,
access and deny policy, budgets, approvals, apply targets, publish/sync behavior, auth, secrets, and
destructive-action policy are explicitly excluded.

Approval-gated patch generation and repository application remain a later boundary. An eligible
candidate grants neither mutation authority nor approval.

## Prompt Layers

Use prompts in layers so the durable parts stay maintainable:

| Prompt layer | Scope |
| --- | --- |
| Standing repo rules | durable source precedence, terminology, quality bars, forbidden claims |
| Task-pack prompt skeletons | reusable role/task intent and required artifact expectations |
| Per-run compiled prompts | current user request, selected sources, task split, repair context |

Do not persist a fully compiled per-run prompt as repo config. Persist the reusable skeleton or rule
that made the run better.

## Generated Workflows

Agent-generated workflows are allowed as session artifacts. They can become repo-local workflow
patches only after promotion checks and inside a declared mutable workflow area.

Default path:

1. planner writes a session artifact describing the proposed workflow or task pack
2. verifier checks safety, schema, source boundaries, and value over one strong direct worker
3. apply proposes a repo diff for the soft layer only
4. human or strong automated review accepts, rejects, or sends it to repair

The generic package-owned workflow remains the governed execution contract. Repo-local wrappers
should stay thin unless they add durable repo defaults or a verified higher-level workflow.

## Master Agent Boundary

A future master-agent layer may run either on the host or inside a container. The important boundary
is not which model runs the master role; it is where execution authority lives.

There are two valid executor modes:

| Mode | Use when | Host authority |
| --- | --- | --- |
| Bridge mode | the master runs in Docker or another non-host-sandboxed lane | all host actions go through the governed DockPipe MCP or host bridge |
| Native sandbox mode | the master executor has a trusted host sandbox and escalation runtime, such as Codex | safe host reads, edits, and commands may use the native sandbox path; privileged actions still require escalation |

Both modes must emit the same structured request, approval, and operation-result events so CLI, UI,
logs, and artifacts do not depend on which model/provider executed the master role.

Supported shape:

- a master model plans, reads artifacts, and proposes next actions
- privileged host actions go through a governed DockPipe MCP or host bridge
- the bridge presents structured intent, risk, expected mutation, and required approval through the
  CLI first
- the user or configured policy approves, denies, or escalates the request
- the bridge executes only the approved action and returns an operation-result event/artifact

This allows Codex, Ollama, Claude, or another configured model lane to act as the planner when it is
boxed into the same bridge contract. No model should receive raw host authority merely because it is
the master lane.

The first product surface should be CLI-driven:

- interactive runs prompt in the terminal with the requested operation, risk, paths, command class,
  and expected mutation
- non-interactive runs use explicit policy flags or environment configuration and fail closed when
  approval is required but unavailable
- every approved, denied, skipped, or failed host request emits the same operation-result event shape
- a future UI should subscribe to or render the same bridge events instead of inventing a separate
  approval protocol
- UI/chat surfaces should send requests into the CLI/master protocol, stream structured events back,
  render approval prompts, and forward approve/deny/escalate decisions

Default policy:

- host-side master executors must use a sandbox and escalation model
- containerized master executors must use capability-scoped MCP or bridge calls for host actions
- models without host sandbox semantics are acceptable as planners only when the bridge owns all
  host mutation authority
- publish, apply, cleanup, auth, dependency install, and destructive operations require structured
  bridge requests and operation-result logging

## Safety Bias

Prefer persistence when a verified improvement will improve future runs. Avoid AI junk by limiting
the mutable surface, requiring small diffs, enforcing repo-native wording, and making rollback easy.

The main failure to avoid is letting useful task knowledge remain ephemeral forever. The second
failure is allowing a planner to quietly grant itself more runtime authority.
