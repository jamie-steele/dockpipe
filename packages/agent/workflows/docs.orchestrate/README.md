# docs.orchestrate

`docs.orchestrate` is an example workflow that consumes the orchestration primitive from declarative
YAML instead of redefining it in workflow-specific shell.

It treats the useful core as:

1. plan
2. task graph
3. worker task artifacts
4. merge
5. verify
6. approve

The workflow is sequential today, but the contract is DAG-shaped so later parallel execution can
reuse the same task, result, merge, and verifier artifacts.

## Stack lifecycle

The workflow brings up the DorkPipe dev stack before planning and tears it down after approval:

1. Postgres + pgvector
2. Ollama
3. orchestration planning/workers/merge/verify/approval
4. stack down

By default, this workflow leaves the sidecars running after approval so iterative CLI/app testing can
reuse the same reasoning stack. Override `DORKPIPE_DEV_STACK_AUTODOWN=1` when you want the workflow
to stop the stack at the end. The default local endpoints are:

- `OLLAMA_HOST=http://127.0.0.1:11434`
- `DATABASE_URL=postgresql://dorkpipe:dorkpipe@127.0.0.1:15432/dorkpipe`

The stack helper waits for Ollama and pulls the declared local model by default. Override
`DORKPIPE_DEV_STACK_PULL_MODEL=0` to skip model bootstrap, or set `DORKPIPE_DEV_STACK_OLLAMA_MODEL`
to use a different local model.

Cloud-backed Codex/Claude lanes are enabled by the workflow's governed policy and remain bounded by
the declared token budgets, halt marker, and approval gate.

## YAML-driven setup

The example is driven directly by `config.yml`.

The plan step carries an `agent.orchestration` block, and the worker steps carry `agent.task_id`.
The shared DorkPipe scripts read that YAML through injected workflow/step context and materialize
request, plan, task, merge, and verify artifacts without needing a workflow-specific sidecar spec.

## Artifact root

All orchestration artifacts land under:

`bin/.dockpipe/packages/dorkpipe/orchestrate/docs.orchestrate/`

Key files:

- `request.json`
- `plan.json`
- `task-graph.json`
- `cloud-usage.json`
- `halt.json`
- `tasks/<task-id>/task.json`
- `tasks/<task-id>/result.json`
- `merge/result.json`
- `merge/final.md`
- `verify/result.json`
- `approval.md`

## Worker model

Each worker step uses the same generic `scripts/dorkpipe/orchestrate-run-task.sh` contract.

Resolvers specialize execution:

- `ollama` for `repo_shape`
- `codex` for `package_contracts`
- `claude` for `safety_model`

If a backend is unavailable, the task records fallback output rather than pretending the worker ran
live.

## Cloud budget guardrail

`codex` and `claude` worker lanes now share a DorkPipe-owned cloud budget ledger:

- `DORKPIPE_ORCH_MAX_TOTAL_CLOUD_TOKENS` sets the run-wide estimate cap.
- `DORKPIPE_ORCH_MAX_TASK_CLOUD_TOKENS` sets the per-task estimate cap.
- `DORKPIPE_ORCH_STOP_ON_BUDGET_EXCEEDED=true` turns the ledger into a kill switch.

When a cloud task would exceed budget, DorkPipe writes `halt.json`, skips later cloud workers, and
surfaces the decision in `tasks/*/result.json`, `cloud-usage.json`, and `verify/result.json`.
