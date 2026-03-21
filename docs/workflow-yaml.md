# Workflow YAML (`config.yml`)

**Bundled templates** use **`templates/<name>/config.yml`**. Load with **`dockpipe --workflow <name>`** (plus your command after **`--`**).

**Repo-root workflow:** put the **same** YAML shape in **`dockpipe.yml`** (or any path) and run **`dockpipe --workflow-file dockpipe.yml`** so scripts resolve relative to that file’s directory (e.g. **`resolvers/`** next to the file). Do not pass **`--workflow`** and **`--workflow-file`** together.

**Lint:** **`dockpipe workflow validate [path]`** — parses the workflow (including **`imports:`**) and checks against a small embedded JSON Schema. Default path: **`dockpipe.yml`** in the current directory.

**Terminology (same as the CLI):**

| Word | Meaning |
|------|---------|
| **run** | Host scripts *before* the container (paths under `run:`). |
| **isolate** | Container image (and your command after `--`). |
| **act** | Follow-up after the main command (usually a **host** script; see **[architecture.md](architecture.md)** for in-container `DOCKPIPE_ACTION`). |
| **workflow** | This file: a named preset selected with **`--workflow <name>`**. |
| **strategy** | Optional **named lifecycle** wrapper: small **`KEY=value`** files under **`templates/core/strategies/<name>`** (or **`templates/<workflow>/strategies/<name>`** next to this workflow) define host scripts to run **before** and **after** the workflow body. See [Named strategies](#named-strategies) below. |

**Learning path:** [onboarding.md](onboarding.md) · Implementation notes: [`lib/dockpipe/README.md`](../lib/dockpipe/README.md).

---

## Single-command vs multi-step

| Mode | When | `config.yml` |
|------|------|----------------|
| **Single flow** | No `steps:` (or empty) | Top-level **`run`**, **`isolate`**, **`act`** / **`action`** (and optional **`vars:`**) set pre-scripts, image, and action script. |
| **Multi-step** | Non-empty **`steps:`** | Each list item is a **step** (container + optional pre-scripts). The CLI argument after `--` can supply the **last** step’s command if that step has no `cmd`/`command`. |

Variable precedence for workflows is documented in **[CLI reference](cli-reference.md)** (CLI > config > `.env` / `--env-file` / `--var`).

---

## Top-level keys

| Key | Purpose |
|-----|---------|
| `name` | Optional display title for stderr (defaults to the template folder name, e.g. `run-worktree`). |
| `description` | Optional one-line task summary printed after `name` (e.g. what this workflow is for). |
| `vars` | Map of default env vars (merged if not already set; `--var` overrides). |
| `run` | String or list of host pre-script paths (repo `scripts/…` or paths under the template). |
| `isolate` | Template name or image for the container. For **resolver-driven** workflows (e.g. **run-worktree**), prefer **`default_resolver`** to pick **`resolvers/<name>`**; **`isolate`** still works as a **legacy** default resolver name when **`default_resolver`** is empty. |
| `act` / `action` | Action script after the container command (when not using per-step act). |
| `resolver` | Default resolver name (**multi-step** workflows). |
| `default_resolver` | Default resolver name (**single-flow** workflows); takes precedence over **`isolate`** for selecting **`resolvers/<name>`**. |
| `steps` | List of **steps** (multi-step mode). |
| `imports` | List of paths (relative to this file) to merge **before** this file: each imported file’s **`vars`** are merged (later files override), then **`steps`** from imports run **before** **`steps`** here. Circular imports are rejected. Requires loading from disk (not raw bytes-only parse). |
| `strategy` | Default **strategy name** when the CLI does **not** pass **`--strategy <name>`**. |
| `strategies` | Optional allowlist: if non-empty, the effective strategy (CLI **`--strategy`** or **`strategy:`**) must be one of the listed names. |

---

## Named strategies

**Strategies** wrap the workflow body with optional **host** scripts **before** and **after** success (same spirit as **`resolvers/`** small files). Shared definitions live under **`templates/core/strategies/`**; see **[templates/core/README.md](../templates/core/README.md)**.

**Resolution order** for the strategy file path: **`--strategy <name>`** (overrides **`strategy:`** in YAML when both are set) → **`templates/<this-workflow>/strategies/<name>`** (if present) → **`templates/core/strategies/<name>`** → legacy **`templates/strategies/<name>`**.

**File format** (`KEY=value`, `#` comments):

| Key | Meaning |
|-----|--------|
| `DOCKPIPE_STRATEGY_BEFORE` | Comma-separated repo-relative script paths (under **`scripts/…`** from repo root, or paths relative to the workflow template dir), run **in order** on the host before **`run:`** / the container / **`steps:`**. |
| `DOCKPIPE_STRATEGY_AFTER` | Same, run **after** the workflow body completes successfully (exit **0**). Bundled **`scripts/commit-worktree.sh`** is treated like today’s commit-on-host path (single commit, no duplicate with workflow **`act:`** when the strategy owns commit). |
| `DOCKPIPE_STRATEGY_KIND` | Optional tag (e.g. `git`) for documentation only. |

**Built-in names** (bundled under **`templates/core/strategies/`**):

| Name | Role |
|------|------|
| **`git-worktree`** | **`before`:** `scripts/clone-worktree.sh` · **`after`:** `scripts/commit-worktree.sh` — worktree + resolver flows (**legacy template folder** **`run-worktree`** uses this). |
| **`git-commit`** | **`after`:** commit only — e.g. **`commit-run`** workflow. |

Do **not** list **`clone-worktree.sh`** in **`run:`** when **`git-worktree`** already provides clone (dockpipe will error). Do **not** duplicate bundled **`act:`** commit with the same strategy **`after`** hook.

---

## Step fields

Each **`-`** under `steps:` is one step (or a **`group`** wrapper — see [Async groups](#async-groups-parallelism) below).

| Key | Purpose |
|-----|---------|
| `id` | Optional. Used in stderr logs (e.g. `[merge]` lines). If omitted, logs use `step 1`, `step 2`, … |
| `cmd` / `command` | Shell command line inside the container (parsed for argv). |
| `run` | String or YAML list: host pre-scripts before this step’s container. |
| `pre_script` | Single extra pre-script path (in addition to `run`). |
| `isolate` | Template/image for this step (falls back to workflow / CLI / resolver). |
| `resolver` | Optional **name** of **`resolvers/<name>`** next to this workflow (same folder layout as **run-worktree**). Loads **`DOCKPIPE_RESOLVER_TEMPLATE`**, **`DOCKPIPE_RESOLVER_WORKFLOW`** (runs **`templates/<name>/config.yml`** via the same runner), **`DOCKPIPE_RESOLVER_HOST_ISOLATE`**, and default **`act`** when the step does not set them. **`isolate:`** on the step still overrides the template from the resolver file. Workflow or host-isolate resolvers cannot run in **async** groups (`is_blocking: false`); use a blocking step. |
| `act` / `action` | Action script for this step. |
| `vars` | Per-step env map (merged for that step; `--var` keys can be “locked”). |
| `outputs` | Path to a **dotenv-style** file (`KEY=value` lines) written by the step; merged into env for **later** steps. Default if omitted: `.dockpipe/outputs.env`. |
| `capture_stdout` | Host path (relative to **`DOCKPIPE_WORKDIR`** / **`--workdir`**) — container **stdout** is also appended to this file (still printed on the terminal). |
| `manifest` | Host path — after the step, dockpipe writes a small JSON file with **`exit_code`**, **`duration_ms`**, **`step_index`**, **`id`** (if set), and **`step_display`**. |
| `skip_container` | If `true`, no container: only pre-scripts + merge `outputs` from disk. **`run:`** scripts are **executed** with inherited stdio (so messages and launchers are visible). Steps that use the container still **source** `run:` scripts to capture exported env (see `lib/dockpipe/infrastructure/prescript.go`). |
| `is_blocking` | Default **`true`**. If **`false`**, this step joins an **async group** with adjacent non-blocking steps (see below). |

All keys use **snake_case** in YAML (e.g. `is_blocking`, not `isBlocking`).

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

Multiple **separate** `dockpipe` invocations (same `--workdir`) are still valid; see **[chaining.md](chaining.md)**. Use **`steps:`** when you want one workflow file to own ordering, outputs, and optional parallelism.

---

## Example templates in this repo

| Template | Purpose |
|----------|---------|
| **[templates/workflow-demo/](../templates/workflow-demo/)** | Runnable demo: **`group.mode: async`**, distinct **`outputs:`**, blocking **join**; uses **`alpine`**. |
| **[templates/chain-test/](../templates/chain-test/)** | Minimal **two-step** sequential chain via **`.dockpipe/outputs.env`**. |

Run from repo root: `dockpipe --workflow workflow-demo` (see each template’s README).

---

## See also

- **[CLI reference](cli-reference.md)** — flags, `--workflow`, `--workflow-file`, `workflow validate`, `--var`, `--env-file`.
- **[Architecture](architecture.md)** — how the Go CLI runs steps, docker, pre-scripts.
- **[lib/dockpipe/README.md](../lib/dockpipe/README.md)** — package layout and contributor-oriented notes.
