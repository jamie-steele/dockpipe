# Isolation layer (named execution environments)

Dockpipe separates **what runs** (workflow, steps, commands) from **where / how it runs** (container image, host script, embedded sub-workflow). This doc names that second concern the **isolation layer**.

**Normative terminology:** **[architecture-model.md](architecture-model.md)** (FINAL). **workflow** · **runtime** (environment) · **resolver** (tool) · **strategy** · **`runtime.type`**. A single on-disk file may hold keys for **both** subsystems; they remain **semantically separate** per architecture-model § *Configuration layering*. **`DOCKPIPE_RUNTIME_TYPE`** = **`runtime.type`**. See **[runtime-architecture.md](runtime-architecture.md)** for mechanics.

---

## Concepts

| Term | Meaning |
|------|--------|
| **runtime.type** | **`execution`** \| **`ide`** \| **`agent`** — classification of **runtime behavior** only (not Docker vs EC2). Set with **`DOCKPIPE_RUNTIME_TYPE`** (see **`domain/runtime_kind.go`**). |
| **Technical runtime keys** | *How* isolation is wired — Dockerfile-backed image, pinned image, embedded workflow YAML, or host script. Expressed with **`DOCKPIPE_RUNTIME_*`** (vendor-agnostic field names). |
| **Resolver (specific)** | *Which named profile* — **`templates/core/resolvers/<name>`** (file or **`profile`**) (legacy: **`templates/run-worktree/resolvers/<name>`**), **`--resolver`**, **`DOCKPIPE_RESOLVER_*`** as aliases of **`DOCKPIPE_RUNTIME_*`** for that **specific** file. |
| **Isolation profile** | Informal: the **specific** on-disk **`KEY=value`** file (resolver) that describes one isolate phase. |
| **Profile name** | The string you pass as **`--runtime`** / **`--resolver`** or **`runtime:`** / **`resolver:`** in YAML (resolved only under **`templates/core/`**). **`--isolate`** / **`isolate:`** can also name a **`TemplateBuild`** template or a raw image without a profile file. |
| **Profile file** | A **`KEY=value`** file under **`templates/core/resolvers/<name>`** or **`templates/core/resolvers/<name>/profile`** (or legacy **`templates/run-worktree/resolvers/`**). **No** per-workflow override — custom behavior belongs in **workflow** YAML. |

**Workflow** = sequence, vars, steps, **`strategy:`** (lifecycle wrappers).  
**Strategy** = before/after host hooks around that body.  
**Isolation profile** (informal) = on-disk **`KEY=value`** used for the isolate phase; **`DOCKPIPE_RUNTIME_TYPE`** carries **`runtime.type`**; **`DOCKPIPE_RUNTIME_*`** / **`DOCKPIPE_RESOLVER_*`** map to **runtime** vs **resolver** roles per **[architecture-model.md](architecture-model.md)**.

---

## Profile kinds (cohesion model)

A profile is **one** of these execution shapes. The runner decides from which keys are set in the profile file (and from `isolate:` / `TemplateBuild`).

| Kind | Mechanism | Typical examples |
|------|-----------|------------------|
| **Dockerfile template** | **`DOCKPIPE_RESOLVER_TEMPLATE`** → **`TemplateBuild`** → build **`images/<name>/`**, run **`docker run`**. | `claude`, `codex`, `vscode`, `base-dev`, `dev`, `agent-dev` |
| **Pinned image** | **`isolate:`** in YAML or CLI **`--isolate`** with a name **`TemplateBuild`** does not know → treat as **image name** (optional `:` tag). | `alpine`, `dockpipe-claude:1.2.3` |
| **Embedded workflow** | **`DOCKPIPE_RESOLVER_WORKFLOW`** → run **`templates/<name>/config.yml`** with the same runner (multi-step / host IDE). | `cursor-dev`, `vscode`, `claude`, `codex`, `code-server` (single-step templates) |
| **Host isolate** | **`DOCKPIPE_RESOLVER_HOST_ISOLATE`** → host script instead of `docker run` for that step/run. | Custom installers |
| **Compose / URL / desktop** | *Not first-class in the runner yet* — extension points below. | Future |

**Same name, different axes:** e.g. **`code-server`** can be a **Dockerfile template** image when you **`--isolate code-server`**, or **embedded delegate** when the **`worktree`** strategy sample + resolver **`code-server`** sets **`DOCKPIPE_RESOLVER_WORKFLOW=code-server`**. The profile file defines which path applies.

---

## Where things live (framework layout)

| Location | Role |
|----------|------|
| **`templates/core/resolvers/<name>`** | Shared **resolver** profiles (tool integrations): claude, codex, cursor, vscode, code-server, … |
| **`images/<name>/Dockerfile`** | **Dockerfile-backed** profiles; **`TemplateBuild`** maps template name → image + build dir. |
| **`templates/<workflow>/config.yml`** | **Embedded workflows** referenced by **`DOCKPIPE_RESOLVER_WORKFLOW`** (e.g. cursor-dev, vscode). |
| **`scripts/*.sh`** | Host-side helpers; **host isolate** profiles point here via **`DOCKPIPE_RESOLVER_HOST_ISOLATE`**. |

Resolution order for a profile file: **`templates/core/resolvers/<name>`** → **`templates/core/resolvers/<name>/profile`** → legacy **`templates/run-worktree/resolvers/<name>`**. Profiles are **not** read from **`templates/<workflow>/resolvers/`** — custom flows use **workflow** YAML under **`templates/`** or **`templates/<workflow>/`**, not parallel resolver trees.

---

## Adding a new profile (checklist)

1. **Container from a new Dockerfile** — add **`images/<name>/`**, **`TemplateBuild`** case in **`lib/dockpipe/infrastructure/template.go`**, optional **`templates/core/resolvers/<name>`** with **`DOCKPIPE_RESOLVER_TEMPLATE=<name>`** (and docs / env hints).
2. **Reuse an existing image only** — often no new Dockerfile; **resolver** file sets **`DOCKPIPE_RESOLVER_TEMPLATE`** or users pass **`--isolate <image>`** directly.
3. **IDE / long-running host flow** — add **`templates/<myflow>/config.yml`** + **`steps:`**; set **`DOCKPIPE_RESOLVER_WORKFLOW=myflow`** in a resolver profile.
4. **Host-only** — **`DOCKPIPE_RESOLVER_HOST_ISOLATE=scripts/...`**.

---

## Future extension points (doors to open)

These are **not** implemented as separate kinds in the runner today, but the isolation layer is meant to grow here:

| Idea | Possible direction |
|------|---------------------|
| **Docker Compose** | Profile kind **compose** → `docker compose run` / `up` with a defined service name; new keys in profile file or `compose.yml` path. |
| **Raw image URL** | Already partially supported **via** `--isolate` when the value looks like a registry reference; could be first-class in profile files. |
| **Electron / desktop app** | Profile kind **desktop** → host script that launches a binary; same **host isolate** path with richer conventions. |
| **Browser / remote** | **Embedded workflow** (vscode, code-server) **or** host script opening a URL — already covered by **workflow** + **host** patterns. |

When adding a new kind, prefer **one profile file** + **one clear primary key** (e.g. `DOCKPIPE_RESOLVER_COMPOSE_FILE=...`) and keep **`FromResolverMap`** / **domain** in sync — see **`lib/dockpipe/domain/resolver.go`**.

---

## Related docs

- **[workflow-yaml.md](workflow-yaml.md)** — `isolate:`, `resolver:` on steps, `resolvers:` lists  
- **[architecture.md](architecture.md)** — data flow and extension points  
- **[templates/core/README.md](../templates/core/README.md)** — bundled shared tree  
- **Resolver KEY reference** — **`templates/core/resolvers/README.md`**
