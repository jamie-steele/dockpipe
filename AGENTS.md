# AGENTS.md — dockpipe maintainer + agent guide

This file defines how this repo works. Follow it strictly.

---

## Core concept

DockPipe provides one primitive:

1. Spawn an isolated environment  
2. Run a command inside it  
3. Optionally act on the result  

Everything else is built around this.

**`dockpipe init`** is project-local scaffolding only: without a workflow name it creates or updates **`templates/`** (including merged **`templates/core/`** with **`runtimes/`**, **`resolvers/`**, **`strategies/`**, **`assets/`** — scripts, images, compose), **`scripts/`**, **`images/`**, in the **current directory**. **`dockpipe init <name>`** adds **`templates/<name>/config.yml`** as a **minimal empty workflow**; use **`--from <template>`** (e.g. **`init`**, **`run`**, **`run-apply`**) to copy a full bundled workflow tree. It does **not** clone Git repositories or bootstrap a remote project. See **`docs/cli-reference.md`** and **`docs/templates-core-assets.md`**.

---

## Architecture model (STRICT)

There are four core concepts:

---

### 1. Templates

User-facing workflows.

- Define **what happens**
- Contain YAML (`config.yml`), steps, and scripts
- Used directly in projects

Examples:
- `init`
- `run`
- `run-apply`
- `run-apply-validate`

📁 Location:

- In **this repository’s checkout:** **`src/templates/<name>/`** (bundled workflows + **`src/templates/core/`**).
- In a **downstream project** after **`dockpipe init`:** **`templates/<name>/`** at the project root (same layout conceptually).

**Do not** put **this repository’s** CI, demo, or internal automation workflows under **`src/templates/`**. Those belong under **`shipyard/workflows/<name>/`** (see **Internal workflows** below).

---

### 2. Runtimes

Execution substrates.

- Define **where execution runs**
- May contain implementation logic
- Must NOT be confused with resolvers

Current runtimes:
- `cli` → local execution
- `docker` → container execution
- `kube-pod` → Kubernetes pod/job execution

Future:
- `cloud-runner` (with providers like lambda/fargate)

📁 Location (under the authoring templates root — **`src/templates/core/`** in this repo, **`templates/core/`** after **`dockpipe init`**):
`…/core/runtimes/`

---

### 3. Resolvers

Tool/platform integrations.

- Define **what performs the work**
- Always tool/platform-specific

Examples:
- `claude`
- `codex`
- `code-server`
- `cursor-dev`
- `vscode`

❗ Resolvers are NOT runtimes.

📁 Location:
`…/core/resolvers/`

---

### 4. Strategies

Lifecycle wrappers.

- Modify execution behavior
- Examples: worktree, commit

📁 Location:
`…/core/strategies/`

---

## CRITICAL STRUCTURE RULE

**`…/core/`** (i.e. **`src/templates/core/`** here, **`templates/core/`** in a downstream project) contains ONLY category folders.

Valid:
…/core/
  runtimes/
  resolvers/
  strategies/
  assets/
    scripts/
    images/
    compose/

Invalid:
…/core/claude
…/core/docker
…/core/test

Rules:

- If it is a **tool/platform** → it MUST be inside `resolvers/`
- If it is an **execution environment** → it MUST be inside `runtimes/`
- No duplicates
- No exceptions

---

## Extension model

DockPipe is extended via:

- templates (workflows)
- runtimes (execution backends)
- resolvers (tools)
- strategies (lifecycle)

NOT via:

- plugins
- core branching
- special-case flags

---

## Template development rule (IMPORTANT)

When working on templates:

You are a **user of DockPipe**, not modifying the engine.

Allowed:
- YAML workflows
- scripts
- images
- documentation

NOT allowed:
- modifying `src/lib/` or `src/cmd/` (core Go) without a general primitive
- adding template-specific logic to core

If something cannot be done:

→ propose a **general primitive**, not a special case

---

## Internal workflows (this repository)

When you work **on the dockpipe project itself**, you are a **user** of the tool: extend it via **`src/templates/`**, **`scripts/`**, and **`shipyard/workflows/`** — **not** by stuffing internal pipelines into **`src/templates/`**.

| Location | Purpose |
|----------|---------|
| **`src/templates/<name>/`** (this repo) / **`templates/<name>/`** (downstream) | **User-facing** workflow examples shipped in the bundle (**`run`**, **`run-apply`**, **`run-apply-validate`**, **`init`**, …). Reusable for any downstream project. |
| **`shipyard/workflows/<name>/`** | **This repo only:** CI, recordings, experiments — workflows that exist to run **dockpipe** on **this** codebase. Not installed by a special `init` flag; copy dirs or use **`dockpipe init &lt;name&gt; --from …`** pointing at a workflow path. |

**Preferred pattern:** `dockpipe init <name> --from run-apply` or **`run-apply-validate`** (or **`run`**, **`blank`**) for user-shaped scaffolds; keep automation-specific YAML under **`shipyard/workflows/`**.

**Accelerator (maintainers):** After **`make build`**, **`make self-analysis`**, **`make self-analysis-host`**, or **`make self-analysis-stack`** run the DorkPipe self-analysis workflows on this repo (container, host-only, or compose stack). See **`docs/dorkpipe.md`** and **`shipyard/workflows/dorkpipe-self-analysis/README.md`**.

### Agent guidance (this repository)

**Cursor / IDE:** **`.cursor/rules/dockpipe-agents.mdc`** (**`alwaysApply: true`**) mirrors this; keep it in sync.

**Two channels — do not conflate them:**

1. **On-disk context** — **`.dockpipe/`** and **`.dorkpipe/`** hold **generated** handoffs, self-analysis facts, CI bundles, metrics, and optional insights. Use them as **read-only grounding** with normal code reading. Pointers: **`docs/compliance-ai-handoff.md`**, **`docs/dorkpipe-ci-signals.md`**, **`docs/user-insight-queue.md`**. Pipeon: **`src/bin/pipeon`**, **`src/pipeon/docs/`**. Do **not** auto-regenerate; refresh only when the **user** asks (then **`make self-analysis*`** / **`dorkpipe-self-analysis`** — see **Accelerator** above).

2. **MCP (`mcpd`)** — **Bounded tools** with **tiered IAM** (`DOCKPIPE_MCP_TIER`: `readonly` → `validate` → `exec`) via **`src/lib/mcpbridge`**. Default tier is **`validate`** (list + validate; **no** `dockpipe.run` / `dorkpipe.run_spec`). Tier **`exec`** (or legacy **`DOCKPIPE_MCP_ALLOW_EXEC=1`** when tier is unset) is required for run tools. See **`docs/mcp-agent-trust.md`** and **`docs/mcp-architecture.md`**.

**Freshness:** If artifacts exist, say whether they look current vs **`HEAD`**; if missing or stale, say so and suggest refresh **when relevant**.

**Scaffold note:** **`dockpipe init … --from dorkpipe-self-analysis`** appends a handoff section to **`AGENTS.md`** in new projects (marker **`<!-- dockpipe: self-analysis handoff -->`**).

---

## Milestone: we run the tool on ourselves

**Dogfooding** is not a CLI feature or a built-in primitive — it is **how we build confidence in the product.**

We run the **same released binary** and **declarative workflows** in **this repository’s CI** (see **`.github/workflows/ci.yml`**) that users get: multi-step **`steps:`**, Docker isolation, resolver/runtime wiring. That alignment is a **deliberate quality bar** — talk it up in release notes and blog posts — but it lives in **automation and culture**, not in extra flags on **`dockpipe init`**.

---

## Core constraints

DO:
- keep core minimal
- keep concepts separate
- prefer scripts over core changes

DO NOT:
- mix runtimes and resolvers
- introduce vendor-specific logic into core
- turn this into a workflow engine
- add orchestration complexity to core

---

## Mental model (memorize this)

- Template → what happens  
- Runtime → where it runs  
- Resolver → what tool runs  
- Strategy → how it wraps execution  

---

## One-line philosophy

DockPipe runs anything, anywhere, in isolation.

Keep it simple. Keep it composable.