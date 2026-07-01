# Workflow YAML (`config.yml`)

**Workflow YAML** for **`--workflow <name>`** resolves to **`workflows/<name>/config.yml`** (when present), then **nested** **`config.yml`** under any directory listed in **`dockpipe.config.json` `compile.workflows`** (same roots **`dockpipe package compile workflows`** uses), then **`src/core/workflows/<name>/config.yml`** (bundled examples in a dockpipe checkout), or **`templates/<name>/config.yml`** (legacy project layout). The **materialized bundle cache** still uses a **`bundle/workflows/`** layout on disk (see **[install.md](install.md#bundled-templates-no-extra-install-tree)**).

If you are new to authoring workflows, start with
**[workflow-authoring.md](workflow-authoring.md)**. This file is the fuller YAML
reference.

## Recommended mental model

For most users, the main concepts are:

- **workflow** — what should happen
- **step** — one host or container action inside that workflow
- **runtime** — where a step runs
- **resolver** — which tool/profile the step uses

Everything else is secondary:

- **`isolate`** is the low-level image/template override
- **`strategy`** is an advanced lifecycle wrapper

If you are writing a normal workflow, prefer thinking in terms of **steps + runtime + resolver** first.

## Defaults and overrides

DockPipe uses a simple layering rule:

- top-level **`runtime`** and **`resolver`** set the **workflow default**
- step-level **`runtime`** and **`resolver`** **override** that default for one step
- **`isolate`** is the advanced low-level override when you need to pin the exact image/template

In practice, most workflows should:

1. set **`runtime`** and **`resolver`** once at the top
2. override them only on the few steps that genuinely differ
3. reach for **`isolate`** only when a step must pin a specific image/template rather than just a substrate/profile

**Workflows vs core slices:** **Runtimes** are **core-owned** execution substrates ( **`templates/core/runtimes/<name>/`** ). Workflow YAML may only **select** a substrate by **name** (`runtime`, per-step `runtime:`) — it does **not** define new runtime types or override how substrates work. **Resolvers** and **strategies** follow the same idea: **definitions** live under **`templates/core/resolvers/`**, **`strategies/`** (or maintainer trees under **`compile.workflows`**); the workflow **references** them — see **Authoring: workflow YAML vs resolver / runtime / strategy slices** in **[package-model.md](package-model.md)** and **[architecture-model.md](architecture-model.md)**. Resolver delegate YAML also loads from **`…/core/resolvers/<name>/config.yml`** next to the authoring core root (**`src/core/resolvers/…`** or **`templates/core/…`**) or **`bundle/core/resolvers/<name>/config.yml`** (materialized bundle). Load with **`dockpipe --workflow <name>`** (plus your command after **`--`**).

**Arbitrary-path workflow:** put the **same** YAML shape in any file (for example **`workflows/foo/config.yml`**) and run **`dockpipe --workflow-file <path>`** so **`run:`** / **`act:`** paths resolve relative to that file’s directory. **Resolver** profiles are **not** beside the file — they load only from **`templates/core/resolvers/`** (see below). Do not pass **`--workflow`** and **`--workflow-file`** together.

**Lint:** **`dockpipe workflow validate [path]`** — parses the workflow (including **`imports:`**) and checks against a small embedded JSON Schema. **`path`** is optional only for the simple flat single-workflow case; organized or multi-workflow trees should pass a **relative** path (resolved from the current directory first, then from **DOCKPIPE_REPO_ROOT** / the materialized bundle root), for example **`workflows/ci/test/config.yml`**.

**Terminology (same as the CLI):**

| Word | Meaning |
|------|---------|
| **run** | Host scripts *before* the container (paths under `run:`). |
| **isolate** | Low-level image or template override. Most workflows should start with **`runtime`** + **`resolver`** and only set **`isolate`** when they want to pin the exact image/template. |
| **image** | Optional image customization for Dockerfile-backed runs. Use **`image.packages.apt`** to declare Debian packages that DockPipe installs into a derived image during package compile/build, instead of bloating every resolver image. Step-level **`image`** adds packages for that step only. |
| **act** | Follow-up after the main command (usually a **host** script; see **[architecture.md](architecture.md)** for in-container `DOCKPIPE_ACTION`). |
| **workflow** | This file: a named preset selected with **`--workflow <name>`**. |
| **strategy** | Optional **named lifecycle** wrapper: small **`KEY=value`** files under **`templates/<workflow>/strategies/<name>`** (optional) or **`templates/core/strategies/<name>`** define host scripts to run **before** and **after** the workflow body. See [Named strategies](#named-strategies) below. |
| **runtime** / **resolver** | **Runtime** — **where** execution runs: **core** profiles under **`templates/core/runtimes/<name>`** (**`DOCKPIPE_RUNTIME_*`**). **Resolver** — **which tool/profile**: **`templates/core/resolvers/<name>`** (**`DOCKPIPE_RESOLVER_*`**). Those are the main selection knobs most workflows should use. In the materialized bundle, the same paths live under **`bundle/core/`**. See **[architecture-model.md](architecture-model.md)** · **[isolation-layer.md](isolation-layer.md)**. |

**Learning path:** [onboarding.md](onboarding.md) · **[architecture-model.md](architecture-model.md)** · **[isolation-layer.md](isolation-layer.md)** · Implementation notes: [`src/lib/README.md`](../src/lib/README.md).

---

## Single-command vs multi-step

| Mode | When | `config.yml` |
|------|------|----------------|
| **Single flow** | No `steps:` (or empty) | Top-level **`run`**, **`runtime`** / **`resolver`** (or advanced **`isolate`**), and **`act`** / **`action`** describe one run. This is the compact shorthand for simple workflows. |
| **Multi-step** | Non-empty **`steps:`** | Each list item is a **step**. A step is either a **container step** (default) or a **host step** (`kind: host`). The CLI argument after `--` can supply the **last** step’s command if that step has no `cmd`/`command`. Top-level **`run`** and **`act`** / **`action`** are not used in this mode. Optional **`finally:`** steps run after the main steps even when a step fails or the run is interrupted. |

Variable precedence for workflows is documented in **[CLI reference](cli-reference.md)** (CLI > config > `.env` / `--env-file` / `--var`).

### Image Packages

Workflow authors can declare extra Debian packages for container steps without changing the base
runtime or resolver image:

```yaml
resolver: codex
image:
  packages:
    apt:
      - golang-go
      - cargo

steps:
  - id: rust-check
    cmd: cargo test
    image:
      packages:
        apt:
          - rustc
```

DockPipe materializes these declarations as a derived Docker image at package compile/build time.
Keep common inspection tools in resolver images; declare heavier project-specific toolchains here.

If a workflow grows beyond one simple run, prefer moving to **`steps:`** instead of piling more meaning onto top-level **`run`** / **`act`**.
Once you switch to **`steps:`**, move those fields onto the specific step that needs them.

Use **`finally:`** for teardown that must happen on failure paths as well:

```yaml
steps:
  - id: stack_up
    kind: host
    run: scripts/dorkpipe/dev-stack-ensure-up.sh

finally:
  - id: stack_down
    kind: host
    run: scripts/dorkpipe/dev-stack-maybe-down.sh
```

### Host steps (`kind: host`)

Most steps are **container steps**. A step becomes a **host step** when you set **`kind: host`**.

For host steps, **`run:`** scripts execute on the host. Use **`kind: host`** when the step genuinely needs host-side behavior, such as:

- starting or stopping a local helper stack
- calling a host-only CLI
- launching a local GUI tool
- preparing files before a later container step

If a host step starts sidecars or helper containers, the runner can clean them up after the step. The exact marker files and cleanup internals are engine behavior; workflow authors usually only need to know that host steps may register cleanup state under **`.dockpipe/`** and that cleanup can be disabled with **`DOCKPIPE_SKIP_HOST_CLEANUP=1`** when a workflow wants to keep the sidecar alive deliberately.

| Variable | Meaning |
|----------|---------|
| **`DOCKPIPE_LAUNCH_MODE`** | Optional hint for **`vars:`** / templates — e.g. **`gui`** means the step opens a **GUI** on the host (desktop app, not a detached “server” process). Scripts can print clearer messaging; dockpipe still **waits on the host script** until it exits unless the script itself returns early. |
| **`DOCKPIPE_SKIP_HOST_CLEANUP`** | If **`1`** or **`true`**, the runner **skips** **`ApplyHostCleanup`** after the host script exits (escape hatch: you stop containers yourself). |

### Step working directory and scopes

Steps run from the source workdir by default. Use **`cwd: repo`** or **`cwd: source`** when
the step should start in the checkout. When a step writes generated files with simple relative
paths, set **`cwd: artifacts`**:

```yaml
steps:
  - id: normalize
    kind: host
    cwd: artifacts
    run:
      - scripts/dorkpipe/normalize-ci-scans.sh
```

DockPipe creates the workflow artifact root under
**`bin/.dockpipe/workflows/<workflow>/artifacts`** and starts the step there. Scripts can write
ordinary relative paths such as **`ci-analysis/findings.json`** without polluting source control.
Scripts should write ordinary relative paths whenever `cwd` already puts them in the right root.
When a script needs an explicit generated path, use **`dockpipe scope`**. If a path must point back
into the checkout, make that explicit with **`dockpipe scope source`**.

When a step needs both worlds, keep **`cwd: repo`** and bind source/artifact scopes explicitly:

```yaml
steps:
  - id: doctor
    kind: host
    cwd: repo
    scopes:
      source: repo
      artifacts: artifacts
    run: assets/scripts/cloud-lanes-doctor.sh
```

In that form the process starts in the checkout, while `dockpipe scope` returns the current workflow
scope object, including `source_root`, `output_root`, and `dockpipe_bin`. Use
`dockpipe scope artifacts <path>` to resolve under the workflow artifact root.
Relative **`outputs:`** files resolve under the output/artifact scope, not the process cwd.

Package state uses the same primitive:

```bash
dockpipe scope --package dorkpipe
dockpipe scope --package dorkpipe training metrics.jsonl
dockpipe scope workflow docs.orchestrate orchestrate
```

The package scope object includes `root`, `state_root`, `workdir`, and `dockpipe_bin`.

Resolver packages can expose resolver-owned paths through their profile, then scripts can read them
without hardcoding provider-specific environment names:

```bash
dockpipe scope resolver codex auth-dir
dockpipe scope resolver codex container-auth-dir
dockpipe scope resolver codex auth-mount-mode
```

DorkPipe agent orchestration also accepts full-string scope references in path lists such as
`agent.accessible_paths`, `agent.access.read/write/deny`, and task `inputs`:

```yaml
inputs:
  - scope:workflow:docs.orchestrate:orchestrate/verify/result.json
  - scope:artifacts:orchestrate/tasks/latest_run_audit/response.md
```

Those references are resolved through the same `dockpipe scope` CLI before prompts and task JSON are
written. Use plain relative paths for normal checkout files.

For task execution lanes, prefer the seeded `worker` profile over repeating provider hints in every
task:

```yaml
tasks:
  - id: scout
    worker: ollama
    worker_type: planning
  - id: patch
    worker: codex
    worker_policy:
      mode: require
    worker_type: patch_decision
```

`worker: codex|claude|ollama` keeps the authoring surface provider-agnostic while DorkPipe maps the
task to seeded lane defaults such as preferred lane family and model provider. By default
`worker_policy.mode: prefer` lets the orchestrator fall back, compare, or escalate when policy says
it should. Use `worker_policy.mode: require` only when the task must stay on that worker profile.
Use `resolver_hint` only as an explicit hard override when a seeded worker profile is not enough.

DockPipe also injects these variables for every step:

| Variable | Meaning |
|----------|---------|
| **`DOCKPIPE_SOURCE_ROOT`** | Absolute source workdir/repo root. Use this when a script running from artifacts needs to read project files. |
| **`DOCKPIPE_ARTIFACT_ROOT`** | Absolute generated artifact root selected by **`scopes.artifacts`**. |
| **`DOCKPIPE_OUTPUT_ROOT`** | Alias for the output/artifact scope used by `dockpipe scope`. |
| **`DOCKPIPE_STEP_CWD`** | Absolute directory DockPipe uses as the process working directory for the step. |

---

## Top-level keys

| Key | Purpose |
|-----|---------|
| `name` | Optional display title for stderr (defaults to the template folder name, e.g. `run`). |
| `description` | Optional one-line task summary printed after `name` (e.g. what this workflow is for). |
| `category` | Optional **UI metadata** for tools like **Pipeon**: e.g. `app` marks a launchable GUI/container IDE-style workflow shown in **Basic** mode. Omit or use other values for pipelines and advanced-only flows. |
| `vars` | Map of default env vars (merged if not already set; `--var` overrides). |
| `compose` | Optional Docker Compose settings for host built-ins such as `compose_up`, `compose_down`, and `compose_ps`. Fields: `file`, `project`, `project_directory`, `autodown_env`, `exports`, `services`. Compose runs inherit DockPipe’s resolved environment and vault-injected vars directly. |
| `container` | Optional container mount defaults. Use this to override the primary host path bound at `/work` (`workdir_host`), set a default container subdirectory under `/work` (`work_path`), and declare extra bind mounts (`mounts`). Relative host paths resolve from the active source/workdir, not the packaged workflow asset directory. |
| `security` | Optional container security policy. Select a core-owned `profile`, then apply bounded `network`, `filesystem`, and `process` overrides. This applies to container execution only; `kind: host` steps remain outside Docker policy. |
| `run` | String or list of host pre-script paths (repo `scripts/…` or paths under the template). Single-flow shorthand only; do not combine with `steps:`. Logical resolver script ids like `scripts/dorkpipe/...` must also have an explicit resolver dependency (`requires_resolvers`, `inject.resolver`, or workflow/step resolver selection). |
| `isolate` | Advanced low-level image/template override. Prefer **`runtime`** + **`resolver`** for the normal authoring path; use **`isolate`** when you need to pin the exact image/template. |
| `act` / `action` | Action script after the container command (when not using per-step act). Single-flow shorthand only; do not combine with `steps:`. |
| `runtime` | Main **where does this run?** field. Names an existing **core** runtime profile under **`templates/core/runtimes/<name>`**. Also acts as the default for steps unless a step overrides it. |
| `resolver` | Main **which tool/profile does this use?** field. Names an existing **core** resolver profile under **`templates/core/resolvers/<name>`**. Also acts as the default for steps unless a step overrides it. |
| `steps` | List of **steps** (multi-step mode). |
| `imports` | List of paths (relative to this file) to merge **before** this file: each imported file’s **`vars`** are merged (later files override), then **`steps`** from imports run **before** **`steps`** here. Circular imports are rejected. Requires loading from disk (not raw bytes-only parse). Use this when you want to change the authored workflow itself. |
| `inject` | Compile-closure dependencies only. Use this when compile needs extra workflows/packages/resolvers included, but you do **not** want to merge their YAML into this workflow. `inject.resolver` also counts as an explicit dependency for logical resolver script ids like `scripts/dorkpipe/...`. |
| `strategy` | Default **strategy name** when the CLI does **not** pass **`--strategy <name>`**. |
| `strategies` | Optional allowlist: if non-empty, the effective strategy (CLI **`--strategy`** or **`strategy:`**) must be one of the listed names. |
| `docker_preflight` | Default **true**. When **false**, the runner skips the Docker reachability check before **`steps:`** if **no** step uses the container. Use for **host-only** workflows whose **`run:`** / **`pre_script`** scripts do **not** invoke Docker. If a script calls **`docker`**, keep the default or the workflow may fail later. |
| `types` | Optional PipeLang type entrypoints used by tooling/catalog/launcher surfaces. Execution still runs through normal workflow YAML and env resolution. |
| `model_policy` | Optional AI model attempt, validation, and escalation policy for DorkPipe harnesses. Use this to express cheap/local attempts, stronger validation, and approval-on-cost escalation intent. |
| `view` | Optional authored launcher presentation layer. Define pages, sections, and ordered field paths over the typed model while keeping the underlying env contract unchanged. |

### `imports` vs `inject`

These two fields solve different problems:

- **`imports`** changes the workflow you are authoring by merging in other YAML
- **`inject`** changes what compile must include, but does **not** merge YAML

If you want more vars or earlier steps to become part of this workflow, use **`imports`**.

If you want compile/package closure to include additional workflows, packages, or resolver profiles without changing this workflow’s authored structure, use **`inject`**.

### `types` and `view`

Use `types:` when you want a workflow to expose a **typed model** to tooling.

Use `view:` when you want to control **how launcher-style clients present that model**.

The intended split is:

- **PipeLang (`types:`)** defines data shape, nested objects, lists, defaults, and docs
- **workflow YAML (`view:`)** defines pages, sections, ordering, and field-path layout
- **workflow YAML (`view:`)** may also define an authored entry choice that writes to a model field and routes the user into the relevant page set
- **`vars:` / env names** remain the runtime contract consumed by the workflow itself

Minimal example:

```yaml
types:
  - ../../resolvers/qemu/models/QemuVmResolverConfig.pipe

view:
  entry:
    type: choice
    field: General.BootSource
    title: Windows VM Source
    options:
      - value: image
        label: Boot existing disk image
        pages: [image]
      - value: installer-iso
        label: Install from ISO
        pages: [install]

  pages:
    - id: image
      title: Existing Image
      sections:
        - id: media
          title: Existing Image
          fields:
            - Storage.Disk
    - id: install
      title: Install Media
      sections:
        - id: media
          title: Images And Media
          fields:
            - Storage.Cdrom
```

Rules:

- `types:` is optional. Workflows without it remain plain YAML/env workflows.
- `view:` is optional. When omitted, launchers may fall back to the older flat/tree rendering.
- `view.entry` is optional. Use it when the launcher should ask a high-level question first and route to the relevant settings pages.
- `view.entry.field` points at a typed model field path, and the chosen option writes that value back into the model/env contract.
- `view.entry.options[].pages[]` and `view.entry.options[].next` describe which page ids should be shown after that choice.
- `view.sections[].fields[]` contains **field paths** into the root typed model, such as `General.BootSource`.
- `view:` is a **presentation** layer only. It does not replace `vars:` or create a second runtime binding mechanism.
- Invalid field paths should be ignored/pruned by tooling rather than breaking workflow execution.

For PipeLang details, see **[pipelang.md](pipelang.md)**.

### `security`

Use `security` to express container hardening intent without dropping to raw Docker flags.

Recommended shape:

```yaml
security:
  profile: secure-default
  network:
    mode: offline
  filesystem:
    root: readonly
    writes: workspace-only
  process:
    user: non-root
    pid_limit: 256
```

Public authoring fields:

- `profile`: `secure-default` | `internet-client` | `build-online` | `sidecar-client`
- `network.mode`: `offline` | `restricted` | `allowlist` | `internet`
- `network.allow`, `network.block`: destination patterns for allowlist/restricted intent
- `filesystem.root`: `readonly` | `writable`
- `filesystem.writes`: `workspace-only` | `declared`
- `filesystem.writable_paths`, `filesystem.temp_paths`
- `process.user`: `auto` | `non-root` | `root`
- `process.pid_limit`
- `process.resources.cpu`, `process.resources.memory`

DockPipe compiles this into an effective runtime policy manifest and derives the actual enforcement mode there. Public workflow YAML does **not** set raw Docker security options or the low-level network enforcement mechanism directly. See **[security-policy.md](security-policy.md)** for the policy model.

---

## Named strategies

**Strategies** are reusable before/after wrappers for a workflow run. Use them when multiple workflows should share the same lifecycle behavior, such as worktree setup or commit-on-success.

Shared definitions live under **`templates/core/strategies/`**; see **[docs/architecture-model.md](architecture-model.md)** ( **`templates/core/`** layout ).

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
resolver: claude
```

Then: **`dockpipe --workflow my-ai --resolver claude --repo https://github.com/you/repo.git -- claude -p "…"`**

Do **not** list **`clone-worktree.sh`** in **`run:`** when **`worktree`** already provides clone (dockpipe will error). Do **not** duplicate bundled **`act:`** commit with the same strategy **`after`** hook.

---

## Step fields

Each **`-`** under `steps:` is one step (or a **`group`** wrapper — see [Async groups](#async-groups-parallelism) below).

| Key | Purpose |
|-----|---------|
| `id` | Optional. Used in stderr logs (e.g. `[merge]` lines). If omitted, logs use `step 1`, `step 2`, … |
| `cmd` / `command` | Shell command line for this step. In a **container** step it runs inside the container. In a **host** step it runs on the host. |
| `run` | String or YAML list: host pre-scripts before this step’s container. Logical resolver script ids like `scripts/dorkpipe/...` require an explicit resolver dependency instead of relying on workflow-package fallback. |
| `pre_script` | Single extra pre-script path (in addition to `run`). |
| `isolate` | Template/image for this step (falls back to workflow / CLI / **core** runtime profile). |
| `kind` | Step kind. Use **`container`** (default) for normal isolated execution, or **`host`** for host-side actions. |
| `cwd` | Step working directory. Omit or use **`source`** / **`repo`** for the checkout; use **`artifacts`** for this workflow's generated artifact root. |
| `scopes` | Optional `{ source, artifacts }` binding for path roots. Use `source: repo` plus `artifacts: artifacts` when the process should run in the checkout but generated output should stay in workflow state. |
| `runtime` | Optional **core** runtime profile basename (same as CLI **`--runtime`** — must exist under **`templates/core/runtimes/`**). Overrides the workflow default for this step. Not meaningful on `kind: host` steps. |
| `resolver` | Optional **resolver** profile basename (same as CLI **`--resolver`**). Overrides the workflow default for this step. Not meaningful on `kind: host` steps. Do **not** use it for packaged workflow calls. |
| `workflow` | Marks this as a **packaged workflow step**. This is the **child workflow name** to run. |
| `package` | Required for a **packaged workflow step**. This is the **child workflow namespace** and must match the nested workflow’s **`namespace:`** in **`config.yml`** (resolution searches packaged / staging / **`workflows/`** trees on disk). |
| `act` / `action` | Action script for this step. Do not combine this with packaged workflow steps. |
| `vars` | Per-step env map (merged for that step; `--var` keys can be “locked”). |
| `agent` | Optional agentic step declaration consumed by DorkPipe-owned scripts/tooling. Use this to declare startup prompt, accessible paths, model knobs, seeded worker profiles, and orchestration fanout directly in `config.yml`. |
| `container` | Optional step-level container mount override. Use this on container steps to change the primary `/work` bind for one step, change its working subdirectory under `/work`, or add extra bind mounts without affecting the whole workflow. |
| `security` | Optional step-level container security override. Use this only on container steps when one step needs a different profile or tighter `network` / `filesystem` / `process` settings than the workflow default. |
| `outputs` | Path to a **dotenv-style** file (`KEY=value` lines) written by the step; merged into env for **later** steps. Relative paths resolve under `DOCKPIPE_OUTPUT_ROOT`, not the process cwd. This is the normal way one step passes values forward to later steps. |
| `capture_stdout` | Host path (relative to **`DOCKPIPE_WORKDIR`** / **`--workdir`**) — container **stdout** is also appended to this file (still printed on the terminal). |
| `manifest` | Host path — after the step, dockpipe writes a small JSON file with **`exit_code`**, **`duration_ms`**, **`step_index`**, **`id`** (if set), and **`step_display`**. |
| `is_blocking` | Default **`true`**. Keep this at its default on normal steps. Async work should use an explicit **`group: { mode: async, tasks: [...] }`** entry instead. |
| `host_builtin` | Optional engine-owned host action for `kind: host` steps. Supported values: `package_build_store`, `compose_up`, `compose_down`, `compose_ps`. Compose built-ins require top-level `compose.file`. |

Step-level `security` follows the same shape as top-level `security`, but it applies only to that one container step. It is not meaningful on `kind: host` steps, and packaged workflow steps should keep their policy inside the child workflow instead of trying to override it from the parent.

Step-level `container` follows the same shape as top-level `container`:

- `workdir_host`: host path to bind at `/work`
- `work_path`: relative container cwd under `/work`
- `mounts`: additional `{ host, guest, mode }` bind mounts, where `mode` is `ro` or `rw`

Like `security`, this is only meaningful on container steps. DockPipe rejects `container:` on `kind: host` steps and on packaged workflow-call steps.

### Agentic steps (`agent`)

Use `agent:` when a step should declare agent behavior directly in workflow YAML while still letting
shared DorkPipe scripts act as the harness.

Typical uses:

- startup prompt text
- whether to include `AGENTS.md` context
- access policy (`read`, `write`, `deny`) plus lightweight accessible path hints
- model/provider knobs that affect generation
- model policy overrides for attempt, validation, and escalation
- orchestration fanout data such as request, tasks, concurrency, merge, verify, and apply policy
- task-level `model_policy` overrides so DorkPipe can choose local/cloud lanes per task without
  hardcoding provider-specific workflow steps

This keeps user intent in `config.yml` and lets shared scripts materialize the contract instead of
forcing one shell file per workflow shape.

`accessible_paths` is a compatibility/simple-hint field. Prefer `access.read`, `access.write`, and
`access.deny` when the workflow needs a real contract that DorkPipe tooling can materialize into
artifacts and, later, stronger runtime policy.

`agent.orchestration.concurrency` lets DorkPipe-owned harnesses cap worker fanout without hiding
scheduling rules in scripts. `max_workers` caps total runnable workers, while `max_local_workers`
and `max_cloud_workers` cap local and cloud-backed lanes separately.

`agent.orchestration.apply` declares approved artifact-to-worktree outputs. DorkPipe harnesses should
write reviewable artifacts first, verify them, require approval when configured, and only then copy
declared outputs into the checkout as normal uncommitted changes.

### Step state flow

For most step-based workflows, the normal pattern is:

1. a step runs
2. it writes values to **`outputs`**
3. later steps see those values in their environment

If you are trying to pass state from one step to another, start with **`outputs`** before inventing custom temp-file or wrapper conventions.

### Host step shape

For normal **`kind: host`** steps, keep the shape simple:

- use **`cmd`** when you want a host shell command
- use **`run`** / **`pre_script`** for host-side setup scripts
- use **`host_builtin`** for engine-owned host actions such as Compose lifecycle or package store build

Do **not** put **`runtime`**, **`resolver`**, or **`isolate`** on a host step. Those fields are for containerized execution.

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
    kind: host
    host_builtin: compose_up

  - id: stack_status
    kind: host
    host_builtin: compose_ps

  - id: stack_down
    kind: host
    host_builtin: compose_down
```

If `compose.autodown_env` is set, `compose_down` is skipped when that env resolves to `0`, `false`, `no`, or `off`.

If `compose.exports` is set, those `KEY=value` pairs are merged into DockPipe’s workflow environment after a successful `compose_up` or `compose_ps`. That makes them available to later steps without an extra env-file layer.

---

## Async groups (parallelism)

**Mental model:** several steps run **concurrently**, then **one merge**, then the **next blocking** step runs with the merged env.

Prefer the explicit **`group: { mode: async }`** form when authoring new workflows. It is easier to read and reason about than relying on adjacency alone.

| Concept | Meaning |
|---------|---------|
| **Async group** | Use a single list entry **`group: { mode: async, tasks: [...] }`**. This is the supported async authoring form. |
| **Join point** | The **next** step with **`is_blocking: true`** (or default). It waits until **every** async member has finished. |
| **Inputs** | Each async member sees env from the **last blocking barrier** only, plus its own `vars` / pre-scripts — not siblings’ live env. |
| **Outputs** | After **all** async members finish, each member’s **`outputs:`** file is merged in **declaration order**. Same key → **later** step wins (same as sequential steps). |
| **Merge logging** | On collision during that merge, stderr shows: `[dockpipe] [merge] variable "KEY" overwritten by … (previously set by …)`. |

**Rules:**

- Within one async group, each step needs a **distinct** `outputs` **path** (duplicate paths are rejected).
- Host **commit-worktree** action cannot be used **inside** an async group.
- `kind: host` steps in a group only contribute at **merge** time (their `outputs` file).

### Recommended async form (`group`)

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
          cmd: sh -c 'echo BRANCH=a > out-a.env'
          outputs: out-a.env
        - id: task_b
          cmd: sh -c 'echo BRANCH=b > out-b.env'
          outputs: out-b.env

  - id: join
    cmd: sh -c 'echo $BRANCH'
    is_blocking: true
```

Inside `tasks:`, omitting `is_blocking` means **async** (forced non-blocking). **`is_blocking: true` inside `tasks`** is **invalid** and errors.

**Do not** nest another `group` inside `tasks` — unsupported; unknown keys on a step are ignored by the YAML decoder.

Plain-step **`is_blocking: false`** is no longer accepted. If you want parallelism, wrap those tasks in an explicit **`group`** entry.
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
| **[workflows/ci/test/](../workflows/ci/test/)** (this repo) | CI-style **go test** + **govulncheck** + **gosec** chain via step `outputs` under the workflow artifact root — canonical repo path is **`workflows/`**, not **`templates/`**. |
| **[templates/run/](../templates/run/)** | Compact single-command shorthand in a container, then optional **git** commit on the current branch (**strategy `git-commit`**). |
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
### Packaged workflow step

Use a packaged workflow call directly on the step:

```yaml
steps:
  - id: child
    workflow: child-name
    package: acme-team
```

That is the explicit nesting form. Do not write `runtime: package`.
