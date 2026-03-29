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

## Engine boundary (STRICT — read this)

**`src/lib/`** and **`src/cmd/`** are the **engine**. They must **not** carry **knowledge of** what lives in **`packages/`**, repo-root **`workflows/`**, or **`.staging/workflows/`** — no hardcoded paths into those trees, no maintainer-specific workflow or resolver names in user-facing strings, tests, or control flow, and no “the way to do X is workflow `foo` under `packages/…`” in **`src/`**.

Treat **`packages/`**, **`workflows/`**, and **`.staging/workflows/`** as **separate products** (as if each were its **own repository**): they ship **YAML + assets** and are consumed only through **compile** → **`.dockpipe/internal/packages/`**, **HTTPS/package-store** tarballs, **embed** (repo-root **`embed.go`** is the **single** build-time list of embed roots — not duplicated as ad-hoc strings across **`src/`**), and **declarative** fields the runner already implements. **Outside** trees **touch** the engine through **compiled materialization** and **public CLI behavior** — not through Go code that imports their layout.

**Allowed in `src/`:** generic resolution (workflow name → config, resolver name → profile, logical `scripts/…` → on-disk path), **`.dockpipe/internal/packages/…`**, manifest/install **wire** shapes, and **one** indirection for embed roots (see **`src/lib/infrastructure/embedded_fs.go`** / **`embeddedPackageRootsPrefixes`**).

**Forbidden in `src/`:** repository-relative paths like **`packages/<group>/…`**, pointers to specific dogfood workflows as **the** documented path for a task, or anything that makes downstream **`dockpipe`** depend on **this** checkout’s tree shape. Put repository-specific procedures in **`docs/`** and package READMEs.

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
`…/core/resolvers/` (bundled) · **`packages/…/resolvers/…`** / **`.staging/workflows/…`** (maintainer overlays in this repo)

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

When you work **on the dockpipe project itself**, you are a **user** of the tool: extend it via **`src/core/workflows/`** (bundled examples) and repo-root **`workflows/`** — **not** by stuffing internal pipelines into **`src/core/workflows/`**. First-party workflow scripts belong **beside** that workflow’s **`config.yml`** (e.g. **`workflows/<name>/helper.sh`**) — **do not** add repo-root **`scripts/dockpipe/…`** for one-off flows; logical **`scripts/dockpipe/…`** resolves to **compiled** resolver assets under **`.dockpipe/internal/packages/`** (see **`paths.go`**), and a duplicate directory at the repo root **shadows** the wrong file.

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

**`src/` vs standalone trees (`packages/`, `workflows/`, `.staging/workflows/`):** Same rule as **Engine boundary** above: the engine does **not** mirror those directories in code. **`packages/`** is **standalone** authoring (per-package **`package.yml`**, resolvers, workflows, assets); repo-root **`workflows/`** and **`.staging/workflows/`** are **dogfood / maintainer** trees. All three interact with **`dockpipe`** only through **compile**, **store**, **embed**, and **declared** YAML — never through **`src/`** hardcoding their paths or names. Runtime resolution uses **`.dockpipe/internal/packages/`** and compile roots per **`docs/package-model.md`**.

**Secrets / vault templates:** Template files must contain **references only** (e.g. **`op://…`**), never committed plaintext secrets. Keep local templates gitignored when they name private vaults. **Never** use shell redirects like **`> -`** (that creates a file named **`-`**). **`op inject`** output for workflow env is read into **process memory** in the CLI — no second “resolved template” file is written by DockPipe for that merge.

**Accelerator (maintainers):** After **`make build`**, run dogfood workflows the same way as any downstream project: **`./src/bin/dockpipe --workflow <name> --workdir . --`** with materialized packages under **`.dockpipe/`**. Names and procedures live in **`docs/`** and maintainer package READMEs — not in **`src/`**.

### Agent guidance (this repository)

**Cursor / IDE:** **`.cursor/rules/dockpipe-agents.mdc`** (**`alwaysApply: true`**) mirrors this; keep it in sync.

**Two channels — do not conflate them:**

1. **On-disk context** — **`.dockpipe/`** and **`.dorkpipe/`** hold **generated** handoffs, self-analysis facts, CI bundles, metrics, and optional insights. Use them as **read-only grounding** with normal code reading. Contract: **`docs/artifacts.md`**. Pipeon binary: **`packages/pipeon/bin/pipeon`**; **Pipeon Launcher:** **`src/apps/pipeon-launcher/`**; docs under **`packages/pipeon/resolvers/pipeon/`** (resolver + VS Code extension). Do **not** auto-regenerate; refresh only when the **user** asks (then **`dockpipe --workflow dorkpipe-self-analysis`** / related names — see **Accelerator** above).

2. **MCP (`mcpd`)** — **Bounded tools** with **tiered IAM** (`DOCKPIPE_MCP_TIER`: `readonly` → `validate` → `exec`) via maintainer package **`packages/dockpipe-mcp/`** (module **`dockpipe.mcp`**). Default tier is **`validate`** (list + validate; **no** `dockpipe.run` / `dorkpipe.run_spec`). Tier **`exec`** (or legacy **`DOCKPIPE_MCP_ALLOW_EXEC=1`** when tier is unset) is required for run tools. **Docs:** **`packages/dockpipe-mcp/README.md`**; **`packages/dockpipe-mcp/mcpbridge/README.md`**.

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
- put **`packages/`**, **`workflows/`**, or **`.staging/workflows/`** paths, or maintainer-only workflow/resolver **names**, into **`src/lib/`** or **`src/cmd/`** (see **Engine boundary**)

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