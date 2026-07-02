# DorkPipe Agentic YAML

Use this skill when building agentic workflows through DockPipe.

## Core Position

DorkPipe is the harness. DockPipe remains the governed runtime. Agentic behavior should be declared
in YAML and materialized into artifacts by package scripts.

AI workflows must beat one strong direct worker on quality, safety, cost, review effort, rerun value,
or traceability. Use orchestration for breadth, authority split, validation, isolation, cost control,
or targeted reruns; collapse coherent single-worker work back to one strong lane.

## Contract Shape

- `model_policy` expresses attempt, validation, and escalation intent.
- `steps[].agent.startup_prompt` declares starting instructions.
- `steps[].agent.include_agents_md` controls repo context inclusion.
- `steps[].agent.access.read/write/deny` declares access policy.
- `steps[].agent.orchestration` declares request, tasks, merge, and verify.
- Sibling `agents.yml` defines reusable role agents: who, authority, worker profile, model, access
  defaults, and standing constraints. Inline `agent.orchestration.agents` is for workflow-local
  overrides.
- `tasks[].agent` references a role agent. `tasks[].brief` and `tasks[].context` define this
  workflow's work item.
- `tasks[].context` fields are required briefing/discovery hints, not a hard source allowlist. Use
  access policy and mounts for the exploration boundary.
- `shared[].starting_points` are optional collector seeds. Do not duplicate broad mounts there or
  turn source discovery into a fixed checklist.
- `tasks[].materialize_outputs` lets one role worker produce a structured multi-file bundle that
  DorkPipe splits into deterministic task artifacts before validation/apply.

DorkPipe owns lane planning, budget/halt artifacts, task/result artifacts, validation hooks,
DAG/node evaluation signals, and follow-up rerun mechanics. Users should express intent, mounts,
context, access, outputs, and domain guardrails rather than meta-justification boilerplate.

## Hard Rules

- Do not create one shell script per workflow shape.
- Do not hide cost escalation; require policy and approval boundaries.
- Do not stuff every possible source path into every task. Give roles compact briefing artifacts and
  tell them to inspect additional allowed sources when needed.
- Do not split a coherent task across workers unless the split has distinct role authority, context,
  output contract, validation surface, or rerun value.
- Prefer role-shaped workers over one worker per file. Use materialized outputs when one role owns a
  coherent cluster but apply still needs exact per-file artifacts.
- Keep output artifacts reviewable.
- Treat local models as cheap attempt lanes and cloud models as governed spend lanes.
- Record requested lane, actual lane, token use, fallback/halt state, and escalation reason.
