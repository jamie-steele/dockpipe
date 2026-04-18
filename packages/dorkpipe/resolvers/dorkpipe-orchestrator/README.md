# dorkpipe-orchestrator

**DorkPipe** — DAG orchestration on top of DockPipe (`lib/dorkpipe/`, binary `bin/dorkpipe`).

- **Spec:** `spec.example.yaml` (nodes: shell → Ollama → optional Codex escalation).
- **Compose:** `packages/dorkpipe/resolvers/dorkpipe/assets/compose/docker-compose.yml` for Postgres+pgvector and Ollama.
- **Run directly:** `packages/dorkpipe/bin/dorkpipe run -f packages/dorkpipe/resolvers/dorkpipe-orchestrator/spec.example.yaml --workdir .`
- **Run via DockPipe workflow:** `dockpipe --workflow dorkpipe-orchestrator --workdir . --` (host script `scripts/dorkpipe-orchestrator/run-orchestrator.sh` invokes `bin/dorkpipe`).

Principles: deterministic prep, local-first, parallel levels, pgvector when you add those nodes, Codex on escalation (or explicit non-escalate codex nodes). See **`lib/README.md`** (Go module).

**Self-analysis (repo → Cursor handoff):** use the packaged workflows **`dorkpipe-self-analysis`** or **`dorkpipe-self-analysis-host`**. Optional local sidecar: **`packages/dorkpipe/resolvers/dorkpipe/assets/scripts/dev-stack.sh`**. Writes **`bin/.dockpipe/orchestrator-cursor-prompt.md`** and **`bin/.dockpipe/paste-this-prompt.txt`**.
## Authoring note

Resolver host scripts in this package should use the shared core SDK:

- **Shell:** use **`dockpipe get ...`** for plain context reads; bootstrap **`eval "$(dockpipe sdk)"`** only for shell-specific actions like **`dockpipe_sdk init-script`**
- **DorkPipe package CLI:** use the package-local helper under
  **`packages/dorkpipe/resolvers/dorkpipe/assets/scripts/lib/dorkpipe-cli.sh`**
  when a script in this package needs to invoke the DorkPipe tool itself.
