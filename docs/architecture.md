# DockPipe engine (data flow)

**Terms:** **[architecture-model.md](architecture-model.md)**. This page is **how** the Go CLI wires run → isolate → act.

## Primitive

1. **Run** (host) — optional **`--run`** pre-scripts.  
2. **Isolate** — container from image / template; project at **`/work`**; argv after **`--`** runs inside.  
3. **Act** — optional post-step; bundled **`commit-worktree`** runs **on the host** after exit.

**Detach:** **`-d`** runs the container without attaching (stays up until the inner command exits).

## Components (short)

| Piece | Role |
|-------|------|
| **`src/bin/dockpipe`** | Launcher → **`dockpipe.bin`** or **`go run ./src/cmd`**. |
| **`src/cmd`**, **`src/lib/…`** | CLI: flags, **`config.yml`** / **`steps:`**, Docker, bash pre/act. |
| **`embed.go`** | Bundled **`templates/`**, entrypoint, **`VERSION`**. |
| **`assets/entrypoint.sh`** | Container: run command, then **`DOCKPIPE_ACTION`** if set. |

**Paths:** resolver / bundle **`scripts/…`** resolution — **`src/lib/infrastructure/paths.go`**.

## Default data flow

```
dockpipe --isolate claude --act scripts/commit-worktree.sh -- claude -p "…"
  → TemplateBuild / docker run
  → entrypoint: exec argv; optional DOCKPIPE_ACTION
  → host commit path when --act is bundled commit-worktree
```

**`steps:`** — **`run_steps.go`**; spec **[workflow-yaml.md](workflow-yaml.md)**.

## Extension points

1. **Profiles** — **`templates/core/resolvers/<name>`** (or runtimes) — **[isolation-layer.md](isolation-layer.md)**.  
2. **Images** — Dockerfile under resolver/bundle **`assets/images/`**; **`TemplateBuild`** in **`template.go`**.  
3. **Workflows** — **`templates/<name>/config.yml`**; **`dockpipe init`**, **`--from`**.  

## Permissions

**`docker run -u $(id -u):$(id -g)`** — files in **`/work`** match host user unless the image overrides.
