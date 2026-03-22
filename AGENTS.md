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

**Do not** put **this repository’s** CI, demo, or internal automation workflows in **`templates/`**. Those belong under **`dockpipe-experimental/workflows/<name>/`** (see **Internal workflows** below).

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

When you work **on the dockpipe project itself**, you are a **user** of the tool: extend it via **`templates/`**, **`scripts/`**, and **`dockpipe-experimental/workflows/`** — **not** by stuffing internal pipelines into **`templates/`**.

| Location | Purpose |
|----------|---------|
| **`templates/<name>/`** | **User-facing** workflow examples shipped in the bundle (**`run`**, **`run-apply`**, **`run-apply-validate`**, **`init`**, …). Reusable for any downstream project. |
| **`dockpipe-experimental/workflows/<name>/`** | **This repo only:** CI, recordings, experiments — workflows that exist to run **dockpipe** on **this** codebase. Not installed by a special `init` flag; copy dirs or use **`dockpipe init &lt;name&gt; --from …`** pointing at a workflow path. |

**Preferred pattern:** `dockpipe init <name> --from run-apply` or **`run-apply-validate`** (or **`run`**, **`blank`**) for user-shaped scaffolds; keep automation-specific YAML under **`dockpipe-experimental/workflows/`**.

**Accelerator (maintainers):** After **`make build`**, **`make self-analysis`**, **`make self-analysis-host`**, or **`make self-analysis-stack`** run the DorkPipe self-analysis workflows on this repo (container, host-only, or compose stack). See **`docs/dorkpipe.md`** and **`dockpipe-experimental/workflows/dorkpipe-self-analysis/README.md`**.

### Agent guidance: repository analysis (before repo-level work)

**Cursor / IDE:** This repo ships **`.cursor/rules/dockpipe-agents.mdc`** (**`alwaysApply: true`**) so sessions load the same contract as this section. Keep it in sync when you change the rules below.

This repository includes **this file (`AGENTS.md`)** and may include **generated analysis artifacts**. Other trees may use different layout (e.g. a project-local directory under **`.dockpipe/`**); **here**, the canonical locations are below.

**1. Discover context**

- Read **`AGENTS.md`** first.
- Locate analysis artifacts. In **this** repo they typically live under **`.dockpipe/`** (handoff / paste text) and **`.dorkpipe/`** (orchestrator metadata and **`.dorkpipe/self-analysis/`** raw facts). Use whatever paths this section and your checkout reference.

| Artifact | Path (this repo) |
|----------|------------------|
| Short paste block | **`.dockpipe/paste-this-prompt.txt`** |
| Full handoff | **`.dockpipe/orchestrator-cursor-prompt.md`** |
| Raw facts (git, signals, excerpts) | **`.dorkpipe/self-analysis/`** |
| Orchestrator run metadata (if present) | **`.dorkpipe/run.json`**, **`.dorkpipe/metrics.jsonl`** |
| **CI scan signals** (govulncheck + gosec, normalized) | **`.dockpipe/ci-analysis/findings.json`** — produced by CI / **`scripts/ci-local.sh`**; download from **Actions artifacts** if not local. See **`docs/dorkpipe-ci-signals.md`**. |
| **Compliance / security posture (for AI)** | **`docs/compliance-ai-handoff.md`** — how to answer “compliance issues?” without claiming certification; **`make compliance-handoff`** / workflow **`compliance-handoff`** lists signal paths. |
| **Structured user guidance (optional)** | **`.dockpipe/analysis/insights.json`** — normalized, reviewable **signals** from **`user-insight-enqueue` / `user-insight-process`** (see **`docs/user-insight-queue.md`**). Not repo facts or scan truth. |
| **Pipeon (local Ollama helper)** | **`bin/pipeon`** — builds **`.dockpipe/pipeon-context.md`** and can **`chat`** via Ollama; **feature-flagged** (`DOCKPIPE_PIPEON=1`, min version **0.6.5** unless **`DOCKPIPE_PIPEON_ALLOW_PRERELEASE=1`**). Editor story: **Code OSS fork** + **`contrib/pipeon-vscode-extension/`** — **`docs/pipeon-vscode-fork.md`**, **`docs/pipeon-architecture.md`**. Harness: **`scripts/pipeon/README.md`**, **`docs/pipeon-shortcuts.md`**. |

If present, **load and use** them as **primary** context for understanding the repository, together with normal code reading.

**2. Evaluate freshness**

- Use available metadata: **git** ref / commit in **`self-analysis`** or handoff text, **timestamps** on files, **`VERSION`**, or fields in **`.dorkpipe/run.json`** when present.
- Judge whether analysis appears **current** relative to **`HEAD`** and recent change volume (merges, large refactors).

**3. Decide behavior**

- If analysis **exists** and appears **current** → use it **directly** as context.
- If analysis **exists** but may be **stale** → **tell the user** that refreshing repo analysis is **recommended** before substantial work → **continue** with existing analysis **unless** they say otherwise.
- If **no** analysis exists → **tell the user** it has not been generated and **recommend** running the repo-analyzer workflow (here: **`dorkpipe-self-analysis`** via **`make self-analysis`**, **`make self-analysis-host`**, or **`make self-analysis-stack`** — see **Accelerator** above).

**4. Refresh rules**

- Do **not** automatically refresh analysis unless the user **explicitly** asks.
- If the user requests a refresh → run the **repo-analyzer** workflow (**`dorkpipe-self-analysis`** / **`make self-analysis*`**), let artifacts update, then **re-load** analysis before continuing.

**5. Usage contract**

- Treat analysis artifacts as **structured, authoritative** context **where applicable**.
- Do **not** duplicate **large-scale** repo analysis if usable artifacts already exist.
- Use analysis to guide **planning**, **validation**, and **implementation** decisions.

**6. Response requirements**

When responding, **state** whether analysis artifacts were **found**, and whether they appear **current** or **potentially stale**; **recommend refresh** when appropriate; then **proceed** with the requested task using the **best available** context.

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