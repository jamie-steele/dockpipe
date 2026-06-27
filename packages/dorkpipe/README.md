# DorkPipe plugin (`dorkpipe` package)

All DorkPipe **maintainer** workflows for this repo live here under **`resolvers/<name>/`** — one directory per **`--workflow`** leaf ( **`config.yml`** + assets). Umbrella metadata: **`package.yml`**.

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

## Dev Control Plane

The DorkPipe dev stack is package-owned under
**`resolvers/dorkpipe/assets/compose/`**. It runs the persistent local services that orchestration
can share across CLI and app surfaces:

- **`dorkpipe-stack`**: MCP/control-plane container with built `dockpipe`, `dorkpipe`, and `mcpd`
- **`dorkpipe-mcp-proxy`**: loopback-only host MCP endpoint at **`http://127.0.0.1:8766/mcp`**
- **`postgres`**: pgvector-backed local state
- **`ollama`**: local model lane storage/runtime

Codex and Claude stay out of this persistent stack. They are resolver-backed worker lanes that run
ephemerally for bounded tasks, record artifacts, and exit.

**Detail:** **`lib/README.md`** (Go module); this tree is YAML + assets.
