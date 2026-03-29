# AGENTS.md — dockpipe maintainer + agent guide

This file defines how this repo works. Follow it strictly.

---

## Core concept

DockPipe’s **engine** has one core **action** (spawn → run → act). Separately, **capabilities** (dotted ids like **`cli.codex`**) and **resolver** packages are documented in **`docs/capabilities.md`** — not the same word as this three-step loop.

1. Spawn an isolated environment  
2. Run a command inside it  
3. Optionally act on the result  

Everything else is built around this.

**Package model (store vs tree):** Installed store artifacts are headed for **`.dockpipe/internal/packages/`** (workflows, **core** slices, assets); authoring stays under **`templates/`** and repo-root **`workflows/`**. See **`docs/package-model.md`**.

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

- In **this repository’s checkout:** **`src/core/workflows/<name>/`** (bundled example workflows) alongside **`src/core/`** category dirs (**`runtimes/`**, **`resolvers/`**, **`strategies/`**, **`assets/`**).
- In a **downstream project** after **`dockpipe init`:** prefer repo-root **`workflows/<name>/`**; legacy **`templates/<name>/`** remains supported.

**Do not** put **this repository’s** CI, demo, or internal automation workflows under **`src/core/workflows/`**. Those belong under repo-root **`workflows/<name>/`** (lean CI / dogfood) or **`.staging/workflows/<name>/`** (maintainer packaging and experiments) (see **Internal workflows** below).

**Package IDs:** Workflow and resolver names may use dotted namespaces (e.g. **`acme.workflow.ci`**, **`acme.resolver.custom`**) when the directory name under **`workflows/`** or **`src/core/workflows/`** matches (same rules as **`--workflow`** resolution).

---

### 2. Runtimes

Execution substrates.

- Define **where execution runs**
- May contain implementation logic
- Must NOT be confused with resolvers

Current runtimes (substrates):
- `dockerimage` → container from a pre-built image; host-only steps use **`skip_container: true`** (legacy YAML may say `cli` / `powershell` / `cmd` — those normalize to **`dockerimage`**)
- `dockerfile` → container built from a Dockerfile
- `package` → nest a namespaced workflow (`resolver:` + `package:`)

Not runtimes: Kubernetes, cloud APIs, Terraform — use **`dockerimage`** / **`dockerfile`** plus scripts and resolvers.

📁 Location (under the authoring core root — **`src/core/`** in this repo, **`templates/core/`** after **`dockpipe init`**):
`…/core/runtimes/`

---

### 3. Resolvers

**Workflow-local tooling profiles** — **`DOCKPIPE_RESOLVER_*`** (and optional delegate **`config.yml`**) under **`…/core/resolvers/<name>/`**, or maintainer trees under **`.staging/workflows/<name>/profile`** in this repo (same layout, merged into **`packages/resolvers/`** when compiled). You choose **`resolver:`** / **`default_resolver:`** in YAML; there is **no** separate “capability” indirection in the runner. **Packaged** workflows can pull in **additional** resolver packages (e.g. a future **`dockpipe.cloud.aws`** pack) via **`package.yml`** / store installs.

- Define **what performs the work** for that workflow or package slice
- Always tool/platform-specific

Examples (profile names):
- `claude`
- `codex`
- `code-server`
- `cursor-dev`
- `vscode`

❗ Resolvers are NOT runtimes. **Runtime** = where (see **`…/core/runtimes/README.md`**); **resolver** = which tool profile.

📁 Location:
`…/core/resolvers/` (bundled) · **`.staging/workflows/<tool>/`** (maintainer overlays in this repo)

---

### 4. Strategies

Lifecycle wrappers.

- Modify execution behavior
- Examples: worktree, commit

📁 Location:
`…/core/strategies/`

---

## CRITICAL STRUCTURE RULE

**`…/core/`** (i.e. **`src/core/`** here — category dirs plus **`workflows/`** for bundled examples — and **`templates/core/`** in a downstream project after init) contains ONLY category folders **and** (in this repo only) **`workflows/`** for shipped examples.

Valid (this repo **`src/core/`**):
  runtimes/
  resolvers/
  strategies/
  assets/
    scripts/
    images/
    compose/
  workflows/
    <bundled-example>/

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

When you work **on the dockpipe project itself**, you are a **user** of the tool: extend it via **`src/core/workflows/`** (bundled examples), **`scripts/`**, and repo-root **`workflows/`** — **not** by stuffing internal pipelines into **`src/core/workflows/`**.

| Location | Purpose |
|----------|---------|
| **`src/core/workflows/<name>/`** (this repo) / **`workflows/<name>/`** or legacy **`templates/<name>/`** (downstream) | **User-facing** workflow examples shipped in the bundle (**`run`**, **`run-apply`**, **`run-apply-validate`**, **`init`**, …). Reusable for any downstream project. |
| **`workflows/<name>/`** (repo root, this repo) | **Lean first-party** workflows wired into CI and dogfood: **`test`**, **`codex-pav`**, **`codex-security`**, **`dockpipe-repo-quality`**, etc. |
| **`.staging/workflows/<name>/`** (repo root, this repo) | **Maintainer / packaging / experiments** (R2 publish, self-analysis stacks, orchestrator, sandbox demos, …). Same **`--workflow <name>`** as **`workflows/`**; the binary embed merges both into the materialized cache. |

**Preferred pattern:** `dockpipe init <name> --from run-apply` or **`run-apply-validate`** (or **`run`**, **`blank`**) for user-shaped scaffolds; keep **automation you want in default CI** under **`workflows/`**; put **extra maintainer trees** under **`.staging/workflows/`**.

### Containment and official reference

**`.staging/`** and repo-root **`workflows/`** (this repository) are **maintainer / dogfood** trees. They **must not** alter the **engine contract** for anyone outside this checkout: downstream installs, minimal **`dockpipe`** usage, and **`src/lib/`** + **`src/cmd/`** behavior do **not** depend on those paths.

**Canonical ship:** Published **compiled** artifacts — **`dockpipe install core`**, future **`dockpipe install package …`**, and **HTTPS/static origins** you host (e.g. **`core.*` / `dockpipe.*`** style namespaces once live) — are the **official reference** for **versioned** workflows and core slices. **Those** are what external consumers pin; ad-hoc repo paths are **not** the stability boundary.

**Self-contained:** Packages are **YAML + assets + resolver/runtime wiring** resolved by the existing CLI; they **cannot** inject new engine primitives without a **separate** core change.

**Accelerator (maintainers):** After **`make build`**, **`make self-analysis`**, **`make self-analysis-host`**, or **`make self-analysis-stack`** run the DorkPipe self-analysis workflows on this repo (container, host-only, or compose stack). See **`docs/dorkpipe.md`** and **`src/lib/dorkpipe/workflows/dorkpipe-self-analysis/README.md`**.

### Agent guidance (this repository)

**Cursor / IDE:** **`.cursor/rules/dockpipe-agents.mdc`** (**`alwaysApply: true`**) mirrors this; keep it in sync.

**Two channels — do not conflate them:**

1. **On-disk context** — **`.dockpipe/`** and **`.dorkpipe/`** hold **generated** handoffs, self-analysis facts, CI bundles, metrics, and optional insights. Use them as **read-only grounding** with normal code reading. Pointers: **`docs/compliance-ai-handoff.md`**, **`docs/dorkpipe-ci-signals.md`**, **`docs/user-insight-queue.md`**. Pipeon: **`src/bin/pipeon`**, **`src/apps/pipeon/docs/`**. Do **not** auto-regenerate; refresh only when the **user** asks (then **`make self-analysis*`** / **`dorkpipe-self-analysis`** — see **Accelerator** above).

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
- Resolver → tool profile (**what** runs), declared in workflow YAML or added via packages  
- Strategy → how it wraps execution  

---

## One-line philosophy

DockPipe runs anything, anywhere, in isolation.

Keep it simple. Keep it composable.