# DorkPipe plugin (`dorkpipe` package)

The `dorkpipe` package is the consumer-facing agentic stack. It ships DorkPipe workflows, MCP sidecar
assets, packaged helper binaries, and the local control-plane compose contract without requiring a
DockPipe source checkout in the consuming repo.

| Resolver / workflow | Role |
|---------------------|------|
| **`compliance-handoff/`** | CI + self-analysis signal pointers |
| **`dorkpipe-orchestrator/`** | Example DAG / host orchestrator |
| **`dorkpipe-self-analysis/`** | Container self-analysis |
| **`dorkpipe-self-analysis-host/`** | Host-only variant |
| **`dorkpipe-self-analysis-stack/`** | Compose sidecar + analysis |
| **`dorkpipe/`** | Small domain **`config.yml`** pack (namespace wiring) |
| **`user-insight-process/`** | Host workflow: queue → **`insights.json`** — **`resolvers/user-insight-process/README.md`** |
| **`workflows/skills.render/`** | Package workflow that renders curated DorkPipe skills to target-specific local formats |
| **`workflows/orchestrate.stack/`** | Package workflow wrapper that owns stack lifecycle around the standard orchestration flow |

Go code for the orchestrator lives in **`lib/`** (module **`dorkpipe.orchestrator`**) — this tree is **YAML + assets** only (not the Go module).

## Skills Renderer

Curated DorkPipe skill sources live under **`resolvers/dorkpipe/assets/skills/`**. Render them
through the package workflow:

```bash
dockpipe --package dorkpipe --workflow skills.render -- --target codex
dockpipe --package dorkpipe --workflow skills.render -- --target claude
```

Codex defaults to **`~/.codex/skills/<skill-name>/SKILL.md`**. Claude defaults to
**`~/.claude/skills/<skill-name>/SKILL.md`**. Target names are adapters; skill ids in
**`AGENTS.md`** and **`docs/agents/index.yaml`** stay target-independent.

## Agentic Orchestration Lanes

DorkPipe owns the agentic model-lane catalog under
**`resolvers/dorkpipe/assets/model-lanes/catalog.yml`**. The planner materializes lane choices to
**`lanes/plan.json`** and **`tasks/<task-id>/lane-selection.json`**, then worker execution records
outcome metrics in **`training/metrics.jsonl`**. This keeps model escalation package-owned and
YAML-driven: Ollama can be the cheap local lane, while Codex CLI and Claude CLI remain governed
cloud lanes behind budget/halt policy.

Set **`DORKPIPE_ORCH_LIVE_MODELS=false`** for dry runs and tests that should exercise the full
artifact graph without calling live model backends.

When Codex or Claude lanes run in containers, DorkPipe seeds the matching host skill directory into
the worker home before launching the CLI: **`~/.codex/skills`** for Codex and **`~/.claude/skills`**
for Claude. DorkPipe now stages a run-local merged skills directory first: it copies the user's
existing provider skills, then overlays the curated DorkPipe rendered skills for that provider, and
mounts the merged result read-only into the worker. This keeps user skills available while making
the DorkPipe routing/script skills deterministic for every run. Set
**`DORKPIPE_ORCH_CONTAINER_SKILLS=never`** to disable skill propagation, or set
**`DORKPIPE_ORCH_CODEX_SKILLS_DIR`**, **`DORKPIPE_ORCH_CLAUDE_SKILLS_DIR`**, or
**`DORKPIPE_ORCH_SKILLS_DIR`** to override the base host skill source that gets merged before the
curated overlay.

## Dev Control Plane

The DorkPipe stack is package-owned under **`resolvers/dorkpipe/assets/`**. The default path uses
compiled package assets, including packaged helper binaries under **`assets/tooling/bin/`**, so a
consumer repo can depend on `dorkpipe` and run the stack from installed/extracted package material.
It runs the persistent local services that orchestration can share across CLI and app surfaces:

- **`dorkpipe-stack`**: MCP/control-plane container with built `dockpipe`, `dorkpipe`, and `mcpd`
- **`dorkpipe-mcp-proxy`**: loopback-only host MCP endpoint at **`http://127.0.0.1:8766/mcp`**
- **`postgres`**: pgvector-backed local state
- **`ollama`**: local model lane storage/runtime

Codex and Claude stay out of this persistent stack. They are resolver-backed worker lanes that run
ephemerally for bounded tasks, record artifacts, and exit.

Maintainer-only local rebuild behavior is explicit:

- compile packaged consumer artifacts: `dockpipe package compile resolvers --workdir . --from packages/dorkpipe --force`
- or opt into checkout binaries for the stack only: `DORKPIPE_DEV_STACK_BUNDLE_MODE=checkout scripts/dorkpipe/dev-stack.sh up`

GPU policy is explicit and workflow-safe:

- `DORKPIPE_DEV_STACK_GPU=auto|cpu|nvidia|all`
- `DORKPIPE_DEV_STACK_GPU_SETUP=never|auto|prompt`
- `DORKPIPE_DEV_STACK_GPU_ON_FAILURE=cpu|fail`

Workflow callers should set the policy they want in YAML/env. The packaged stack workflows use
`GPU=auto`, `GPU_SETUP=never`, and `GPU_ON_FAILURE=cpu`, so automation never prompts and never
advances unless the requested services are actually up.

## Package Workflow Wrapper

Consumers that want DorkPipe to own stack lifecycle can call the package workflow:

```yaml
steps:
  - id: plan
    workflow: orchestrate.stack
    package: dockpipeproject
    agent:
      orchestration:
        # caller-owned request/plan/tasks/merge/verify/apply block
```

By default the wrapper plans against the caller workflow config and the caller step id `plan`, then
runs stack up and stack down internally with `finally:`. If the orchestration declaration lives on a
different caller step, set `DORKPIPE_ORCH_SOURCE_STEP_ID` on the packaged workflow step.

**Detail:** **`lib/README.md`** (Go module); this tree is YAML + assets.
### Follow-up repair mode

`orchestrate.stack` can reuse an existing orchestration workspace for a targeted repair pass instead
of rerunning the full worker graph from scratch.

Set:

- `DORKPIPE_ORCH_FOLLOWUP_REQUEST` to describe the correction you want
- `DORKPIPE_ORCH_FOLLOWUP_GOAL` to override the planner goal for the repair pass
- `DORKPIPE_ORCH_FOLLOWUP_TASK_IDS` as a comma-separated list of orchestration task ids to rerun

Behavior:

- selected tasks rerun
- downstream dependent tasks rerun automatically
- untouched task results are reused from the existing artifact root
- merge, verify, approval, and apply consume the mixed set of reused and fresh artifacts

This is intended for repair or refinement passes on the same managed session/workspace, especially
when the original run already produced mostly-correct artifacts.
