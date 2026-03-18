**dockpipe** is a general-purpose CLI: run any command in a disposable container, then optionally run an action on the result (e.g. commit, export patch). Same flow for tests, one-off scripts, codegen, or AI tools—not an AI framework; AI is one use case. In **0.5** a default data volume means you only mount your work directory; tool state and first-time login persist in the volume.

This post is a short intro to what it is, why it's useful, and how you can use it for automation and chained workflows (including AI).

---

## What is dockpipe?

dockpipe is a **single primitive** for running commands in disposable containers and optionally acting on the result. It's not a workflow engine, not an AI framework, and not tied to any vendor. It's a composable building block you plug into your own scripts and automation. **General-purpose first:** run tests, builds, or scripts; AI (e.g. Claude) is a strong use case but the core is command-agnostic.

**The model:**

1. **Spawn** — Start a container from an image (default is a light dev image; you can use your own).
2. **Run** — Execute whatever you pass in: a one-liner, a script, or a tool (e.g. Claude, Codex, `npm test`).
3. **Act** — Optionally run an action script after the command (e.g. commit all changes, export a patch, print a summary).

Your current directory is mounted at `/work` in the container, so the command sees your project. The action runs inside the same container right after the command, with access to exit code and work dir. By default a **named volume** (`dockpipe-data`) is mounted at `/dockpipe-data` and set as `HOME`—tool state (e.g. first-time login) and repos persist there. Use **`--data-vol <name>`**, **`--data-dir /path`**, or **`--no-data`** to change or skip. By default the run is **attached**: if you close the terminal, the container exits (no lingering processes). Use **`-d`** to run in the background and disconnect.

---

## Why use it?

**Isolation without commitment** — Run risky or messy commands (deps, globals, one-off experiments) in a container. When the run finishes, the container is gone. Your host stays clean.

**Same flow for everything** — One pattern for "run tests," "run a linter," "run Claude and commit," or "run my custom script." You choose the image, the command, and the action. No special cases in the tool.

**Pipe-friendly** — Stdin and stdout work normally. You can pipe a prompt in, pipe output to the next step, or chain multiple `dockpipe` invocations in a shell script.

**Composable** — The command can be a script. The action can be your own script. You can chain dockpipe with other tools (e.g. `dockpipe -- ./step1.sh | dockpipe -- ./step2.sh` or use it inside a Makefile or CI). It's designed to be composed into larger workflows.

**AI workflows without lock-in** — Great for "run an AI assistant in a container, then commit (or patch)." The core doesn't know or care that it's AI; you pass the AI tool as the command and use an action to persist the result. Same primitive works for non-AI automation.

---

## Chaining and automation

Because dockpipe is a single primitive, you can:

- **Chain steps** — Run one script in a container, pipe or pass its output to the next (e.g. plan → implement → review, each in its own clean run).
- **Automate AI workflows** — "Pipe prompt → run Claude (or another agent) in container → action commits or exports patch." The repo includes an example (clone, worktree, Claude, commit) you can copy or adapt.
- **CI-like local runs** — `dockpipe -- make test` or `dockpipe -- bash -c "npm ci && npm test"` in a clean environment without polluting your machine.
- **One-off experiments** — Try a new tool or version in a container; no global installs, no cleanup.

You stay in control: you pick the image, the command, and the action. The tool doesn't impose workflow structure—it just runs and (optionally) acts.

---

## Try it (15 seconds)

Install the [latest .deb](https://github.com/jamie-steele/dockpipe/releases) or clone the repo and add `bin` to your PATH. Then:

```bash
dockpipe -- make test
```

Runs `make test` in a clean container; your dir is at `/work`, container is removed when done. Same for `npm test`, `cargo test`, or any command. Sanity check: `dockpipe -- echo "hello from container"`. Requirements: Bash and Docker.

---

## Examples

**Generic command (default image):**

```bash
dockpipe -- ls -la
dockpipe -- bash -c "npm test"
```

**Scaffold an action, then use it:**

```bash
dockpipe action init my-action.sh
# Edit my-action.sh (it has DOCKPIPE_EXIT_CODE and DOCKPIPE_CONTAINER_WORKDIR), then:
dockpipe --action my-action.sh -- ./my-script.sh
```

**Pipe in a prompt, run Claude, commit the result:**

```bash
echo "refactor the auth module" \
  | dockpipe --template agent-dev --action examples/actions/commit-worktree.sh \
  -- claude --dangerously-skip-permissions -p "$(cat)"
```

(Use `--template agent-dev` for the Node + Claude image; `--template claude` is an alias.)

**Run Claude in the background (detach):**

```bash
dockpipe -d --template agent-dev -- claude -p "review this code"
# Prints container ID; use docker logs <id> or docker attach <id>
```

**Custom image and workdir:**

```bash
dockpipe --image my-dev --workdir /path/to/repo -- bash -c "npm ci && npm test"
```

**Run your script, then run an action (e.g. commit or export patch):**

```bash
dockpipe --action examples/actions/commit-worktree.sh -- ./my-script.sh
```

---

## When it fits

- You want **isolation** for one-off or repeated commands without maintaining a full orchestration stack.
- You're building **automation** (scripts, cron, local pipelines) and want a simple "run in container, then do X" step.
- You're **chaining AI tools** (or any CLI) and want a clean boundary: run in container, then commit/patch/summarize.
- You like **Unix-style composition**: small tools, pipes, and scripts rather than a big framework.

---

## Summary

dockpipe gives you one primitive: **spawn → run → act**. Use it for isolated runs, automation, chained steps, or AI workflows. Same isolation as `docker run`, with less boilerplate: consistent workdir + UID/GID, optional action phase, templates, and pipe-friendly CLI. It stays minimal and composable so you can plug it into your own scripts and tooling without adopting a framework.

If you try it and have ideas or feedback, the repo is [github.com/jamie-steele/dockpipe](https://github.com/jamie-steele/dockpipe).
