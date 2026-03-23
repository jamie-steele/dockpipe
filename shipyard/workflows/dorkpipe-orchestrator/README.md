# dorkpipe-orchestrator

**DorkPipe** — DAG orchestration on top of DockPipe (`lib/dorkpipe/`, binary `bin/dorkpipe`).

- **Spec:** `spec.example.yaml` (nodes: shell → Ollama → optional Codex escalation).
- **Compose:** `templates/core/bundles/dorkpipe/assets/compose/docker-compose.yml` for Postgres+pgvector and Ollama.
- **Run directly:** `bin/dorkpipe run -f shipyard/workflows/dorkpipe-orchestrator/spec.example.yaml --workdir .`
- **Run via DockPipe workflow:** `dockpipe --workflow dorkpipe-orchestrator --workdir . --` (host script invokes `bin/dorkpipe`).

Principles: deterministic prep, local-first, parallel levels, pgvector when you add those nodes, Codex on escalation (or explicit non-escalate codex nodes). See **`lib/dorkpipe/README.md`** and **`docs/dorkpipe.md`**.

**Self-analysis (repo → Cursor handoff):** **`shipyard/workflows/dorkpipe-self-analysis/`** — **container-isolated** step; **`dorkpipe-self-analysis-host`** if you need **skip_container**. Optional **Compose** sidecar: **`scripts/dorkpipe/dev-stack.sh`**. Writes **`.dockpipe/orchestrator-cursor-prompt.md`** and **`paste-this-prompt.txt`**.
