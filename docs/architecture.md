# dockpipe architecture

This document describes the core primitive, data flow, and extension points.

---

## Primitive

Mental model (same as **[onboarding.md](onboarding.md)**): **run** (host prep) → **isolate** (container) → **act** (host follow-up). Under the hood:

1. **Run** (host) — Optional scripts before the container (`--run`), e.g. clone-worktree.
2. **Isolate** — Container from image or template (`--isolate`); your argv after `--` runs **inside** it with the project at **`/work`**.
3. **Act** — Optional script after the main command. **Usually** the entrypoint runs it **inside** the container (`DOCKPIPE_ACTION`). **Exception:** the bundled **`scripts/commit-worktree.sh`** is detected and run **on the host** after the container exits (normal `git` against the mounted worktree); the container does not set `DOCKPIPE_ACTION` in that case. Container exit code is still the main command's exit code.

No built-in commit, clone, or AI logic — those are scripts you plug in.

**Lifecycle:** By default the container runs **attached** (stdin/stdout connected). Closing the terminal or disconnecting the client sends SIGTERM; the entrypoint stops the command and the container exits (`--rm` removes it). Use **`-d` / `--detach`** to run in the background (no attach); the container stays up until the command inside exits.

---

## Components

| Component | Role |
|-----------|------|
| **Isolation layer** | Named **resolver profiles** (`KEY=value` under **`templates/core/resolvers/`**): **runtime** wiring (`DOCKPIPE_RUNTIME_*`) + tool integration (**`DOCKPIPE_RESOLVER_*`**); **`DOCKPIPE_RUNTIME_TYPE`** = **`runtime.type`**. See **[architecture-model.md](architecture-model.md)** and **[isolation-layer.md](isolation-layer.md)**. |
| `bin/dockpipe` | Launcher: runs **`bin/dockpipe.bin`** if present (`make`), otherwise **`go run ./cmd/dockpipe`**. |
| `cmd/dockpipe`, `lib/dockpipe/application`, `lib/dockpipe/domain`, `lib/dockpipe/infrastructure` | **Go** CLI (DDD-ish): application layer (flags + orchestration + `windows setup/doctor`), domain (workflow/env/resolver semantics), infrastructure (FS, docker, bash, git). **`config.yml`** / **`steps:`** (YAML v3), resolver `KEY=value` files, template→image map, bash `source` for pre-scripts, **`docker run`** / build, host **git** commit. |
| **Embedded bundle** (`embed.go`) | Stock **`templates/`** (including full **`templates/core/`**: **`assets/`** — scripts, images, compose — plus runtimes, resolvers, strategies), **`lib/entrypoint.sh`**, **`VERSION`** compiled into the binary; materialized to the **user cache** at runtime (override with **`DOCKPIPE_REPO_ROOT`** for development). |
| `lib/entrypoint.sh` | Container entrypoint: run command, then `DOCKPIPE_ACTION` if set (skipped when act is **host** commit — see bundled `commit-worktree`). |
| `templates/core/assets/images/*/Dockerfile` | Framework images for **`TemplateBuild`**; each copies `lib/entrypoint.sh` as `ENTRYPOINT` (build context: repo root). |
| `templates/core/assets/scripts/*.sh` | Shared host scripts (clone/commit, resolver helpers); YAML still uses **`scripts/…`** (resolved to project **`scripts/`** or **`templates/core/assets/scripts/`**). |
| `templates/*/` | Workflow templates (`config.yml`). Multi-step / async: see **[workflow-yaml.md](workflow-yaml.md)**. |

---

## Data flow (default: Go CLI)

```
User: dockpipe --isolate claude --act scripts/commit-worktree.sh -- claude -p "..."

  bin/dockpipe → dockpipe (Go binary when packaged / built)
    → TemplateBuild("claude") from --isolate → image=dockpipe-claude, build=.../templates/core/assets/images/claude
    → docker build (if needed) / docker run (no DOCKPIPE_ACTION for bundled commit-worktree)
    → see lib/dockpipe/infrastructure/docker.go

  Container (lib/entrypoint.sh)
    → cd /work
    → exec user argv (e.g. claude -p "...")
    → save exit code
    → run DOCKPIPE_ACTION if set (not set for bundled commit-worktree)
    → exit with saved exit code

  Host (after container exits, if --act is bundled commit-worktree)
    → host git commit / bundle (CommitOnHost path)
```

**`--workflow` with `steps:`** — `lib/dockpipe/application/run_steps.go` runs each step (optional parallel async groups, merge `outputs:` in order). Spec: **[workflow-yaml.md](workflow-yaml.md)**.

Environment variables that cross the boundary:

- **Host → container (CLI / runner):** `DOCKPIPE_CONTAINER_WORKDIR`, `DOCKPIPE_ACTION` (path inside container), plus any `--env` and image defaults.
- **Entrypoint → action:** `DOCKPIPE_EXIT_CODE`, `DOCKPIPE_CONTAINER_WORKDIR`.

---

## Extension points

1. **Isolation profiles** — Add or extend a file under **`templates/core/resolvers/<name>`** (see **[isolation-layer.md](isolation-layer.md)**): **`DOCKPIPE_RESOLVER_TEMPLATE`** (Docker), **`DOCKPIPE_RESOLVER_WORKFLOW`** (embedded `templates/<wf>/config.yml`), or **`DOCKPIPE_RESOLVER_HOST_ISOLATE`** (host script). CLI **`--resolver`** selects the profile name for resolver-driven workflows.
2. **Images** — Add a Dockerfile under **`templates/core/assets/images/<name>/`**, use the shared entrypoint, and add a case in **`lib/dockpipe/infrastructure/template.go`** (`TemplateBuild`) so `--isolate <name>` builds and uses it.
3. **Act scripts** — Any script that can run in the container and read `DOCKPIPE_EXIT_CODE` / `DOCKPIPE_CONTAINER_WORKDIR`. Bundled shared scripts live under **`templates/core/assets/scripts/`**; reference with **`scripts/…`** in YAML or `--act`. Users can copy a bundled script: `dockpipe action init my-commit.sh --from commit-worktree`.
4. **Scripts / workflows** — Bundled workflows use **`templates/<name>/config.yml`**; user workflows may use **`templates/<name>/config.yml`**. Optional **`steps:`** for multi-step and async groups — **[workflow-yaml.md](workflow-yaml.md)**. **`dockpipe init`** creates top-level **scripts/**, **images/**, **templates/** + **`templates/core/`**. **`dockpipe init my-template`** adds **templates/my-template/**. The **`worktree`** strategy is **`templates/core/strategies/worktree`**; use **`strategy: worktree`** in your YAML — **[workflow-yaml.md § Named strategies](workflow-yaml.md#named-strategies)**.

The core does not parse command content or assume any particular tool (Claude, git, etc.). It only runs the given argv and the optional action script.

---

## Permissions (UID/GID)

The runner passes `-u "$(id -u):$(id -g)"` to `docker run`, so the container process runs as your host user. Files created in the mounted workdir (e.g. `/work`) are owned by you and you can edit or delete them on the host. If you override the user (e.g. via a custom image or `--env`) or use a volume that was created as root, you can still hit permission issues; the default is “run as me.”

---

## Passing state between chained runs

Chained workflows (e.g. step 1 → step 2 → step 3) share state in two ways:

1. **Shared workdir** — Use the same `--workdir` for each run. Files written in `/work` (e.g. `plan.md`, build artifacts) are visible to the next run. This is the main “bridge.”
2. **stdout → stdin** — Pipe one run’s output into the next: `dockpipe -- ./gen.sh | dockpipe -- ./consume.sh`. Good for streaming or one-off payloads.

For structured data between steps, use a convention in the workdir (e.g. an action or command writes `.dockpipe/result.json` or `DOCKPIPE_RESULT_PATH`); the next run can read it. We don’t define a standard format; you own the schema. Env vars don’t survive between separate `dockpipe` invocations; use files or pipes.

---

## When the primitive isn’t enough

The primitive is deliberately minimal. You might need to script around it when:

- You want **orchestration** (retries, fan-out, dependencies) — use a Makefile, shell loop, or a real orchestrator and call dockpipe as one step.
- You need **rich state** between many steps — use the workdir (or a DB) and a convention; or a single long-running script inside one container that does the whole pipeline.
- You hit **image or tool limits** — use a custom image (`--isolate <image>` / `--build`) or run the heavy part outside dockpipe and use it for the parts that benefit from isolation.

Escape hatch: compose dockpipe with other tools rather than stretching the core.
