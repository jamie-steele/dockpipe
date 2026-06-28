# DorkPipe Orchestration Contract

This document defines the minimal orchestration primitive for DorkPipe:

- a declared task graph
- bounded task artifacts
- normalized worker result artifacts
- package-owned model lane catalog
- planner-selected model lanes
- training/evaluation metrics for lane choices
- merge and verifier artifacts
- explicit approval before apply/publish

This is the core primitive behind "agentic" orchestration. Marketing terms are secondary.

The primitive should be driven by workflow-owned declarative data such as YAML task specs. Shared
scripts should materialize and execute the contract; they should not hardcode one example workflow's
task graph.

## Artifact root

```text
bin/.dockpipe/workflows/<workflow-name>/dorkpipe/orchestrate/
```

## Core files

- `request.json`
- `plan.json`
- `task-graph.json`
- `cloud-usage.json`
- `halt.json`
- `lanes/plan.json`
- `training/metrics.jsonl`
- `shared/*`
- `tasks/<task-id>/task.json`
- `tasks/<task-id>/lane-selection.json`
- `tasks/<task-id>/prompt.md`
- `tasks/<task-id>/response.md`
- `tasks/<task-id>/result.json`
- `merge/result.json`
- `merge/final.md`
- `verify/result.json`
- `approval.md`

## Task artifact

Each `task.json` should define:

- `id`
- `goal`
- `inputs`
- `constraints`
- `expected_output`
- `worker_type`
- `resolver_hint`
- `max_cloud_tokens`
- `depends_on`

Each `task.json` may also include:

- `requested_resolver_hint`
- `lane`
- `model_policy`
- `access`

## Model lane catalog

DorkPipe owns model lane metadata in package assets:

```text
assets/model-lanes/catalog.yml
assets/model-lanes/baseline-policy.yml
```

A lane describes a usable execution path, not just a provider string:

- `id`
- `provider`
- `resolver_hint`
- local/cloud/free flags
- model/context metadata
- capabilities
- availability checks
- budget policy
- training/exploration hints

Local lanes such as Ollama are cheap attempt lanes. Cloud-backed lanes such as Codex CLI and Claude
CLI are governed spend lanes and must remain behind budget/halt policy.

The planner writes `lanes/plan.json` for each run and `tasks/<task-id>/lane-selection.json` for each
task. Explicit task hints can still be honored, but `auto` should resolve through `model_policy`,
task intent, lane availability, and training metadata.

The baseline policy starts as a conservative cheap-first cascade:

- local lanes are preferred by default
- `DORKPIPE_ORCH_CLOUD_LANES=false` blocks automatic cloud lane selection
- when cloud lanes are enabled, cloud candidates must cross baseline score thresholds
- historical metrics adjust lane scores only after a minimum sample count
- all gates and training adjustments are written into `lanes/plan.json`

## Worker result artifact

Each `result.json` should normalize to:

- `task_id`
- `status`
- `provider_requested`
- `provider_actual`
- `lane_id`
- `lane_selection`
- `used_live_model`
- `budget_halt`
- `estimated_input_tokens`
- `estimated_output_tokens`
- `estimated_total_tokens`
- `summary`
- `claims`
- `artifacts`
- `citations`
- `confidence`
- `issues`
- `next_actions`

## Merge / verify primitives

Merge and verify are contract stages, not provider-specific features:

- merge compares and synthesizes task outputs
- verify checks coverage, conflicts, and escalation risk

Resolvers such as `codex`, `claude`, and `ollama` specialize execution under this contract rather
than redefining it.

## Cloud budget primitive

For cloud-backed worker lanes such as `codex` and `claude`, DorkPipe should own a run-level budget
ledger and halt signal:

- `cloud-usage.json` tracks run-wide estimated token usage
- `halt.json` records why cloud execution stopped
- per-task `result.json` records estimated prompt/response token usage

This keeps spend governance in the orchestration contract instead of scattering it across provider
wrappers. Local lanes such as `ollama` can stay outside that budget.

Set `DORKPIPE_ORCH_LIVE_MODELS=false` to force fallback artifacts without calling live model
backends. This is useful for package tests, training-mode dry runs, and demos that should exercise
the artifact graph without spending cloud tokens.

Cloud CLI workers run inside their resolver containers by default (`DORKPIPE_ORCH_CONTAINERIZE_CLOUD=true`).
Users sign in normally on the host; DorkPipe passes that auth into the container at runtime:

- Codex mounts `DORKPIPE_ORCH_CODEX_AUTH_DIR`, or `CODEX_HOME`, or `~/.codex` to `/home/node/.codex`
- Claude mounts `DORKPIPE_ORCH_CLAUDE_AUTH_DIR`, or `CLAUDE_HOME`, or `~/.claude` to `/home/node/.claude`
- `DORKPIPE_ORCH_AUTH_MOUNT_MODE` controls mount mode (`rw` default, `ro` supported)
- API-key env vars declared by resolver profiles are still forwarded by the DockPipe runner

Do not bake credentials into images or require a separate container login as the normal path.

## Training metrics

DorkPipe records lane outcome metrics as JSONL:

```text
training/metrics.jsonl
bin/.dockpipe/packages/dorkpipe/training/metrics.jsonl
```

Each line captures task id, selected lane, provider, status, confidence, token estimates, whether a
live model was used, and whether a budget halt occurred. This starts as observation data. Later,
DorkPipe can use these stats to weight lane selection without changing the DockPipe engine.
