# dorkpipe-self-analysis-stack

**DockPipe YAML** drives the full lifecycle:

1. **`stack_up`** (host) — `docker compose up -d` for Postgres + Ollama (`scripts/dorkpipe/dev-stack.sh`).
2. **`self_analysis`** (container) — same as **`dorkpipe-self-analysis`**.
3. **`stack_down`** (host) — `docker compose down` **unless** **`DORKPIPE_DEV_STACK_AUTODOWN=0`**.

Use this when you want **one command** from YAML instead of running **`dev-stack.sh`** by hand.

```bash
make build
./bin/dockpipe --workflow dorkpipe-self-analysis-stack --workdir . --
```

Keep sidecars up after the run:

```bash
DORKPIPE_DEV_STACK_AUTODOWN=0 ./bin/dockpipe --workflow dorkpipe-self-analysis-stack --workdir . --
```

Analysis-only (no compose lifecycle): **`dorkpipe-self-analysis`**. Host-only: **`dorkpipe-self-analysis-host`**.

**Isolation:** Sidecars run via compose on the **host**; **DorkPipe** still runs the analysis step in a **disposable Docker container** (same as non-stack). Agent rules: **`.cursor/rules/dockpipe-agents.mdc`**, **`AGENTS.md`** (repository analysis section).
