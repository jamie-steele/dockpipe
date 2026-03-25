# dockpipe Go packages (DDD-style layout)

| Layer | Package | Responsibility |
|-------|---------|----------------|
| **Domain** | `dockpipe/src/lib/dockpipe/domain` | Workflow/step model, YAML parse from **bytes** (`ParseWorkflowYAML`), env merge helpers, **resolver** key semantics, branch-prefix rules. No `docker` / subprocess / file I/O in non-test code. |
| **Infrastructure** | `dockpipe/src/lib/dockpipe/infrastructure` | Filesystem, `docker`, `bash` pre-scripts, git commit-on-host, repo root discovery, `.env` files, template→image paths, version tags. **`RunHostScript`** defer: **`ApplyHostCleanup`** (run-scoped: **`.dockpipe/runs/<id>.container`** when **`DOCKPIPE_RUN_ID`** is set; legacy sweep of **`.dockpipe/cleanup/docker-*`** only when the run id is absent) and **`RemoveHostRunArtifacts`** for the host-run registry. |
| **Application** | `dockpipe/src/lib/dockpipe/application` | CLI flags, subcommands (`init`, `template`, …), and the **run** use-case that wires domain + infrastructure. |

`src/cmd/dockpipe` is a thin entrypoint that calls `application.Run`.

### Application package files (baseline)

Keep new orchestration in the right file so `run.go` stays the single-command path only:

| File | Role |
|------|------|
| `run.go` | `Run()` — main CLI flow for `dockpipe [opts] -- cmd` |
| `run_steps.go` | Multi-step workflow loop (`steps:` in config) |
| `workflow_env.go` | Workflow/env merge helpers (`.env`, outputs, branch prefix, `--var` locks) |
| `flags.go` | `CliOpts`, `ParseFlags` |
| `subcmds.go` | `init`, `action`, `pre`, `template` |
| `usage.go` | `--help` text |

Shell assets (`assets/entrypoint.sh` at repo root, etc.) stay outside **`src/`**; only **Go** lives under `src/lib/dockpipe/`.

---

### Multi-step workflows: async group, join point, merge

**End-user / authoritative spec:** **[../../../docs/workflow-yaml.md](../../../docs/workflow-yaml.md)** (keep in sync when behavior changes).

One-liner: **parallel steps share one merge; declaration order decides overwrites; the next blocking step is the join.**

| Idea | In YAML |
|------|---------|
| **Async group** | One or more consecutive steps with **`is_blocking: false`**, **or** one YAML entry **`group: { mode: async, tasks: [...] }`** (sugar for the same thing). They all start after the previous **blocking** step finishes. |
| **Join point** | The next step with **`is_blocking: true`** (or default). It runs only after **every** step in the async group has finished. |
| **Merged inputs** | The join step sees env from **before** the async group, plus **`outputs:`** from each async step merged in **list order** (same key → **later** list entry wins). |
| **Reset** | After a blocking step runs, the next async group starts fresh from that step’s outputs (same as sequential workflows). |

Field names use **snake_case** in YAML (`is_blocking`, not `isBlocking`). Optional **`id:`** on a step labels it in logs (e.g. `[merge]` lines); if omitted, logs use `step 1`, `step 2`, …

**Inputs (inside an async group)**  
Each member sees environment from the **last blocking barrier** only: snapshot at group start, then **only that step’s** `vars:` and `run` / `pre_script`. Siblings do **not** see each other’s env while running.

**Outputs (async group → join)**  
Nothing is merged until **all** async members finish. Then each member’s **`outputs:`** file is applied in **declaration order**. Duplicate keys → **last declarer wins** (same as sequential step-to-step merges). Overwrites are logged to stderr as:

`[dockpipe] [merge] variable "KEY" overwritten by <id or step N> (previously set by …)`

Use **distinct** `outputs:` *file paths* within one async group (duplicate paths are rejected).

**Restrictions**  
Host **commit-worktree** is not allowed inside an async group. **`skip_container`** members only contribute at merge time (their `outputs:` file, in order).

**Optional `group` syntax (readability)** — compiles to consecutive `is_blocking: false` steps; runtime and merge rules are unchanged. A `group` entry must be **only** the key `group` (no sibling keys). `tasks` use the same fields as a normal step (`cmd`, `id`, `outputs`, …). **`is_blocking` inside `tasks`** is ignored except **`is_blocking: true`**, which is rejected.

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

  - id: aggregate
    cmd: sh -c 'echo branch=$BRANCH'
    is_blocking: true   # join; BRANCH=b (last task in group wins on collision)
```

Equivalent flat form (what the parser produces):

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
  - id: aggregate
    cmd: sh -c 'echo branch=$BRANCH'
    is_blocking: true
```
