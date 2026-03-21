# Runtime layer — refactor plan (first-class isolated execution)

This document is the **implementation / migration** companion. For the **canonical separation** of **workflow**, **runtime**, **resolver**, **strategy**, and **`runtime.type`**, read **[architecture-model.md](architecture-model.md)** first.

Below, **`runtime`** still refers to **isolated execution environment** (technical `DOCKPIPE_RUNTIME_*`). The CLI flag **`--runtime`** today selects a **named profile file** that **bundles** runtime wiring and resolver/tool metadata — see **architecture-model.md** for why those ideas stay distinct.

---

## 1. Core concepts (mental model)

| Concept | Answers | Examples |
|--------|---------|----------|
| **`workflow`** | **What** runs — the execution pattern, steps, vars, structure. | **`test`**, **`run`**, **`run-apply-validate`**, thin **`claude`** / **`codex`** / **`code-server`** helpers — each is **`templates/<name>/config.yml`**, not a runtime folder. **`vscode`** delegate YAML is not marketed as a first-class workflow in the README. |
| **`runtime` (environment)** | **Isolated execution environment** — *where* work runs; **platform-agnostic** contract via **`DOCKPIPE_RUNTIME_*`**. | Image template, embedded workflow isolate, host script — not “Claude vs Codex.” |
| **`runtime.type` (`DOCKPIPE_RUNTIME_TYPE`)** | **Behavior classification** — **`execution`** \| **`ide`** \| **`agent`**; intent only, not Docker vs EC2. | Bundled **`claude`** profile → **`agent`**; **`vscode`** → **`ide`** |
| **Profile name (CLI `--runtime` / `--resolver`)** | Selects **`templates/core/runtimes/<name>`** or **`resolvers/<name>`** — file that **composes** runtime + resolver for that integration. | **`--runtime vscode`** → **`templates/core/resolvers/vscode`** |
| **`resolver` (tool adapter)** | **Platform-specific** integration — how commands run for that tool. Same on-disk file as the profile today; conceptually separate from **runtime** wiring. | **`claude`**, **`codex`**, **`playwright`** — vendor hints, default CLI, sub-workflow |
| **Technical keys (`DOCKPIPE_RUNTIME_*`)** | *How* isolation is wired (container image, host script, embedded workflow, …). **Not** tied to a vendor name. | “Dockerfile template”, “pinned image”, “delegate to workflow”, “host script” |
| **`strategy`** | **What wraps** the run — lifecycle **before** / **after** the workflow body (host hooks). | **`worktree`**, **`commit`**, … — **`templates/core/strategies/worktree`**; set **`strategy: worktree`** in workflow YAML. |

**`runtime.type` vs technical keys vs resolver**

- **`DOCKPIPE_RUNTIME_TYPE`** (**`runtime.type`**) — *what behavior class?* (**`execution`**, **`ide`**, **`agent`**).
- **`DOCKPIPE_RUNTIME_*`** — *how is the environment isolated?* (vendor-neutral key names).
- **Resolver** — *which tool adapter?* (named profile; **`DOCKPIPE_RESOLVER_*`** aliases). **`--runtime`** and **`--resolver`** select the same profile **name**; prefer **`--runtime`** in docs.

**Strict separation:**

- Do **not** encode **runtime** or **strategy** choices into **workflow** names (avoid `*-claude-*` workflows except as legacy aliases).
- Do **not** put lifecycle policy (clone, commit) into **runtime** — that stays **strategy**.
- Do **not** overload **strategy** with “which container” — that is **`DOCKPIPE_RUNTIME_*`** wiring and **resolver** selection, not lifecycle.

---

## 2. Current state vs target

| Today (approx.) | Role | Target |
|-----------------|------|--------|
| **`--isolate`** | Image or `TemplateBuild` name for single-command / step `isolate:` | Becomes **runtime selection** for container-backed runtimes (subset of `--runtime`). |
| **`--resolver`** | Selects a **specific** named profile (**`templates/core/resolvers/<name>`**, or **`runtimes/<name>`** first) | **`--runtime <name>`** is preferred; same lookup. **Resolver** = **specific** name; **runtime** = **agnostic** contract the file implements. |
| **`templates/core/resolvers/`** | **Specific** named profiles (`claude`, …) | **`templates/core/runtimes/`** checked first for the same **name**; bundled files still ship under **`resolvers/`** today. |
| **`DOCKPIPE_RESOLVER_*`** keys | **Specific** profile fields (per named resolver file) | **`DOCKPIPE_RUNTIME_*`** = **agnostic** contract; **`DOCKPIPE_RESOLVER_*`** = same semantics, legacy name (reader merges both). |
| **`domain.ResolverAssignments`** | Parsed profile | **`RuntimeAssignments`** (or `ResolverAssignments` embedded in `RuntimeConfig`) with same semantics. |

**Absorption rule:** the **agnostic** “how do we isolate?” contract is **runtime** (**`DOCKPIPE_RUNTIME_*`**). The **specific** “which named profile (`claude`, …)?” is **resolver** (on-disk file + **`DOCKPIPE_RESOLVER_*`** aliases). Neither belongs in **workflow** (what runs) or **strategy** (lifecycle hooks).

**On-disk layout**

- **Specific** profiles: **`KEY=value`** files only under **`templates/core/runtimes/<name>`** or **`templates/core/resolvers/<name>`** (or **`resolvers/<name>/profile`**). **Not** next to a workflow’s `config.yml`.
- **Workflows** are **`templates/<workflow>/config.yml`** (or any path via **`--workflow-file`**). Multi-step pipelines (`test` → `run` → **apply** → **validate**) are expressed as **steps:** or **imports:** in YAML, or as separate **`--workflow`** names — not as extra files beside the workflow folder.

---

## 3. Runtime definition (minimal shape)

A **runtime** is a **named** bundle of rules the runner uses to enter the isolate phase. Minimal v1 shape (evolution of today’s resolver file):

```text
# templates/core/runtimes/<name>  (or resolvers/<name> during transition)

# Exactly one primary backend (validated):
DOCKPIPE_RUNTIME_KIND=dockerfile   # | image | compose | workflow | host | remote (future)

# Docker image from repo Dockerfile (today: DOCKPIPE_RESOLVER_TEMPLATE + TemplateBuild)
DOCKPIPE_RUNTIME_IMAGE_TEMPLATE=claude

# Or raw image / URL
# DOCKPIPE_RUNTIME_IMAGE=ghcr.io/org/img:tag

# Or delegate to a bundled workflow (today: DOCKPIPE_RESOLVER_WORKFLOW)
# DOCKPIPE_RUNTIME_WORKFLOW=cursor-dev

# Or host script instead of docker (today: DOCKPIPE_RESOLVER_HOST_ISOLATE)
# DOCKPIPE_RUNTIME_HOST_SCRIPT=scripts/...

# Optional: compose file (future)
# DOCKPIPE_RUNTIME_COMPOSE_FILE=docker-compose.runtime.yml
# DOCKPIPE_RUNTIME_COMPOSE_SERVICE=tool

# Metadata (docs / UX)
DOCKPIPE_RUNTIME_CMD=claude
DOCKPIPE_RUNTIME_ENV=ANTHROPIC_API_KEY,...
DOCKPIPE_RUNTIME_EXPERIMENTAL=0
```

**Kinds map to implementations:**

| `KIND` | Mechanism in runner |
|--------|---------------------|
| `dockerfile` | `TemplateBuild` + `docker run` (current path). |
| `image` | `docker run` with explicit image, no build. |
| `compose` | `docker compose run` / `up` (new code path). |
| `workflow` | Embedded `templates/<wf>/config.yml` (current `DOCKPIPE_RESOLVER_WORKFLOW`). |
| `host` | Host script, no container (current host isolate). |
| `remote` / `electron` / `browser` | Future: thin wrappers; same **outer** contract (env, workdir, exit code). |

**Versioning:** add **`DOCKPIPE_RUNTIME_FORMAT=1`** when introducing new keys; older files without it are interpreted as **format 0** = current resolver semantics only.

---

## 4. CLI direction

**Target UX (additive migration):**

```bash
dockpipe -- <command>                                    # default runtime / isolate from workflow or base-dev
dockpipe --workflow test --runtime playwright -- npx playwright test
dockpipe --workflow plan-apply-validate --runtime claude --strategy worktree -- ...
```

**Precedence (proposed):**

1. **`--runtime <name>`** — selects **`templates/core/runtimes/<name>`** (or legacy **`resolvers/<name>`**).
2. If omitted: **`workflow.runtime`** in YAML (new field, optional).
3. If omitted: **`default_runtime`** / **`default_resolver`** (legacy) / **`isolate`** (legacy) — same resolution order as today for compatibility.

**Aliases:** **`--resolver <name>`** → identical to **`--runtime <name>`** for N releases; deprecate **`--resolver`** in docs once **`--runtime`** is ubiquitous.

**`--isolate`:** keep for **direct image** or **template name** without a profile file; document as “inline runtime override” or merge into **`--runtime`** when the name is unambiguous.

---

## 5. Workflow YAML surface (target)

```yaml
name: plan-apply-validate
description: ...

# Optional defaults when CLI omits --runtime
runtime: claude          # new canonical field
runtimes: [claude, codex] # optional allowlist (like strategies:)

strategy: worktree
strategies: [worktree, commit]

steps: ...
```

**`isolate:`** on a step remains the **per-step runtime override** (image or template name); consider aliasing to **`runtime:`** on steps for consistency (same merge rules as today).

---

## 6. Interaction diagram

```
┌─────────────┐     ┌──────────────┐     ┌─────────────┐
│  strategy   │────▶│   workflow   │────▶│   runtime   │
│ before/after│     │ what / steps │     │ isolate env │
└─────────────┘     └──────────────┘     └─────────────┘
       │                     │                    │
       │                     │                    ▼
       │                     │            docker / compose /
       │                     │            embedded wf / host /
       │                     │            (future) electron…
       └─────────────────────┴──────────────────────────────▶ success path only: strategy after
```

- **Strategy** wraps the **whole** workflow execution (including runtime setup that runs *inside* the “body” if we define body = workflow steps only — today strategy before runs *before* pre-scripts merged with workflow `run`; keep that ordering).
- **Runtime** applies to **each isolate phase** (single-command or per-step).

---

## 7. Migration phases (minimal risk)

| Phase | Work |
|-----|------|
| **A — Docs & naming** | Treat **`docs/isolation-layer.md`** as predecessor to **`runtime`**; add this doc; README/onboarding use **workflow / runtime / strategy** triad. **No** breaking CLI. |
| **B — CLI alias** | Add **`--runtime`** to **`CliOpts`**, **`ParseFlags`**, same variable as **`--resolver`** internally (`effectiveRuntime := opts.Runtime \|\| opts.Resolver`) or unified field. Update usage + **`cli-reference.md`**. |
| **C — YAML** | Add **`runtime`**, **`runtimes`** optional allowlist; mirror **`strategy`** validation. **`default_resolver`** → **`default_runtime`** with legacy read of old key. |
| **D — On-disk** | Optionally symlink or copy **`templates/core/resolvers` → `templates/core/runtimes`**; loader checks both. |
| **E — Keys** | Introduce **`DOCKPIPE_RUNTIME_*`** with **`FromResolverMap`** compatibility shim reading old keys. |
| **F — Domain** | Rename or wrap **`ResolverAssignments`** as **`RuntimeAssignments`**; keep JSON tag aliases for tests. |
| **G — Deprecation** | Warn on **`--resolver`**; remove in a major version if desired. |

---

## 8. What does *not* change (by design)

- **Workflows** stay **YAML** with **`steps:`**, **`vars:`**, **`imports:`** — not renamed to “runtimes.”
- **Strategies** stay **before/after** only — no Docker knowledge inside strategy files except **script paths** that *happen* to call git.
- **Embedded workflows** used as **runtime** backends (cursor-dev, vscode) remain **workflows** on disk; **runtime** is the **pointer** (`DOCKPIPE_RUNTIME_WORKFLOW=cursor-dev`), not duplication of their YAML.

---

## 9. Success criteria

- New users learn: **workflow = what**, **runtime = isolated env**, **strategy = wrap**.
- One directory (**`templates/core/runtimes/`**) and one flag (**`--runtime`**) become the obvious place to add **Playwright**, **Compose**, **Electron**, without new top-level concepts.
- Existing **`--resolver`** / **`resolvers/`** continue to work until deprecated.

---

## 10. Related docs

- **[architecture-model.md](architecture-model.md)** — canonical **workflow / runtime / resolver / strategy / runtime.type** definitions and composability examples.
- **[isolation-layer.md](isolation-layer.md)** — profile files, keys, lookup order.
- **[workflow-yaml.md](workflow-yaml.md)** — workflow + strategy fields.
- **[architecture.md](architecture.md)** — data flow and extension points.
