# DorkPipe (`dorkpipe.orchestrator`)

Go module **`dorkpipe.orchestrator`** — local-first orchestration **on top of** DockPipe: DAG specs, parallel levels, real workers (shell, `dockpipe` subprocess, Ollama HTTP, PostgreSQL/pgvector), aggregation + confidence, optional Codex escalation.

**Location:** **`packages/dorkpipe/lib/`** (sibling to **`resolvers/`** YAML in this maintainer pack). The root repo **`go.work`** includes this module next to **`dockpipe`**.

- **`spec/`** — YAML DAG schema  
- **`planner/`** — validation + cycle detection  
- **`scheduler/`** — parallel batches + escalation ordering  
- **`workers/`** — real execution (no stubs)  
- **`aggregator/`** — harmonic per dimension + weighted `calibrated` + escalation policy  
- **`eval/`** — summarize `bin/.dockpipe/packages/dorkpipe/metrics.jsonl`  
- **`promotion/`** — heuristic promotion hints from metrics + last `run.json`  
- **`composegen/`** — Postgres+pgvector (+ optional Ollama) compose file  
- **`cianalysis/`** — normalize CI scan outputs into DorkPipe findings artifacts  
- **`userinsight/`** — queue / normalize / review user guidance signals  
- **`handoff/`** — build AI-facing handoff documents and signal summaries  
- **`statepaths/`** — canonical DorkPipe artifact layout under `bin/.dockpipe/`  
- **`engine/`** — wires planner → scheduler → workers → aggregator  

CLI: **`make maintainer-tools`** (repo root) writes **`../bin/dorkpipe`** (next to this **`lib/`** tree). Run that path directly — it is **not** installed under **`src/bin/`**.

## Authoring note

When maintainer scripts in the DorkPipe package need generic DockPipe workflow context, use the shared core SDK instead of open-coding `command -v` lookups:

- **Shell:** use **`dockpipe get ...`** for plain context reads; bootstrap **`eval "$(dockpipe sdk)"`** only for shell-specific actions like **`dockpipe_sdk init-script`**
- **`src/core/assets/scripts/lib/repo-tools.ps1`**
- **`src/core/assets/scripts/lib/repo_tools.py`**
- **`src/core/assets/scripts/lib/repotools/repotools.go`**

That shared SDK surface prefers the real repo-local DockPipe build:

- **`src/bin/dockpipe`**

before falling back to `PATH`.

If a DorkPipe package script needs to invoke the DorkPipe tool itself, keep that resolution package-local via:

- **`packages/dorkpipe/resolvers/dorkpipe/assets/scripts/lib/dorkpipe-cli.sh`**

Does **not** replace DockPipe’s workflow engine; it **invokes** the `dockpipe` binary for resolver steps.

**Confidence:** per-node **vectors** → harmonic mean **per dimension** across nodes → **weighted `calibrated`** (see `policy.merge_weights`). Skipped nodes (branch, `retrieve_if`, `early_stop`) are excluded from the aggregate.

**Orchestration:** `policy.branch_judge` + `branch_required` on nodes (JSON `{"winner":"…"}` from judge); `retrieve_if_calibrated_below`; `policy.early_stop_calibrated_above`; `parallel_group` agreement within a level; `kind: verifier` (Ollama transport, `verifier` score in JSON). CLI: **`dorkpipe eval`**, **`dorkpipe promote`**.

Artifacts: **`bin/.dockpipe/packages/dorkpipe/run.json`**, **`bin/.dockpipe/packages/dorkpipe/metrics.jsonl`** (schema v2). Example DAG: **`examples/full-bar.yaml`** in this directory.

**DockPipe self-analysis:** the packaged workflows **`dorkpipe-self-analysis`** and **`dorkpipe-self-analysis-host`** run the analysis entrypoint in containerized or host mode. Optional local sidecar: **`packages/dorkpipe/resolvers/dorkpipe/assets/scripts/dev-stack.sh`**. Writes **`bin/.dockpipe/`** artifacts including **`bin/.dockpipe/packages/dorkpipe/`**.
