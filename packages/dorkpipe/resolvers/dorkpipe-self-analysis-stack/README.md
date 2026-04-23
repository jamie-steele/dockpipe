# dorkpipe-self-analysis-stack

**DockPipe YAML** drives the full lifecycle:

1. **`stack_up`** (host) — DockPipe **`compose_up`** host builtin against the DorkPipe compose file.
2. **`self_analysis`** (container) — same as **`dorkpipe-self-analysis`**.
3. **`stack_down`** (host) — DockPipe **`compose_down`** host builtin, skipped when **`DORKPIPE_DEV_STACK_AUTODOWN=0`**.

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

**Isolation:** Sidecars run via compose on the **host**; **DorkPipe** still runs the analysis step in a **disposable Docker container** (same as non-stack). Agent rules: **`.cursor/rules/dockpipe-agents.mdc`**, **`AGENTS.md`** (repository analysis section).

Current shape:

- **compose up** uses the DockPipe-owned Compose primitive from workflow YAML
- **compose down** also uses the DockPipe-owned Compose primitive, with keepalive controlled by **`compose.autodown_env`**
- the stack also exports **`DATABASE_URL`** and **`OLLAMA_HOST`** into the later DockPipe steps through **`compose.exports`**

Host endpoint note:

- the workflow currently exports **`host.docker.internal`** endpoints for the isolated analysis container
- on Linux engines that do not provide that alias automatically, override **`OLLAMA_HOST`** / **`DATABASE_URL`** as needed (for example the bridge IP you already use today)
