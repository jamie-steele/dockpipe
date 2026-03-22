# DorkPipe orchestrator

DorkPipe is a **local-first reasoning orchestrator** built on DockPipe. It lives in **`lib/dorkpipe/`** and ships as **`bin/dorkpipe`** (see `Makefile` `build`).

## Model (aligned with design notes)

- **Workflows** in DockPipe remain the user-facing primitive; DorkPipe adds a **DAG spec** (YAML) for multi-stage reasoning.
- **Runtimes / resolvers** are unchanged — DorkPipe **invokes** the real `dockpipe` binary for tool/resolver steps.
- **Pipeline stages:** intake → plan → optional conditional retrieve → parallel workers → aggregate (multi-signal confidence) → optional Codex escalation.

## Modules

| Area | Package |
|------|---------|
| DAG + policy | `lib/dorkpipe/spec` |
| Validation | `lib/dorkpipe/planner` |
| Parallel levels | `lib/dorkpipe/scheduler` |
| Execution | `lib/dorkpipe/workers` |
| Confidence + escalation | `lib/dorkpipe/aggregator` |
| Compose (pgvector + Ollama) | `lib/dorkpipe/composegen` |
| Run loop | `lib/dorkpipe/engine` |
| Metrics summary | `lib/dorkpipe/eval` |
| Promotion hints | `lib/dorkpipe/promotion` |

## CLI

```bash
bin/dorkpipe run -f lib/dorkpipe/examples/full-bar.yaml --workdir /tmp/run
bin/dorkpipe compose -o docker-compose.dorkpipe.yml
bin/dorkpipe validate -f <spec.yaml>
bin/dorkpipe eval --workdir /tmp/run
bin/dorkpipe promote --workdir /tmp/run
```

## Policy and node knobs (high level)

| Mechanism | Purpose |
|-----------|---------|
| `policy.merge_weights` | Blend harmonic means into `calibrated`. |
| `policy.escalate_confidence_below` | Trigger Codex (`escalate_only`) when `calibrated` is low. |
| `policy.early_stop_calibrated_above` | After each level, stop remaining phase-1 nodes if aggregate is already high enough. |
| `policy.branch_judge` | Node id whose stdout JSON includes `"winner":"<id>"` for `branch_required` nodes. |
| `retrieve_if_calibrated_below` | Skip node when phase-1 aggregate is already ≥ threshold (conditional retrieval / workers). |
| `branch_required` | Run only if it matches the judge winner (requires `needs: [branch_judge]`). |
| `parallel_group` | Within a level, set `agreement` from dispersion of `node_self` across the group. |
| `kind: verifier` | Same as Ollama HTTP, but scores land in the **verifier** dimension (JSON `verifier` / `score` / `pass`). |

## Confidence model

Node results carry a **multi-signal vector**. The engine aggregates **per dimension** with a **harmonic mean** across **non-skipped** nodes, then forms **`calibrated`** via **weighted blend** (`policy.merge_weights`). Escalation compares **`calibrated`** to `escalate_confidence_below`. This is a **structured** step toward calibrated orchestration, not a full research calibration pipeline.

Artifacts: **`.dorkpipe/run.json`** (provenance, branch winner, skipped nodes) and **`.dorkpipe/metrics.jsonl`** (one line per successful run, schema v2: `early_stop`, `skipped_nodes`, …).

## Evaluation and promotion

- **`dorkpipe eval`** reads `.dorkpipe/metrics.jsonl` and prints average calibrated score, escalation rate, early-stop rate, and average skipped nodes per run.
- **`dorkpipe promote`** applies lightweight heuristics (high escalation, repeated `skip_reason` patterns) and prints human-readable suggestions. Use this as a starting point for “promote to asset / workflow fragment / resolver template.”

## Internal workflow

**`dockpipe-experimental/workflows/dorkpipe-orchestrator/`** runs the orchestrator via a host script (`skip_container`) so the same repo can dogfood **`bin/dorkpipe`** without changing DockPipe’s core engine.

**`dockpipe-experimental/workflows/dorkpipe-self-analysis/`** runs **`scripts/dorkpipe/run-self-analysis.sh`** inside an **isolated container** (`isolate: golang:1.25-bookworm`, workdir **`/work`**). **`dorkpipe-self-analysis-stack`** adds **host** steps before/after: **`dev-stack-ensure-up.sh`** (compose up) and **`dev-stack-maybe-down.sh`** (compose down unless **`DORKPIPE_DEV_STACK_AUTODOWN=0`**). **`scripts/dorkpipe/dev-stack.sh`** is the same compose driver used by that workflow. **`dorkpipe-self-analysis-host`** is the same analysis on the **host** when Docker is unavailable.

The DorkPipe DAG is **`spec.yaml`** (deterministic) or **`spec.combined.yaml`** (adds **Ollama** `refine`). Outputs include **`.dockpipe/orchestrator-cursor-prompt.md`**, **`.dockpipe/paste-this-prompt.txt`**, **`.dorkpipe/self-analysis/`**, and **`merge-paste-prompt.sh`** when using the combined spec.

## Reusable assets

Starter scripts and prompts live under **`templates/core/bundles/dorkpipe/`** (including **`prompts/`** under that tree).

## Research roadmap (still open-ended)

Stronger calibration (temperature scaling, ensemble judges), bounded **plan** search beyond two-branch competition, active retrieval policies, and a full offline benchmark harness with labeled tasks remain **outside** this core and can wrap these primitives in templates and CI.
