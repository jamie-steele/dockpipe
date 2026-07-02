# Model Escalation

Read when working on agentic AI workflows, DorkPipe orchestration, local/cloud model policy, or token budgets.

## Position

DockPipe provides governed runtime primitives. DorkPipe owns AI harness behavior through package YAML/assets/scripts.
Codex, Claude, and Ollama are targets/resolvers/adapters, not DockPipe core concepts.

## Policy Surface

Use YAML to declare:

- `model_policy.mode`
- cheap/local attempt preference
- max attempts
- validation preference
- escalation triggers
- approval over cost
- role/task cloud token budgets
- halt behavior

For AI workflow value, lane routing, and direct-worker baseline rules, read
`docs/agents/ai-workflow-value-bar.md` first. This file only covers model escalation mechanics.

Treat a model as an escalation lane, not only a provider/model string. A lane can represent a local
Ollama model, a CLI-backed Codex/Claude agent, or another package-owned adapter. The lane definition
should include enough metadata for DorkPipe and tooling to answer:

- is it local or cloud-backed?
- what capabilities does it provide?
- what context window or task shape is it suited for?
- is it installed/available when the stack starts?
- what budget/halt rules apply before DorkPipe can use it?

Workflow authoring should normally select these lanes through reusable role definitions in
`agents.yml`, not by repeating provider choices on every task. Role definitions carry seeded worker
profiles such as `worker: ollama`, `worker: codex`, and `worker: claude`; task entries reference
roles with `agent:` and carry only workflow-specific goal, brief, context, dependencies, and outputs.
If a role must stay on one worker class, declare `worker_policy.mode: require`; otherwise use
`prefer` so DorkPipe can still compare, fall back, or escalate through the lane catalog.

## Hard Rules

- Local models such as Ollama can be cheap/default attempt lanes.
- Cloud-backed lanes such as Codex/Claude need budget ledgers and halt markers.
- Escalate by work class, not provider habit: inventory/extraction can be cheap only when the lane
  receives deterministic source packets or has real tool access; source selection, architecture,
  validation, and edit repair need stronger lanes when they affect durable output.
- Record requested lane, actual lane, token use, fallback state, and escalation reason in artifacts.
- Do not hide escalation behind provider wrappers.
- Do not hardcode provider behavior in `src/lib/` or `src/cmd/`.
- Human approval stays explicit before promotion/apply/publish.
- Model browser and template designer UX must round-trip through workflow YAML and package-owned
  catalogs; extension-local state is only a draft/cache layer.

## Current DorkPipe Artifacts

- `lanes/plan.json`
- `tasks/<task-id>/lane-selection.json`
- `training/metrics.jsonl`
- `cloud-usage.json`
- `halt.json`
- `tasks/<task-id>/result.json`
- `merge/result.json`
- `verify/result.json`
- `approval.md`

## Canonical Docs

- `docs/agentic-workflows.md`
- `docs/workflow-yaml.md`
- `packages/dorkpipe/resolvers/dorkpipe/assets/docs/orchestration-contract.md`

## Training Mode

Use `DORKPIPE_ORCH_TRAINING_MODE=observe` to collect lane outcome metrics without changing selection
behavior. Use `DORKPIPE_ORCH_LIVE_MODELS=false` for dry runs and tests that should exercise planner,
budget, and artifact plumbing without calling live model backends.
