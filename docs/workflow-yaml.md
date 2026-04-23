# Workflow YAML (`config.yml`)

**Workflow YAML** for **`--workflow <name>`** resolves to **`workflows/<name>/config.yml`** (when present), then **nested** **`config.yml`** under any directory listed in **`dockpipe.config.json` `compile.workflows`** (same roots **`dockpipe package compile workflows`** uses), then **`src/core/workflows/<name>/config.yml`** (bundled examples in a dockpipe checkout), or **`templates/<name>/config.yml`** (legacy project layout). The **materialized bundle cache** still uses a **`bundle/workflows/`** layout on disk (see **[install.md](install.md#bundled-templates-no-extra-install-tree)**).

**Workflows vs core slices:** **Runtimes** are **core-owned** execution substrates ( **`templates/core/runtimes/<name>/`** ). Workflow YAML may only **select** a substrate by **name** (`runtime`, `default_runtime`, `runtimes:` allowlist, per-step `runtime:`) — it does **not** define new runtime types or override how substrates work. **Resolvers** and **strategies** follow the same idea: **definitions** live under **`templates/core/resolvers/`**, **`strategies/`** (or maintainer trees under **`compile.workflows`**); the workflow **references** them — see **Authoring: workflow YAML vs resolver / runtime / strategy slices** in **[package-model.md](package-model.md)** and **[architecture-model.md](architecture-model.md)**. Resolver delegate YAML also loads from **`…/core/resolvers/<name>/config.yml`** next to the authoring core root (**`src/core/resolvers/…`** or **`templates/core/…`**) or **`bundle/core/resolvers/<name>/config.yml`** (materialized bundle). Load with **`dockpipe --workflow <name>`** (plus your command after **`--`**).

**Arbitrary-path workflow:** put the **same** YAML shape in any file (for example **`workflows/foo/config.yml`**) and run **`dockpipe --workflow-file <path>`** so **`run:`** / **`act:`** paths resolve relative to that file’s directory. **Resolver** profiles are **not** beside the file — they load only from **`templates/core/resolvers/`** (see below). Do not pass **`--workflow`** and **`--workflow-file`** together.

**Lint:** **`dockpipe workflow validate [path]`** — parses the workflow (including **`imports:`**) and checks against a small embedded JSON Schema. **`path`** is optional when exactly one **`workflows/*/config.yml`** exists under the workflows root. Otherwise pass a **relative** path (resolved from the current directory first, then from **DOCKPIPE_REPO_ROOT** / the materialized bundle root), for example **`workflows/test/config.yml`**.

**Terminology (same as the CLI):**

| Word | Meaning |
|------|---------|
| **run** | Host scripts *before* the container (paths under `run:`). |
| **isolate** | Container image (and your command after `--`). |
| **act** | Follow-up after the main command (usually a **host** script; see **[architecture.md](architecture.md)** for in-container `DOCKPIPE_ACTION`). |
| **workflow** | This file: a named preset selected with **`--workflow <name>`**. |
| **strategy** | Optional **named lifecycle** wrapper: small **`KEY=value`** files under **`templates/<workflow>/strategies/<name>`** (optional) or **`templates/core/strategies/<name>`** define host scripts to run **before** and **after** the workflow body. See [Named strategies](#named-strategies) below. |
| **runtime** / **resolver** | **Runtime** — **where** execution runs: **core** profiles under **`templates/core/runtimes/<name>`** (**`DOCKPIPE_RUNTIME_*`**). Workflows **choose** a profile name; they do **not** add substrates. **Resolver** — **which tool**: **`templates/core/resolvers/<name>`** (**`DOCKPIPE_RESOLVER_*`**). In the materialized bundle, the same paths live under **`bundle/core/`**. Both may be set; the runner **merges** them. See **[architecture-model.md](architecture-model.md)** · **[isolation-layer.md](isolation-layer.md)**. Optional **`runtimes:`** lists **which core substrate names** remain valid when more than one is in play; when omitted, **`runtime`** / **`default_runtime`** imply a minimal allowlist (no duplicate **`runtimes: [dockerimage]`** next to **`runtime: dockerimage`**). |

**Learning path:** [onboarding.md](onboarding.md) · **[architecture-model.md](architecture-model.md)** · **[isolation-layer.md](isolation-layer.md)** · Implementation notes: [`src/lib/README.md`](../src/lib/README.md).

---

## Single-command vs multi-step

| Mode | When | `config.yml` |
|------|------|----------------|
| **Single flow** | No `steps:` (or empty) | Top-level **`run`**, **`isolate`**, **`act`** / **`action`** (and optional **`vars:`**) set pre-scripts, image, and action script. |
| **Multi-step** | Non-empty **`steps:`** | Each list item is a **step** (container + optional pre-scripts). The CLI argument after `--` can supply the **last** step’s command if that step has no `cmd`/`command`. |

Variable precedence for workflows is documented in **[CLI reference](cli-reference.md)** (CLI > config > `.env` / `--env-file` / `--var`).

### Host `skip_container` lifecycle (core)

For steps with **`skip_container: true`**, **`run:`** scripts execute on the host via **`RunHostScript`**. When they start sidecars (e.g. **`docker run -d`**), templates should register what to tear down: **`workdir/.dockpipe/runs/<DOCKPIPE_RUN_ID>.container`** (one line: the Docker container name for this host run) and, for humans and older flows, **`workdir/.dockpipe/cleanup/docker-*`** (same one-line format) plus optional legacy **`.dockpipe/cursor-dev/session_container`**. After the host script exits, **`ApplyHostCleanup`** stops **only** the container recorded for **this** run id (and removes matching **`cleanup/`** markers); it does **not** sweep every file under **`cleanup/`** from other runs. If **`DOCKPIPE_RUN_ID`** is unset (no host-run registry), the runner falls back to the legacy behavior: **`docker stop`** for each name still listed under **`cleanup/docker-*`** and the session marker. This is the **core** cleanup path—not resolver-specific logic in templates.

| Variable | Meaning |
|----------|---------|
| **`DOCKPIPE_LAUNCH_MODE`** | Optional hint for **`vars:`** / templates — e.g. **`gui`** means the step opens a **GUI** on the host (desktop app, not a detached “server” process). Scripts can print clearer messaging; dockpipe still **waits on the host script** until it exits unless the script itself returns early. |
| **`DOCKPIPE_SKIP_HOST_CLEANUP`** | If **`1`** or **`true`**, the runner **skips** **`ApplyHostCleanup`** after the host script exits (escape hatch: you stop containers yourself). |

---

## Top-level keys

| Key | Purpose |
|-----|---------|
| `name` | Optional display title for stderr (defaults to the template folder name, e.g. `run`). |
| `description` | Optional one-line task summary printed after `name` (e.g. what this workflow is for). |
| `capability` | Dotted **capability** id (e.g. `cli.codex`, **`dockpipe.workflow.*`**). When **`resolver:`** is omitted, the runner picks **`templates/core/resolvers/<name>`** from resolver **`package.yml`** files that declare this capability. **`dockpipe.*`** ids may imply **`runtime:`** when unset (see **[capabilities.md](capabilities.md)**). Deprecated alias: **`primitive`**. |
| `category` | Optional UI hint for tools like **Pipeon**: e.g. `app` marks a launchable GUI/container IDE-style workflow shown in **Basic** mode. Omit or use other values for pipelines and advanced-only flows. |
| `vars` | Map of default env vars (merged if not already set; `--var` overrides). |
| `compose` | Optional Docker Compose settings for host built-ins such as `compose_up`, `compose_down`, and `compose_ps`. Fields: `file`, `project`, `project_directory`, `autodown_env`, `exports`, `services`. Compose runs inherit DockPipe’s resolved environment and vault-injected vars directly. |
| `run` | String or list of host pre-script paths (repo `scripts/…` or paths under the template). |
| `isolate` | Template name or image for the container. For **resolver-driven** flows with **`strategy: worktree`**, prefer **`default_runtime`** / **`default_resolver`** to pick **core** profile names; **`isolate`** remains a **fallback** default when those are empty. |
| `act` / `action` | Action script after the container command (when not using per-step act). |
| `runtime` | **Core** runtime profile (**substrate**) for **single-flow** — basename under **`templates/core/runtimes/<name>`**. Selects **where** to run; does **not** define a new substrate. |
| `default_runtime` | Fallback **core** runtime profile name when **`runtime`** is unset (**single-flow**). Still only names an existing **`templates/core/runtimes/<name>`** tree. |
| `runtimes` | Optional allowlist of **core** substrate basenames when multiple substrates are valid (e.g. **`[dockerimage, dockerfile]`**). Values must match **bundled** runtime profiles — not an extension point. If omitted, **`runtime`** / **`default_runtime`** (when set) imply the allowlist — you do not need a one-element array that repeats **`runtime`**. |
| `resolver` | Default **resolver** (tool) profile name for **multi-step** workflows (basename under **`templates/core/resolvers/<name>`**). For **single-flow**, prefer **`default_resolver`**. |
| `default_resolver` | Default **resolver** profile (**single-flow**); takes precedence over **`isolate`** for selecting a **core** shared tool profile. |
| `steps` | List of **steps** (multi-step mode). |
| `imports` | List of paths (relative to this file) to merge **before** this file: each imported file’s **`vars`** are merged (later files override), then **`steps`** from imports run **before** **`steps`** here. Circular imports are rejected. Requires loading from disk (not raw bytes-only parse). |
| `strategy` | Default **strategy name** when the CLI does **not** pass **`--strategy <name>`**. |
| `strategies` | Optional allowlist: if non-empty, the effective strategy (CLI **`--strategy`** or **`strategy:`**) must be one of the listed names. |
| `docker_preflight` | Default **true**. When **false**, the runner skips the Docker reachability check before **`steps:`** if **no** step uses the container (**`skip_container`** omitted or false on any step still forces the check). Use for **host-only** workflows whose **`run:`** / **`pre_script`** scripts do **not** invoke Docker. If a script calls **`docker`**, keep the default or the workflow may fail later. |

---

## Named strategies

**Strategies** wrap the workflow body with optional **host** scripts **before** and **after** success (same spirit as **`resolvers/`** small files). Shared definitions live under **`templates/core/strategies/`**; see **[docs/architecture-model.md](architecture-model.md)** ( **`templates/core/`** layout ).

**Resolution order** for the strategy file path: **`--strategy <name>`** (overrides **`strategy:`** in YAML when both are set) → **`templates/<this-workflow>/strategies/<name>`** (beside that workflow’s `config.yml`, if present) → **`templates/core/strategies/<name>`** (see **`ResolveStrategyFilePath`** in **`src/lib/application/strategy.go`**).

**File format** (`KEY=value`, `#` comments):

| Key | Meaning |
|-----|--------|
| `DOCKPIPE_STRATEGY_BEFORE` | Comma-separated repo-relative script paths (under **`scripts/…`** from repo root, or paths relative to the workflow template dir), run **in order** on the host before **`run:`** / the container / **`steps:`**. |
| `DOCKPIPE_STRATEGY_AFTER` | Same, run **after** the workflow body completes successfully (exit **0**). Bundled **`scripts/commit-worktree.sh`** is treated like today’s commit-on-host path (single commit, no duplicate with workflow **`act:`** when the strategy owns commit). |
| `DOCKPIPE_STRATEGY_KIND` | Optional tag (e.g. `git`) for documentation only. |

**Built-in names** (bundled under **`templates/core/strategies/`**):

| Name | Role |
|------|------|
| **`worktree`** | **`before`:** `scripts/clone-worktree.sh` · **`after`:** `scripts/commit-worktree.sh` — clone/worktree on the host, then resolver-driven isolate, then commit. There is **no** separate bundled workflow for this; add **`strategy: worktree`** to **your** `templates/<name>/config.yml` (or **`--workflow-file`**). |
| **`commit`** | **`after`:** commit only — e.g. **`run`** workflow. |

**Example** (your repo, e.g. **`templates/my-ai/config.yml`**):

```yaml
name: my-ai
strategy: worktree
strategies: [worktree, commit]
default_resolver: claude
```

Then: **`dockpipe --workflow my-ai --resolver claude --repo https://github.com/you/repo.git -- claude -p "…"`**

Do **not** list **`clone-worktree.sh`** in **`run:`** when **`worktree`** already provides clone (dockpipe will error). Do **not** duplicate bundled **`act:`** commit with the same strategy **`after`** hook.

---

## Step fields

Each **`-`** under `steps:` is one step (or a **`group`** wrapper — see [Async groups](#async-groups-parallelism) below).

| Key | Purpose |
|-----|---------|
| `id` | Optional. Used in stderr logs (e.g. `[merge]` lines). If omitted, logs use `step 1`, `step 2`, … |
| `cmd` / `command` | Shell command line inside the container (parsed for argv). |
| `run` | String or YAML list: host pre-scripts before this step’s container. |
| `pre_script` | Single extra pre-script path (in addition to `run`). |
| `isolate` | Template/image for this step (falls back to workflow / CLI / **core** runtime profile). |
| `runtime` | Optional **core** runtime profile basename (same as CLI **`--runtime`** — must exist under **`templates/core/runtimes/`**). **Only** **`runtime: package`** enters a **packaged** workflow from a parent step — there is no other nesting shape (see **[architecture-model.md — Workflows and packaged workflows](architecture-model.md#workflows-and-packaged-workflows-same-spine)**). Otherwise pairs with **`resolver:`**; merged by the runner. |
| `resolver` | Optional **resolver** profile basename (same as CLI **`--resolver`**), **or** when **`runtime: package`**, the **nested workflow name** (directory / workflow id). **May** be set together with **`runtime:`** on the same step (except **`runtime: package`** uses **`resolver`** only as that workflow name). **`isolate:`** can still override the template. Profiles that delegate to host or embedded workflows cannot run in **async** groups (`is_blocking: false`); use a blocking step. |
| `package` | Required when **`runtime: package`** (the **only** way to invoke a packaged workflow from a parent). **Namespace** label — must match the nested workflow’s **`namespace:`** in **`config.yml`** (resolution searches packaged / staging / **`workflows/`** trees on disk). |
| `capability` | Optional per-step capability — when set (and **`resolver:`** omitted), the runner resolves the resolver profile from **`package.yml`** like the workflow-level rule. **`dockpipe.*`** may imply **`runtime:`** for the step. |
| `act` / `action` | Action script for this step. |
| `vars` | Per-step env map (merged for that step; `--var` keys can be “locked”). |
| `outputs` | Path to a **dotenv-style** file (`KEY=value` lines) written by the step; merged into env for **later** steps. Default if omitted: `.dockpipe/outputs.env`. |
| `capture_stdout` | Host path (relative to **`DOCKPIPE_WORKDIR`** / **`--workdir`**) — container **stdout** is also appended to this file (still printed on the terminal). |
| `manifest` | Host path — after the step, dockpipe writes a small JSON file with **`exit_code`**, **`duration_ms`**, **`step_index`**, **`id`** (if set), and **`step_display`**. |
| `skip_container` | If `true`, no container: only pre-scripts + merge `outputs` from disk. **`run:`** scripts are **executed** with inherited stdio (so messages and launchers are visible). Steps that use the container still **source** `run:` scripts to capture exported env (see `src/lib/infrastructure/prescript.go`). |
| `is_blocking` | Default **`true`**. If **`false`**, this step joins an **async group** with adjacent non-blocking steps (see below). |
| `host_builtin` | Optional engine-owned host action for `skip_container: true` steps. Supported values: `package_build_store`, `compose_up`, `compose_down`, `compose_ps`. Compose built-ins require top-level `compose.file`. |

All keys use **snake_case** in YAML (e.g. `is_blocking`, not `isBlocking`).

### Compose lifecycle example

```yaml
name: stack-demo
compose:
  file: assets/compose/docker-compose.yml
  project: dockpipe-dev
  project_directory: ../../..
  autodown_env: STACK_AUTODOWN
  exports:
    OLLAMA_HOST: http://host.docker.internal:11434
  services: [proxy]

steps:
  - id: stack_up
    skip_container: true
    host_builtin: compose_up

  - id: stack_status
    skip_container: true
    host_builtin: compose_ps

  - id: stack_down
    skip_container: true
    host_builtin: compose_down
```

If `compose.autodown_env` is set, `compose_down` is skipped when that env resolves to `0`, `false`, `no`, or `off`.

If `compose.exports` is set, those `KEY=value` pairs are merged into DockPipe’s workflow environment after a successful `compose_up` or `compose_ps`. That makes them available to later steps without an extra env-file layer.

---

## Async groups (parallelism)

**Mental model:** several steps run **concurrently**, then **one merge**, then the **next blocking** step runs with the merged env.

| Concept | Meaning |
|---------|---------|
| **Async group** | One or more **consecutive** steps with **`is_blocking: false`**, **or** a single list entry **`group: { mode: async, tasks: [...] }`** (syntactic sugar). |
| **Join point** | The **next** step with **`is_blocking: true`** (or default). It waits until **every** async member has finished. |
| **Inputs** | Each async member sees env from the **last blocking barrier** only, plus its own `vars` / pre-scripts — not siblings’ live env. |
| **Outputs** | After **all** async members finish, each member’s **`outputs:`** file is merged in **declaration order**. Same key → **later** step wins (same as sequential steps). |
| **Merge logging** | On collision during that merge, stderr shows: `[dockpipe] [merge] variable "KEY" overwritten by … (previously set by …)`. |

**Rules:**

- Within one async group, each step needs a **distinct** `outputs` **path** (duplicate paths are rejected).
- Host **commit-worktree** action cannot be used **inside** an async group.
- `skip_container` steps in a group only contribute at **merge** time (their `outputs` file).

### Flat async (explicit `is_blocking`)

```yaml
steps:
  - id: setup
    cmd: echo ready
    is_blocking: true

  - id: task_a
    cmd: sh -c 'echo BRANCH=a > .dockpipe/out-a.env'
    is_blocking: false
    outputs: .dockpipe/out-a.env

  - id: task_b
    cmd: sh -c 'echo BRANCH=b > .dockpipe/out-b.env'
    is_blocking: false
    outputs: .dockpipe/out-b.env

  - id: join
    cmd: sh -c 'echo $BRANCH'
    is_blocking: true   # BRANCH=b (last in group wins)
```

### `group` sugar (same runtime as above)

The entry must contain **only** the key `group` (no `cmd` beside it).

```yaml
steps:
  - id: setup
    cmd: echo ready
    is_blocking: true

  - group:
      mode: async
      tasks:
        - id: task_a
          cmd: sh -c 'echo BRANCH=a > .dockpipe/out-a.env'
          outputs: .dockpipe/out-a.env
        - id: task_b
          cmd: sh -c 'echo BRANCH=b > .dockpipe/out-b.env'
          outputs: .dockpipe/out-b.env

  - id: join
    cmd: sh -c 'echo $BRANCH'
    is_blocking: true
```

Inside `tasks:`, omitting `is_blocking` means **async** (forced non-blocking). **`is_blocking: true` inside `tasks`** is **invalid** and errors.

**Do not** nest another `group` inside `tasks` — unsupported; unknown keys on a step are ignored by the YAML decoder.

---

## Chaining without `steps:`

Multiple **`dockpipe`** runs with the same **`--workdir`** are valid (fresh container each time). Example:

```bash
dockpipe --workdir "$R" -- make lint && dockpipe --workdir "$R" -- make test
```

Pipe stdout between runs if needed. Prefer **`steps:`** in **`config.yml`** when one workflow should own order, **`outputs:`**, and optional parallelism.

---

## Example workflows in this repo

| Workflow | Purpose |
|----------|---------|
| **[workflows/test/](../workflows/test/)** (this repo) | CI-style **go test** + **govulncheck** + **gosec** chain via **`.dockpipe/outputs.env`** — canonical repo path is **`workflows/`**, not **`templates/`**. |
| **[templates/run/](../templates/run/)** | Single command in a container, then optional **git** commit on the current branch (**strategy `git-commit`**). |
| **[templates/run-apply/](../templates/run-apply/)** | Two-step **run → apply** pipeline (replace **`cmd:`** with your tools). |
| **[templates/run-apply-validate/](../templates/run-apply-validate/)** | Three-step **run → apply → validate** pipeline (replace **`cmd:`** with your tools). |

**Async groups** (`group.mode: async`) are documented above in this file.

```bash
dockpipe --workflow test
dockpipe --workflow run -- echo ok
dockpipe --workflow run-apply
dockpipe --workflow run-apply-validate
```

---

## See also

- **[capabilities.md](capabilities.md)** — abstract **capabilities**, **resolver** packages, **`capability:`** / **`requires_capabilities:`**

- **[CLI reference](cli-reference.md)** — flags, `--workflow`, `--workflow-file`, `workflow validate`, `--var`, `--env-file`.
- **[Architecture](architecture.md)** — how the Go CLI runs steps, docker, pre-scripts.
- **[src/lib/README.md](../src/lib/README.md)** — package layout and contributor-oriented notes.
