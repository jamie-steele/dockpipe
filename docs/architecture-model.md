# DockPipe architecture model (normative)

This document is **FINAL**. It defines the core concepts, their relationships, and **strict invariants**. Do not simplify, merge, reinterpret, or replace these concepts elsewhere in documentation.

---

## Core model

| Concept | Definition |
|--------|------------|
| **workflow** | **What happens** — execution intent (steps, structure, vars). |
| **runtime** | **Where execution happens** — isolated environment, **platform-agnostic**. |
| **resolver** | **Which platform/tool performs the work** — **platform-specific** adapter. |
| **strategy** | **Lifecycle wrapper** — before/after execution behavior. |
| **assets** | **Support files** — scripts, image trees, compose examples under **`templates/core/assets/`** (not additional primitives). |
| **runtime.type** | **Classification of runtime behavior** — not implementation. |

---

## Definitions

### Workflow

- Expresses **execution intent** only.
- **Must not** encode runtime implementation, resolver choice, or `runtime.type` as fixed behavior. Optional defaults for UX must remain swap‑out without changing the workflow’s intent.

### Runtime

- Represents an **isolated execution environment**.
- **Examples (non‑exhaustive):** `docker-node`, `docker-browser`, `ec2-worker`, `ide-local`, `browser-ide`.
- **Platform-agnostic:** the same concept applies whether the backend is Docker, EC2, a local browser sandbox, or another substrate.
- **Must not** encode tool- or vendor-specific logic (no Claude, Cursor, Playwright behavior inside the **runtime** definition).

### Resolver

- Represents a **platform/tool adapter**.
- **Examples (non‑exhaustive):** `claude`, `codex`, `cursor`, `code-server`, `playwright`.
- Defines **how a tool operates within or against** a runtime (invocation, auth hints, tool-specific defaults).
- **Must not** define or control the environment (no Docker, EC2, or infrastructure logic inside the **resolver** definition).

### Strategy

- Wraps the workflow run with **before/after** lifecycle (e.g. clone, commit, git worktree).
- **Not** a substitute for runtime, resolver, or `runtime.type`.

### runtime.type

- **Only** a **classification** of **runtime behavior** for the run. **Must not** depend on implementation (Docker, EC2, browser, etc.).
- **Values:**
  - **`execution`** — command/test execution (non-interactive)
  - **`ide`** — interactive development environment
  - **`agent`** — autonomous task execution

In configuration, **`DOCKPIPE_RUNTIME_TYPE`** is the field that carries **`runtime.type`** (see `src/lib/dockpipe/domain/runtime_kind.go`). The field classifies **behavior intent**, not the substrate.

---

## `templates/core/` layout (filesystem)

Under the repository (and in materialized bundles), **`templates/core/`** contains **only** these category directories — **no loose files** at the `core/` root:

| Directory | Role |
|-----------|------|
| **`runtimes/`** | Runtime profiles (**where** execution runs). |
| **`resolvers/`** | Resolver profiles (**which** tool/platform). |
| **`strategies/`** | Strategy **KEY=value** files (lifecycle before/after). |
| **`bundles/`** | **Domain** script/asset trees (**dorkpipe**, **pipeon**, **review-pipeline**, …) — not resolvers; see **`paths.go`** resolution order. |
| **`assets/`** | Reusable **support files** — **`assets/scripts/`** (agnostic shell only), **`assets/images/`** (agnostic Dockerfiles only: **base-dev**, **dev**, **example**, **minimal**), **`assets/compose/README.md`** + agnostic **`minimal/`** / **`multi-service/`** demos. Per-domain images and compose live under **`resolvers/…/assets/`** or **`bundles/…/assets/`**. Not additional primitives. |

Workflows continue to reference scripts in YAML as **`scripts/…`**; the runner resolves **`repo/scripts/…`** first, then **`templates/core/resolvers/…`** (resolver-owned), **`templates/core/bundles/…`** (domain asset packs), then **`templates/core/assets/scripts/…`** in the bundled tree.

Bundling policy and legal classification: **[templates-core-assets.md](templates-core-assets.md)**.

---

## Strict invariants (must never be violated)

1. **Runtime and resolver are separate concepts and must NEVER be merged** (not in naming, not in documentation, not as a single “thing” in the mental model).
2. **Runtime must NEVER contain tool/platform-specific logic** (no Claude, Cursor, Playwright behavior inside **runtime**).
3. **Resolver must NEVER define or control the environment** (no Docker, EC2, or infrastructure logic inside **resolver**).
4. **runtime.type is ONLY a classification** and must **NOT** depend on implementation (Docker, EC2, browser, etc.).
5. **Workflows must NOT encode runtime or resolver behavior internally** (no embedding of isolation or tool choice as the only way to run).
6. **Templates are scaffolding only** and must **NOT** define architecture, behavior, or classification. Bundled `templates/` trees are **examples and file layout** for `dockpipe init` / samples — not the **definition** of workflow, runtime, resolver, strategy, or `runtime.type`.

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
| runtime | `docker-browser` |
| runtime.type | `execution` |
| resolver | `playwright` |

**Example 2**

| Field | Value |
|-------|--------|
| workflow | `plan-apply-validate` |
| runtime | `docker-node` |
| runtime.type | `execution` |
| resolver | `claude` |

**Example 3**

| Field | Value |
|-------|--------|
| workflow | `run` |
| runtime | `ide-local` |
| runtime.type | `ide` |
| resolver | `cursor` |

**Example 4**

| Field | Value |
|-------|--------|
| workflow | `plan-apply-validate` |
| runtime | `ec2-agent` |
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

- [ ] **Runtime** and **resolver** are **not** merged into a single concept or single named primitive.
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
| **Runtimes** | Where execution runs — **stable, platform-agnostic profiles** | **In-repo** under **`templates/core/runtimes/`** (light profile files; stays in the bundle / git tree). |
| **Strategies** | Lifecycle before/after — **small, stable** | **In-repo** under **`templates/core/strategies/`** (thin env + script pointers). |
| **Compiled core** | Tight **`templates/core`** tree users can refresh from HTTPS | **Optional S3/R2 (or any static origin)** via **`dockpipe install core`** + manifest (slim baseline; not every resolver in the universe). |
| **Resolvers** | Tool/platform **adapters** — **plugin-shaped**, reusable across workflows | **Bundled** under **`templates/core/resolvers/`** *or* **store packages** (tarball / **`.dockpipe/internal/packages/`**) for extended catalogs. |
| **Workflows** | What runs — **rich metadata** for authoring and store discovery | **Project `workflows/`**, **installed packages**, or **store**; **`package.yml`** carries discovery and dependency hints. |

**Ecosystem shape:** **workflows** and **resolver** packs are the natural **“plugin store”** surface (metadata-heavy). **Runtimes** and **strategies** stay **minimal and in-repo** so every install has a predictable, lightweight spine.

**Execution and network (product intent):**

- **Two run modes** are both valid: **source** workflows from the repo authoring tree (today’s low-friction path), and **compiled** workflows under **`.dockpipe/internal/packages/workflows/`** after **`dockpipe package compile workflow`** (and future richer compile). Neither replaces the other.
- **Remote fetches** (HTTPS / CDN / registry) are aimed at **install and release** commands, not at every **`dockpipe run`** once artifacts are local.

Full detail: **[package-model.md](package-model.md)** (**`package.yml`**, compile → package → release, workflow vs resolver dependencies, resolution order).

---

## Related docs (non-normative mechanics)

- [isolation-layer.md](isolation-layer.md) — file paths, `DOCKPIPE_RUNTIME_*` / `DOCKPIPE_RESOLVER_*` aliases, lookup order  
- [runtime-architecture.md](runtime-architecture.md) — CLI and migration notes  
- [workflow-yaml.md](workflow-yaml.md) — workflow and strategy fields  
- [architecture.md](architecture.md) — data flow  
