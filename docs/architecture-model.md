# DockPipe architecture model (normative)

This document is **FINAL**. It defines the core concepts, their relationships, and **strict invariants**. Do not simplify, merge, reinterpret, or replace these concepts elsewhere in documentation.

---

## Core model

| Concept | Definition |
|--------|------------|
| **workflow** | **What happens** — execution intent (steps, structure, vars). |
| **runtime** | **Where execution happens** — isolated environment, **platform-agnostic**. |
| **capability** (abstract) | **Which need** — stable dotted id (e.g. **`cli.codex`**, **`blob.storage`**) for packages and docs. **Resolvers satisfy capabilities**; runtime stays separate. See **[capabilities.md](capabilities.md)**. |
| **resolver** | **Which platform/tool performs the work** — **platform-specific** adapter (a **resolver package** implements a **capability**). |
| **strategy** | **Lifecycle wrapper** — before/after execution behavior. |
| **assets** | **Support files** — scripts, image trees, and compose examples under the core authoring tree (**`src/core/assets/`** in-repo; equivalent installed/materialized core layout elsewhere). |
| **runtime.type** | **Classification of runtime behavior** — not implementation. |

---

## Workflows and packaged workflows (same spine)

**Workflows** and **workflow packages** are the **same idea** at different layers of reuse:

- Both express **what happens** with the **same** knobs: **runtime** (where execution runs), **resolver** (which tool/profile performs the work), and **strategy** (lifecycle before/after). **Packaging** does not invent a parallel model — it **ships** that same shape as an installable unit (**`package.yml`**, **`namespace:`**, compiled or published tree).

- **Running a workflow from disk** is the low-friction path: your repo holds **`workflows/…`** or **`templates/…`** and you point the CLI at it.

- **Using a packaged workflow inside another workflow** is **nesting** at the **call site**. The parent step uses the explicit packaged-workflow shape: **`workflow:`** names the **nested workflow** and **`package:`** is the **namespace** matching the child’s **`namespace:`** in **`config.yml`**. The parent does **not** duplicate the child’s scripts, resolver trees, or strategy files.

- **Inside** the packaged workflow, **child steps** still use ordinary runtimes (**`dockerimage`**, **`dockerfile`**, …). The packaged-workflow step form applies **only** to the **parent step** that **enters** the packaged unit — not to every step in the child.

- **Specialization** is meant to stay **thin**: the **child** workflow still owns its **defaults** (names of **core** runtime / resolver / strategy profiles in its YAML). The **caller** tunes behavior with **`vars`**, **`env`**, and shared CLI-style inputs merged into the nested run; explicit **step-level** **`runtime:`** / **`resolver:`** **selection** among **core** profiles is the natural extension when you need to swap substrate or tool **without** forking the package.

This keeps the **mental model** one-dimensional — **workflow → runtime → resolver → strategy** — whether the workflow is **inline** in the repo or **packaged** and **referenced**.

For authored workflow YAML, the practical rule is simple:

- top-level **`runtime`** / **`resolver`** set the default
- step-level **`runtime`** / **`resolver`** override that default when needed
- **`isolate`** is the low-level escape hatch for pinning an exact image/template

---

## Definitions

### Workflow

- Expresses **execution intent** only.
- **Must not** encode runtime implementation, resolver choice, or `runtime.type` as fixed behavior. Optional defaults for UX must remain swap‑out without changing the workflow’s intent.

### Runtime

- Represents an **isolated execution environment**.
- **Canonical authored substrate names:** **`dockerimage`** and **`dockerfile`**. Legacy YAML may use **`cli`** / **`powershell`** / **`cmd`** — they normalize to **`dockerimage`**. Labels like **`docker-node`** are **isolate** / image hints paired with **`dockerimage`** or **`dockerfile`**, not additional runtime kinds.
- **Platform-agnostic:** the same concept applies whether the backend is Docker, EC2, a local browser sandbox, or another substrate.
- **Must not** encode tool- or vendor-specific logic (no Claude, Cursor, Playwright behavior inside the **runtime** definition).

Packaged workflow invocation is a **workflow step form**, not part of the public authored runtime model.

### Resolver

- Represents a **platform/tool adapter**.
- **Examples (non‑exhaustive):** `claude`, `codex`, `cursor`, `code-server`, `playwright`.
- Defines **how a tool operates within or against** a runtime (invocation, auth hints, tool-specific defaults).
- **Must not** define or control the environment (no Docker, EC2, or infrastructure logic inside the **resolver** definition).

### Strategy

- Wraps the workflow run with **before/after** lifecycle (e.g. clone, commit, git worktree).
- Best thought of as a reusable wrapper, not as another execution substrate.
- **Not** a substitute for runtime, resolver, or `runtime.type`.

### runtime.type

- **Only** a **classification** of **runtime behavior** for the run. **Must not** depend on implementation (Docker, EC2, browser, etc.).
- **Values:**
  - **`execution`** — command/test execution (non-interactive)
  - **`ide`** — interactive development environment
  - **`agent`** — autonomous task execution

`runtime.type: agent` classifies the behavior of a run. It does not create a separate DockPipe
product model. In DockPipe, AI workers still run as governed workflow/package stages with explicit
resolver choice, artifact outputs, verification, and approval boundaries.

In configuration, **`DOCKPIPE_RUNTIME_TYPE`** is the field that carries **`runtime.type`** (see `src/lib/domain/runtime_kind.go`). The field classifies **behavior intent**, not the substrate.

---

## Core layout (filesystem)

In the repository, the authored core tree is **`src/core/`**. Installed or materialized core keeps
the same logical categories under its own core root. The core root contains **only** these category
directories — **no loose files** at the root:

| Directory | Role |
|-----------|------|
| **`runtimes/`** | Runtime profiles (**where** execution runs). |
| **`resolvers/`** | Resolver profiles (**which** tool/platform). |
| **`strategies/`** | Strategy **KEY=value** files (lifecycle before/after). |
| **`assets/`** | Reusable **support files** — **`assets/scripts/`**, **`assets/images/`**, and **`assets/compose/`**. Per-domain assets live under packages or resolver trees, not as new primitives. |
| **`workflows/`** | Bundled example workflows only. Not the home for repo-specific CI or maintainer automation. |

Workflows continue to reference scripts in YAML as **`scripts/…`**; the runner resolves
**`repo/scripts/…`** first, then resolver/package-owned paths, then core **`assets/scripts/…`**
through the centralized path helpers and package/store resolution logic.

Bundling policy and legal classification: **[templates-core-assets.md](templates-core-assets.md)**.

---

## Strict invariants (must never be violated)

1. **Runtime and resolver are separate concepts and must NEVER be merged** (not in naming, not in documentation, not as a single “thing” in the mental model).
2. **Runtime must NEVER contain tool/platform-specific logic** (no Claude, Cursor, Playwright behavior inside **runtime**).
3. **Resolver must NEVER define or control the environment** (no Docker, EC2, or infrastructure logic inside **resolver**).
4. **runtime.type is ONLY a classification** and must **NOT** depend on implementation (Docker, EC2, browser, etc.).
5. **Workflows must NOT encode runtime or resolver behavior internally** (no embedding of isolation or tool choice as the only way to run).
6. **Templates are scaffolding only** and must **NOT** define architecture, behavior, or classification. Bundled `templates/` trees are **examples and file layout** for `dockpipe init` / samples — not the **definition** of workflow, runtime, resolver, strategy, or `runtime.type`.
7. **Packaged workflow invocation:** a parent step that runs a **packaged** (namespaced) workflow uses the explicit **`workflow:`** + **`package:`** step form. Do not route it through **`runtime`**.

---

## Valid composition (pattern)

Each valid run is characterized by **all** of:

- a **workflow** (what),
- a **runtime** (where),
- a **runtime.type** (classification of that runtime’s behavior for this run),
- a **resolver** (which tool),
- and optionally a **strategy** (lifecycle) — not listed in every example below but composable.

**Example 1**

| Field | Value |
|-------|--------|
| workflow | `test` |
| runtime | `dockerimage` |
| runtime.type | `execution` |
| resolver | `playwright` |

**Example 2**

| Field | Value |
|-------|--------|
| workflow | `plan-apply-validate` |
| runtime | `dockerimage` |
| runtime.type | `execution` |
| resolver | `claude` |

**Example 3**

| Field | Value |
|-------|--------|
| workflow | `run` |
| runtime | `dockerimage` |
| runtime.type | `ide` |
| resolver | `cursor` |

**Example 4** (host secret injection — **substrate vs vendor**)

| Field | Value |
|-------|--------|
| workflow | `secretstore` (or any host `kind: host` flow with env from a vault) |
| runtime | `dockerimage` with host steps where needed; secret merge is **resolver**-owned. |
| runtime.type | `execution` |
| resolver | e.g. bundled **`dotenv`** (plain env file) or maintainer **`onepassword`** (`op`); other vaults are **other resolver profiles**, not other runtimes. |

**Example 5**

| Field | Value |
|-------|--------|
| workflow | `plan-apply-validate` |
| runtime | `dockerimage` |
| runtime.type | `agent` |
| resolver | `codex` |

---

## Composition rule

The **same workflow** must be able to run under different **runtime** + **resolver** pairs (and strategies) **without** changing the workflow definition, preserving **composability** and **separation of concerns**.

---

## Configuration layering (semantic separation)

Even when configuration is stored in one file on disk for convenience, **runtime** and **resolver** remain **semantically separate**:

- Keys and values that describe **where** and **how isolation is provisioned** belong to the **runtime** subsystem and **must not** import tool-specific semantics.
- Keys and values that describe **which tool** and **how it is invoked** belong to the **resolver** subsystem and **must not** import infrastructure or environment provisioning.

Orchestration **selects** a runtime and a resolver **together**; it does **not** merge them into one concept.

---

## Validation checklist (for documentation and design reviews)

Before treating any description as aligned with this architecture, verify:

- [ ] **Runtime** and **resolver** are **not** merged into a single concept (the abstract **capability** id in **[capabilities.md](capabilities.md)** is separate from **runtime** — it names a need, not where it runs).
- [ ] **Runtime** does **not** contain platform/tool product logic (Claude, Cursor, Playwright, etc.).
- [ ] **Resolver** does **not** contain environment/infrastructure provisioning logic (Docker, EC2, etc.).
- [ ] **runtime.type** is **only** used for **classification**, not for choosing Docker vs EC2 vs browser.
- [ ] **Workflows** do **not** encode runtime or resolver behavior as the only way to express intent.
- [ ] **Templates** are **not** treated as defining architecture, behavior, or classification.

If any item fails, **correct the wording** before publishing.

---

## Packaging & distribution (where things live)

This section **does not change** the four primitives above; it describes **where** implementations typically ship.

| Layer | Role | Default home |
|--------|------|----------------|
| **Runtimes** | Where execution runs — **stable, platform-agnostic profiles** | **In-repo** under **`src/core/runtimes/`** (light profile files; compiled/installed/materialized into core elsewhere). |
| **Strategies** | Lifecycle before/after — **small, stable** | **In-repo** under **`src/core/strategies/`**. |
| **Compiled core** | Tight core tree users can refresh from HTTPS | **Optional static origin install** via **`dockpipe install core`** + manifest, or compiled project-local core tarball under the package store. |
| **Resolvers** | Tool/platform **adapters** — **packages** that **implement** **capabilities** (`capability:` in **`package.yml`**) | **Bundled defaults** under **`src/core/resolvers/`** plus package/store resolver tarballs under **`bin/.dockpipe/internal/packages/`** for extended catalogs. |
| **Workflows** | What runs — **packages** when compiled/published; **rich metadata** for authoring and store discovery | **Project `workflows/`**, **installed packages**, or **store**; **`package.yml`** carries **`requires_capabilities`**, **`requires_resolvers`**, and dependency hints. |

**Ecosystem shape:** **workflows** and **resolver** packs are the natural **“plugin store”** surface (metadata-heavy). **Runtimes** and **strategies** stay **minimal and in-repo** so every install has a predictable, lightweight spine.

**Execution and network (product intent):**

- **Two run modes** are both valid: **source** workflows from the repo authoring tree (today’s low-friction path), and **compiled** workflows under **`bin/.dockpipe/internal/packages/workflows/`** after **`dockpipe package compile workflow`** (and future richer compile). Neither replaces the other.
- **Remote fetches** (HTTPS / CDN / registry) are aimed at **install and release** commands, not at every **`dockpipe run`** once artifacts are local.

Full detail: **[package-model.md](package-model.md)** (**`package.yml`**, compile → package → release, workflow vs resolver dependencies, resolution order).

---

## Related docs (non-normative mechanics)

- [isolation-layer.md](isolation-layer.md) — file paths, `DOCKPIPE_RUNTIME_*` / `DOCKPIPE_RESOLVER_*` aliases, lookup order  
- [operation-results.md](operation-results.md) — unit-of-work result/logging contract for meaningful operations
- [workflow-yaml.md](workflow-yaml.md) — workflow and strategy fields  
- [architecture.md](architecture.md) — data flow and extension points  
