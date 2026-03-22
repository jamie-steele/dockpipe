# AGENTS.md — dockpipe maintainer + agent guide

This file defines how this repo works. Follow it strictly.

---

## Core concept

DockPipe provides one primitive:

1. Spawn an isolated environment  
2. Run a command inside it  
3. Optionally act on the result  

Everything else is built around this.

**`dockpipe init`** is project-local scaffolding only: it creates or updates **`templates/`** (including merged **`templates/core/`** with **`runtimes/`**, **`resolvers/`**, **`strategies/`**, **`assets/`** — scripts, images, compose), a project-local **`scripts/`** for copies/samples, **`images/`** for the example scaffold, in the **current directory**. It does **not** clone Git repositories or bootstrap a remote project. See **`docs/cli-reference.md`** and **`docs/templates-core-assets.md`**.

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
templates/<name>/

**Do not** put **this repository’s** CI, demo, or internal automation workflows in **`templates/`**. Those belong under **`dockpipe/workflows/<name>/`** (see **Internal workflows** below).

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

📁 Location:
templates/core/runtimes/

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
templates/core/resolvers/

---

### 4. Strategies

Lifecycle wrappers.

- Modify execution behavior
- Examples: worktree, commit

📁 Location:
templates/core/strategies/

---

## CRITICAL STRUCTURE RULE

`templates/core/` contains ONLY category folders.

Valid:
templates/core/
  runtimes/
  resolvers/
  strategies/
  assets/
    scripts/
    images/
    compose/

Invalid:
templates/core/claude
templates/core/docker
templates/core/test

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
- modifying `lib/` or `cmd/`
- adding template-specific logic to core

If something cannot be done:

→ propose a **general primitive**, not a special case

---

## Internal workflows (this repository)

When you work **on the dockpipe project itself**, you are a **user** of the tool: extend it via **`templates/`**, **`scripts/`**, and **`dockpipe/workflows/`** — **not** by stuffing internal pipelines into **`templates/`**.

| Location | Purpose |
|----------|---------|
| **`templates/<name>/`** | **User-facing** workflow examples shipped in the bundle (**`run`**, **`run-apply`**, **`run-apply-validate`**, **`init`**, …). Reusable for any downstream project. |
| **`dockpipe/workflows/<name>/`** | **This repo only:** CI, recordings, experiments — workflows that exist to run **dockpipe** on **this** codebase. Not installed by a special `init` flag; copy dirs or use **`dockpipe init &lt;name&gt; --from …`** pointing at a workflow path. |

**Preferred pattern:** `dockpipe init <name> --from run-apply` or **`run-apply-validate`** (or **`run`**, **`blank`**) for user-shaped scaffolds; keep automation-specific YAML under **`dockpipe/workflows/`**.

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