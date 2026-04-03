# DorkPipe (`dorkpipe.orchestrator`)

Go module **`dorkpipe.orchestrator`** — local-first orchestration **on top of** DockPipe: DAG specs, parallel levels, real workers (shell, `dockpipe` subprocess, Ollama HTTP, PostgreSQL/pgvector), aggregation + confidence, optional Codex escalation.

**Location:** **`packages/dorkpipe/lib/`** (sibling to **`resolvers/`** YAML in this maintainer pack). The root repo **`go.work`** includes this module next to **`dockpipe`**.

- **`spec/`** — YAML DAG schema  
- **`planner/`** — validation + cycle detection  
- **`scheduler/`** — parallel batches + escalation ordering  
- **`workers/`** — real execution (no stubs)  
- **`aggregator/`** — harmonic per dimension + weighted `calibrated` + escalation policy  
- **`eval/`** — summarize `.dorkpipe/metrics.jsonl`  
- **`promotion/`** — heuristic promotion hints from metrics + last `run.json`  
- **`composegen/`** — Postgres+pgvector (+ optional Ollama) compose file  
- **`engine/`** — wires planner → scheduler → workers → aggregator  

CLI: **`make maintainer-tools`** (repo root) writes **`../bin/dorkpipe`** (next to this **`lib/`** tree). Run that path directly — it is **not** installed under **`src/bin/`**.

Does **not** replace DockPipe’s workflow engine; it **invokes** the `dockpipe` binary for resolver steps.

**Confidence:** per-node **vectors** → harmonic mean **per dimension** across nodes → **weighted `calibrated`** (see `policy.merge_weights`). Skipped nodes (branch, `retrieve_if`, `early_stop`) are excluded from the aggregate.

**Orchestration:** `policy.branch_judge` + `branch_required` on nodes (JSON `{"winner":"…"}` from judge); `retrieve_if_calibrated_below`; `policy.early_stop_calibrated_above`; `parallel_group` agreement within a level; `kind: verifier` (Ollama transport, `verifier` score in JSON). CLI: **`dorkpipe eval`**, **`dorkpipe promote`**.

Artifacts: **`.dorkpipe/run.json`**, **`.dorkpipe/metrics.jsonl`** (schema v2). Example DAG: **`examples/full-bar.yaml`** in this directory.

**DockPipe self-analysis:** **`dockpipe/dorkpipe/resolvers/dorkpipe-self-analysis/`** runs the script **in a container** (isolated). Optional **Compose** sidecar: **`scripts/dorkpipe/dev-stack.sh`**. Writes **`.dockpipe/`** + **`.dorkpipe/`** artifacts.
