# dorkpipe-self-analysis-stack

**DockPipe YAML** drives the full lifecycle:

1. **`stack_up`** (host) — the DorkPipe host stack helper starts Postgres + Ollama and handles Ollama GPU setup when available.
2. **`self_analysis`** (container) — same as **`dorkpipe-self-analysis`**.
3. **`stack_down`** (host) — the DorkPipe stack helper brings compose down, skipped when **`DORKPIPE_DEV_STACK_AUTODOWN=0`**.

Use this when you want **one command** from YAML instead of running **`dev-stack.sh`** by hand.

```bash
make build
dockpipe --workflow dorkpipe-self-analysis-stack --workdir . --
```

Keep sidecars up after the run:

```bash
DORKPIPE_DEV_STACK_AUTODOWN=0 dockpipe --workflow dorkpipe-self-analysis-stack --workdir . --
```

Analysis-only (no compose lifecycle): **`dorkpipe-self-analysis`**. Host-only: **`dorkpipe-self-analysis-host`**.
Proxy-backed policy example: **`dorkpipe-self-analysis-stack-proxy`**.

**Isolation:** Sidecars run via compose on the **host**; **DorkPipe** still runs the analysis step in a **disposable Docker container** (same as non-stack). Agent rules: **`.cursor/rules/dockpipe-agents.mdc`**, **`AGENTS.md`** (repository analysis section).

Current shape:

- **stack up** goes through the package host script so Ollama can auto-enable Docker GPU access, prompt for remediation, and write the temporary GPU compose override when available
- **stack down** also goes through the package host script, with keepalive controlled by **`DORKPIPE_DEV_STACK_AUTODOWN`**
- the stack still exports **`DATABASE_URL`** and **`OLLAMA_HOST`** into the later DockPipe step through workflow vars

GPU note:

- **`DORKPIPE_DEV_STACK_GPU=auto`** is the default and will use NVIDIA when Docker can expose it to the Ollama container
- if the host has NVIDIA but Docker GPU access is missing, the workflow now offers the same remediation / CPU fallback prompt path used by Pipeon

Host endpoint note:

- the workflow currently exports **`host.docker.internal`** endpoints for the isolated analysis container
- on Linux engines that do not provide that alias automatically, override **`OLLAMA_HOST`** / **`DATABASE_URL`** as needed (for example the bridge IP you already use today)
