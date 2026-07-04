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
7. apply approved source-tree changes

For repo guidance work, use the package-owned baseline under
`packages/dorkpipe/resolvers/dorkpipe/assets/docs/example-brain/` before writing repo-specific
brain docs. Durable consumer output should read like native repo guidance, not runtime commentary.

The workflow runs bounded worker tasks through the package-owned scheduler. The contract remains
DAG-shaped so dependency handling, richer splitters, and additional worker lanes can reuse the same
task, result, merge, and verifier artifacts.

## Stack lifecycle

The workflow brings up the DorkPipe dev stack before planning and tears it down from `finally:`:

1. Postgres + pgvector for persistent orchestration memory
2. Ollama for local model lanes
3. `dorkpipe-stack` for the MCP/control-plane process
4. `dorkpipe-mcp-proxy` for a loopback-only local MCP endpoint
5. orchestration planning/workers/merge/verify/approval/apply
6. stack down

By default, this workflow tears the sidecars down at the end like a normal one-shot DockPipe run.
Override `DORKPIPE_DEV_STACK_AUTODOWN=0` when you want iterative CLI/app testing to reuse the same
reasoning stack between runs. The default local endpoints are:

- `MCP_HTTP_URL=http://127.0.0.1:8766/mcp`
- `OLLAMA_HOST=http://127.0.0.1:11434`
- `DATABASE_URL=postgresql://dorkpipe:dorkpipe@127.0.0.1:15432/dorkpipe`

The stack helper waits for Ollama and pulls the declared local model by default. Override
`DORKPIPE_DEV_STACK_PULL_MODEL=0` to skip model bootstrap, or set `DORKPIPE_DEV_STACK_OLLAMA_MODEL`
to use a different local model.

Cloud-backed Codex/Claude lanes are enabled by the workflow's governed policy and remain bounded by
the declared token budgets, halt marker, and approval gate.

Codex and Claude are not long-lived stack services. They remain ephemeral resolver lanes: DorkPipe
starts them for bounded worker tasks, captures their artifacts, and lets their containers exit.

## Apply behavior

After approval, the workflow copies the merged synthesis into the checkout as:

`docs/workflows/dorkpipe-orchestration-synthesis.md`

This is intentionally an uncommitted source-tree change. It lets CLI-first orchestration prove the
write path without creating commits or hiding changes in generated artifact directories.

## YAML-driven setup

The example is driven directly by `config.yml`.

The plan step carries an `agent.orchestration` block, and the worker steps carry `agent.task_id`.
The shared DorkPipe scripts read that YAML through injected workflow/step context and materialize
request, plan, task, merge, and verify artifacts without needing a workflow-specific sidecar spec.

That same pattern should seed deterministic consumer guidance first, then layer repo-specific facts
on top. Do not let workflow mechanics become the consumer repo's durable vocabulary.

## Artifact root

All orchestration artifacts land under:

`dockpipe scope workflow docs.orchestrate orchestrate`

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
