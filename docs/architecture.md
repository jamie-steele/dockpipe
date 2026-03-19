# dockpipe architecture

This document describes the core primitive, data flow, and extension points.

---

## Primitive

dockpipe implements a single flow:

1. **Run** (host) — Optional scripts on the host before the container (`--run`). E.g. clone-worktree.
2. **Isolate** — Start a container from a given image (default or from `--isolate`: template name or image).
3. **Command** — Execute the user's command inside the container. Workdir is mounted at `/work`.
4. **Act** — If `--act <script>` was given, run that script inside the container after the command (e.g. commit-worktree). Container exits with the command's exit code.

No built-in commit, clone, or AI logic — those are actions or scripts you plug in.

**Lifecycle:** By default the container runs **attached** (stdin/stdout connected). Closing the terminal or disconnecting the client sends SIGTERM; the entrypoint stops the command and the container exits (`--rm` removes it). Use **`-d` / `--detach`** to run in the background (no attach); the container stays up until the command inside exits.

---

## Components

| Component | Role |
|-----------|------|
| `bin/dockpipe` | Launcher: runs **`bin/dockpipe.bin`** if present (`make`), otherwise **`go run ./cmd/dockpipe`**. |
| `cmd/dockpipe`, `lib/dockpipe/application`, `lib/dockpipe/domain`, `lib/dockpipe/infrastructure` | **Go** CLI (DDD-ish): application layer (flags + orchestration + `windows setup/doctor`), domain (workflow/env/resolver semantics), infrastructure (FS, docker, bash, git). **`config.yml`** / **`steps:`** (YAML v3), resolver `KEY=value` files, template→image map, bash `source` for pre-scripts, **`docker run`** / build, host **git** commit. |
| `lib/entrypoint.sh` | Container entrypoint: run command, then `DOCKPIPE_ACTION` if set. |
| `images/*/Dockerfile` | Shared images; each copies `lib/entrypoint.sh` as `ENTRYPOINT`. |
| `scripts/*.sh` | Run/act scripts; invoked via `--run` / `--act` or workflow (Go sources them with bash). |
| `templates/*/` | Workflow templates (`config.yml`, `resolvers/`). Multi-step / async: see **[workflow-yaml.md](workflow-yaml.md)**. |
| `scripts/dockpipe-legacy.sh`, `lib/*.sh` | **Legacy** all-bash implementation (optional); default install uses Go. |

---

## Data flow (default: Go CLI)

```
User: dockpipe --isolate claude --act scripts/commit-worktree.sh -- claude -p "..."

  bin/dockpipe → dockpipe (Go binary when packaged / built)
    → TemplateBuild("claude") from --isolate → image=dockpipe-claude, build=.../images/claude
    → docker build (if needed) / docker run
    → see lib/dockpipe/infrastructure/docker.go

  Container (lib/entrypoint.sh)
    → cd /work
    → exec user argv (e.g. claude -p "...")
    → save exit code
    → run DOCKPIPE_ACTION if set (e.g. commit-worktree)
    → exit with saved exit code
```

**`--workflow` with `steps:`** — `lib/dockpipe/application/run_steps.go` runs each step (optional parallel async groups, merge `outputs:` in order). Spec: **[workflow-yaml.md](workflow-yaml.md)**.

**Legacy:** `scripts/dockpipe-legacy.sh` + `lib/runner.sh` mirror a similar `docker run` sequence; not the default install path.

Environment variables that cross the boundary:

- **Host → container (CLI / runner):** `DOCKPIPE_CONTAINER_WORKDIR`, `DOCKPIPE_ACTION` (path inside container), plus any `--env` and image defaults.
- **Entrypoint → action:** `DOCKPIPE_EXIT_CODE`, `DOCKPIPE_CONTAINER_WORKDIR`.

---

## Extension points

1. **Images** — Add a Dockerfile under `images/<name>/`, use the shared entrypoint, and add a case in **`lib/dockpipe/infrastructure/template.go`** (`TemplateBuild`) so `--isolate <name>` builds and uses it (legacy bash: `resolve_template()` in `scripts/dockpipe-legacy.sh`).
2. **Act scripts** — Any script that can run in the container and read `DOCKPIPE_EXIT_CODE` / `DOCKPIPE_CONTAINER_WORKDIR`. Place under `scripts/` and reference with `--act` or workflow config `act:`. Users can copy a bundled script: `dockpipe action init my-commit.sh --from commit-worktree`.
3. **Scripts / workflows** — Named workflows use `templates/<name>/config.yml` + `resolvers/`. Optional **`steps:`** for multi-step and async groups — **[workflow-yaml.md](workflow-yaml.md)**. **`dockpipe init`** creates top-level **scripts/**, **images/**, **templates/**. **`dockpipe init my-template`** adds **templates/my-template/** and copies sample scripts/images. **llm-worktree** lives under `templates/llm-worktree/`; copy with `dockpipe template init my-workflow --from llm-worktree`.

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
