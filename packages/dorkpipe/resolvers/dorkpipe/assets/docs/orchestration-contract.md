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
dockpipe scope workflow <workflow-name> orchestrate
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
- `worker`
- `goal`
- `inputs`
- `constraints`
- `expected_output`
- `worker_type`
- `work_mode`
- `resolver_hint`
- `max_cloud_tokens`
- `depends_on`

Each `task.json` may also include:

- `requested_resolver_hint`
- `lane`
- `model_policy`
- `access`
- `worker_policy`

`worker` is the seeded execution profile selected by workflow authoring, such as `codex`, `claude`,
or `ollama`. DorkPipe expands that profile into lane defaults before planning while still keeping
the worker containers and provider boundaries separate. `worker_policy.mode: prefer` keeps the
profile as a scheduler preference; `require` turns it into a hard lane-family requirement without
forcing authors to drop down to resolver-specific task authoring.

`work_mode` controls how cloud worker prompts should treat mounted source paths:

- `artifact` is the default. Workers gather evidence and return `response.md` artifact content.
  Mounted source paths are treated as read-only, and approval-gated apply/promotion writes files later.
- `edit` is direct workspace mode. Codex or Claude may use normal repo-worker behavior, including
  source edits and validation, but only inside paths that are writable by both access policy and
  container mounts.

Use `artifact` for planning, synthesis, doc drafting, validation, and artifact generation. Use `edit`
only for tasks whose purpose is implementation or repair, and pair it with explicit writable mounts.

When `work_mode: edit` is used with managed DockPipe volume sessions, DorkPipe can request one of
two runtime-owned isolation strategies via `DORKPIPE_ORCH_EDIT_ISOLATION`:

- `serialized` keeps workers on the authoritative session volume but allows only one active edit
  lease at a time.
- `split-volume` provisions a per-worker cloned Docker volume, runs edits there, and applies the
  resulting Git patch back through a runtime-owned release step.

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

For targeted repair or refinement passes on an existing orchestration artifact root, DorkPipe also
supports follow-up mode through:

- `DORKPIPE_ORCH_FOLLOWUP_REQUEST`
- `DORKPIPE_ORCH_FOLLOWUP_GOAL`
- `DORKPIPE_ORCH_FOLLOWUP_TASK_IDS`

In follow-up mode, the planner keeps the full graph, reruns the selected worker task ids plus their
downstream dependents, and reuses untouched `tasks/<task-id>/result.json` artifacts from the
existing workspace. This enables bounded fix-up runs on the same managed session/workspace without
regenerating every worker result from scratch.

The baseline policy starts as a conservative cheap-first cascade:

- local lanes are preferred by default
- `DORKPIPE_ORCH_CLOUD_LANES=false` blocks automatic cloud lane selection
- when cloud lanes are enabled, cloud candidates must cross baseline score thresholds
- task-class gating can override cheap-first when the task is high-authority
- local model fit can be penalized when the selected host hardware is weak for that model
- historical metrics adjust lane scores only after a minimum sample count
- all gates and training adjustments are written into `lanes/plan.json`

Task-class gating should make weak local lanes effectively non-authoritative:

- extraction and inventory tasks can still favor local lanes
- architecture, routing, validation, and edit-mode repair tasks should strongly favor stronger lanes
- local model strings and host capacity should be considered together so a large Ollama model on a
  small laptop does not win just because it is local

Host-local capability hints can be supplied explicitly when needed:

- `DORKPIPE_ORCH_HOST_MEMORY_GB`
- `DORKPIPE_ORCH_HOST_CPU_CORES`
- `DORKPIPE_ORCH_LOCAL_ACCELERATION`
- `DORKPIPE_ORCH_LOCAL_HARDWARE_TIER`

When those hints are not provided, DorkPipe should still make a best-effort host-capability
assessment and record the resulting profile in `lanes/plan.json`.

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

- Codex and Claude auth mounts come from resolver scope fields such as `auth-dir`, `container-auth-dir`, and `auth-mount-mode`
- Workflows read those fields with `dockpipe scope resolver <name> <field>` instead of hardcoding provider auth paths
- API-key env vars declared by resolver profiles are still forwarded by the DockPipe runner

Before launching a `codex` or `claude` worker, DorkPipe runs an auth preflight. API-key env vars
pass immediately. Otherwise DorkPipe checks the host resolver auth files. If auth is missing and the
run has an interactive terminal, it asks whether to launch the provider login command:

- Codex: `codex login`
- Claude: `claude login`

Set `DORKPIPE_ORCH_AUTH_LOGIN_ON_MISSING=never` to fail fast without prompting, or `always` to run
the login command without asking. Non-interactive runs fail fast with the command to run manually.

Do not bake credentials into images or require a separate container login as the normal path.

## Logging modes

DorkPipe orchestration defaults to progress-oriented logs for long-running package-owned work:

- `DORKPIPE_ORCH_LOG_MODE=default` hides raw Codex, Claude, Docker Compose, and Ollama pull output
  behind progress lines and prints log tails on failure.
- `DORKPIPE_ORCH_LOG_MODE=minimal` is accepted as the quieter stable mode for package scripts; it
  currently matches `default` for worker/dev-stack wrappers.
- `DORKPIPE_ORCH_LOG_MODE=verbose` streams raw provider and Docker output for debugging.
- `DORKPIPE_ORCH_LOG_MODE=none` suppresses package-owned progress lines where possible.

Worker logs are stored with task artifacts, for example `tasks/<task-id>/worker.log`. Dev-stack
logs are stored under the DorkPipe package dev-stack state directory. Set
`DORKPIPE_DEV_STACK_LOG_MODE` to override only sidecar stack logging.

Cloud worker tooling should be consumer-selectable and image-backed. Set
`DORKPIPE_ORCH_CONTAINER_IMAGE_PACKAGES` in workflow `vars:` to generate a provider-specific
derived Docker image for Codex or Claude workers. The image tag is fingerprinted from provider, base
image, and normalized package list, so package changes naturally produce a different image. Keep
resolver images lean; workflows that need Python, Ruby, Go, .NET, or other heavier stacks should
declare those packages explicitly. The generated Dockerfile adds Microsoft's Debian package feed on
demand for `.NET` package names such as `dotnet-sdk-8.0`.

## Training metrics

DorkPipe records lane outcome metrics as JSONL:

```text
training/metrics.jsonl
dockpipe scope --package dorkpipe training/metrics.jsonl
```

Each line captures task id, selected lane, provider, status, confidence, token estimates, whether a
live model was used, and whether a budget halt occurred. This starts as observation data. Later,
DorkPipe can use these stats to weight lane selection without changing the DockPipe engine.
