# dockpipe architecture

This document describes the core primitive, data flow, and extension points.

---

## Primitive

dockpipe implements a single flow:

1. **Spawn** — Start a container from a given image (default or from `--image` / `--template`).
2. **Run** — Execute the user’s command inside the container. The host directory (or `--workdir`) is mounted at `/work`; the command runs with that as the working directory.
3. **Act** — If `--action <script>` was given, run that script inside the container after the command, with access to the command’s exit code and work dir. The container then exits with the original command’s exit code.

No built-in commit, clone, or AI logic — those are actions or scripts you plug in.

---

## Components

| Component | Role |
|-----------|------|
| `bin/dockpipe` | CLI: parse args, resolve template → image (and build path), set env, source runner, call `dockpipe_run "$@"`. |
| `lib/runner.sh` | Build `docker run` args (mounts, env, action script mount), then `exec docker run ... image "$@"`. |
| `lib/entrypoint.sh` | Container entrypoint: run command (argv or `DOCKPIPE_CMD`), then run `DOCKPIPE_ACTION` if set, exit with command’s exit code. |
| `images/*/Dockerfile` | Define images; each copies `lib/entrypoint.sh` and uses it as `ENTRYPOINT`. |
| `examples/actions/*.sh` | Example action scripts; usable with `--action`. |
| `examples/*/` | Example workflows (e.g. Claude worktree); scripts + README. |

---

## Data flow

```
User: dockpipe --template claude --action examples/actions/commit-worktree.sh -- claude -p "..."

  bin/dockpipe
    → resolve_template("claude") → image=dockpipe-claude, build=.../images/claude
    → build image if needed
    → set DOCKPIPE_IMAGE, DOCKPIPE_ACTION, DOCKPIPE_WORKDIR, ...
    → source lib/runner.sh
    → dockpipe_run claude -p "..."

  lib/runner.sh
    → docker run --rm -v $PWD:/work -v <action>:/dockpipe-action.sh -e DOCKPIPE_ACTION=... dockpipe-claude claude -p "..."

  Container (lib/entrypoint.sh)
    → cd /work
    → exec "claude" "-p" "..."
    → save exit code
    → run /dockpipe-action.sh (commit-worktree)
    → exit with saved exit code
```

Environment variables that cross the boundary:

- **Host → container (CLI / runner):** `DOCKPIPE_CONTAINER_WORKDIR`, `DOCKPIPE_ACTION` (path inside container), plus any `--env` and image defaults.
- **Entrypoint → action:** `DOCKPIPE_EXIT_CODE`, `DOCKPIPE_CONTAINER_WORKDIR`.

---

## Extension points

1. **Images** — Add a Dockerfile under `images/<name>/`, use the shared entrypoint, and (optionally) register a template in `bin/dockpipe` so `--template <name>` builds and uses it.
2. **Actions** — Any script that can run in the container and read `DOCKPIPE_EXIT_CODE` / `DOCKPIPE_CONTAINER_WORKDIR`. Place under `examples/actions/` and reference with `--action`.
3. **Scripts / workflows** — Arbitrary scripts (e.g. clone + worktree + run tool + commit) live in `examples/` and are invoked as the “command” (e.g. `dockpipe ... -- ./examples/claude-worktree/setup-and-claude.sh`). No change to core required.

The core does not parse command content or assume any particular tool (Claude, git, etc.). It only runs the given argv and the optional action script.
