**dockpipe** is a general-purpose CLI: run any command in a disposable container, then optionally run an action on the result (e.g. commit, export patch). Same flow for tests, one-off scripts, codegen, or AI tools—not an AI framework; AI is one use case. **Agnostic by design:** AI support is via **resolvers** (one file per tool—Claude, Codex, or your own) and **templates**; the core never hardcodes a vendor.

The **default implementation is a Go CLI** (`src/cmd/dockpipe`): it orchestrates Docker, parses **`config.yml`** natively (including optional multi-step **`steps:`**, parallel **async** groups, and **`outputs:`** handoff—see **[workflow-yaml.md](../workflow-yaml.md)**), and runs host **bash** for pre-scripts. Legacy bash-only helpers remain under `scripts/` for reference.

In **0.6** you get **worktree on host**, **commit on host** (so the AI never has container git access), **template init** (copy workflows and customize without contributing back), and **Windows** support (**`dockpipe.exe`** natively with Docker Desktop; optional **WSL bridge** for Linux `dockpipe` inside a distro).

This post is a short intro to what it is, why it's useful, and how you can use it for automation and chained workflows (including AI).

---

## What is dockpipe?

dockpipe is a **single primitive** for running commands in disposable containers and optionally acting on the result. It's not a workflow engine, not an AI framework, and not tied to any vendor. It's a composable building block you plug into your own scripts and automation. **General-purpose first:** run tests, builds, or scripts; AI (e.g. Claude, Codex) is a strong use case but the core is command-agnostic.

**The model:**

1. **Spawn** — Start a container from an image (default is a light dev image; you can use your own).
2. **Run** — Execute whatever you pass in: a one-liner, a script, or a tool (e.g. Claude, Codex, `npm test`).
3. **Act** — Optionally run an action after the command (e.g. commit all changes, export a patch). When you use the bundled commit flow, the **commit runs on the host** so the AI never touches git.

Your current directory (or a **worktree** dockpipe creates on the host) is mounted at `/work`. By default a **named volume** (`dockpipe-data`) is mounted at `/dockpipe-data` and set as `HOME`—tool state persists there. Use **`--data-dir /path`** or **`--no-data`** to change.

**`--repo` + worktree flows:** dockpipe creates or reuses the clone and worktree on the **host** (often under `~/.dockpipe` when using **`--data-dir`** defaults), mounts it as `/work`, runs your command, then can commit on the host. **`--branch`** is optional—omit it and dockpipe picks a new branch name each run (or set **`--work-branch`** / **`--branch`** explicitly). One primitive for Claude, Codex, or any resolver you add.

---

## Agnostic AI: resolvers and worktree on host

AI support is **provider-agnostic**. **Resolvers** are profiles under **`templates/core/resolvers/<name>`** (or **`profile`** inside that directory). Each sets template (image), default command, and env hint. Adding a new AI tool = add a new resolver profile; no changes to core. Use **`--resolver claude`** or **`--resolver codex`**; same flags, same flow.

**Worktree on host:** With **`--repo <url>`**, dockpipe prepares the clone/worktree on the **host**. The container only sees the worktree at `/work`. When the run finishes, dockpipe can run **git on the host** for commit-on-host flows—so the model in the container never needs your credentials. **Authentication** is your normal **git** setup on the OS (HTTPS/SSH, Credential Manager, etc.); dockpipe does not replace that.

**Template init:** Run **`dockpipe template init my-ai [--from init]`** to copy a starter workflow. Add **`strategy: worktree`** in **`config.yml`** when you need clone/resolver flows — see **[workflow-yaml.md](../workflow-yaml.md#named-strategies)**. Then run `dockpipe --workflow my-ai --repo <url> [--resolver claude|codex] -- claude -p "..."`.

**Multi-step workflows:** Optional **`steps:`** in **`config.yml`** (blocking vs parallel **async** steps, **`outputs:`** between steps) — see **[workflow-yaml.md](../workflow-yaml.md)** and **[cli-reference.md](../cli-reference.md)**.

---

## Why use it?

**Isolation without commitment** — Run risky or messy commands (deps, globals, one-off experiments) in a container. When the run finishes, the container is gone. Your host stays clean.

**Same flow for everything** — One pattern for "run tests," "run a linter," "run Claude and commit," or "run Codex and commit." You choose the image (or resolver), the command, and the action. No special cases in the tool.

**AI without lock-in** — Resolvers and templates keep the core agnostic. Commit on host means the AI never has git access. Add your own resolver or template without contributing back.

**Pipe-friendly** — Stdin and stdout work normally. You can pipe a prompt in, pipe output to the next step, or chain multiple `dockpipe` invocations in a shell script.

**Composable** — The command can be a script. The action can be your own script. You can chain dockpipe with other tools or use it inside a Makefile or CI.

---

## Chaining and automation

Because dockpipe is a single primitive, you can:

- **Chain steps** — Run one script in a container, pipe or pass its output to the next (e.g. plan → implement → review, each in its own clean run). See **[chaining.md](../chaining.md)**.
- **Automate AI workflows** — "Run Claude (or Codex) in a worktree → commit on host." Use **`--resolver claude --repo URL`** (add **`--branch`** or **`--work-branch`** as needed) and **`strategy: worktree`** in workflow YAML.
- **CI-like local runs** — `dockpipe -- make test` or `dockpipe -- bash -c "npm ci && npm test"` in a clean environment.
- **One-off experiments** — Try a new tool or version in a container; no global installs, no cleanup.

You stay in control: you pick the image (or resolver), the command, and the action. The tool doesn't impose workflow structure—it just runs and (optionally) acts.

---

## Try it (15 seconds)

**Install:** pick your platform from **[GitHub Releases](https://github.com/jamie-steele/dockpipe/releases)** and follow **[install.md](../install.md)** (Linux `.deb`, tarballs, macOS, **Windows** `dockpipe.exe` + Docker Desktop + **Git for Windows** for bash/git on PATH, etc.).

**First run:**

```bash
dockpipe -- make test
```

Runs `make test` in a clean container; your dir is at `/work`, container is removed when done. Same for `npm test`, `cargo test`, or any command. Sanity check: `dockpipe -- echo "hello from container"`.

**Requirements:** **Docker** and **`bash` on the host** (dockpipe always invokes bash for host tooling). **`git` on the host** only for **`--repo`**, worktrees, and commit-on-host.

---

## Examples

**Generic command (default image):**

```bash
dockpipe -- ls -la
dockpipe -- bash -c "npm test"
```

**AI in worktree, commit on host (agnostic: same for Claude or Codex):**

```bash
dockpipe --resolver claude --repo https://github.com/you/repo.git --branch claude/task \
  -- claude --dangerously-skip-permissions -p "Fix the bug"
# For Codex: --resolver codex and use codex / codex exec "..." as the command.
# Commit runs on host; your host git auth applies—dockpipe does not replace Git Credential Manager / SSH.
```

**Copy a workflow template and customize (no contribution required):**

```bash
dockpipe template init my-ai --from init
# Run with your workflow (config points to scripts/):
dockpipe --workflow my-ai --repo <url> --resolver claude -- claude -p "Your prompt"
```

**Scaffold an action, or clone a bundled one:**

```bash
dockpipe action init my-action.sh
dockpipe action init my-commit.sh --from commit-worktree
# Edit, then: dockpipe --act my-commit.sh -- ./my-script.sh
```

**Pipe in a prompt, run Claude, commit on host (current dir):**

```bash
echo "refactor the auth module" \
  | dockpipe --isolate agent-dev --act scripts/commit-worktree.sh \
  -- claude --dangerously-skip-permissions -p "$(cat)"
```

**Run Claude or Codex in the background (detach):**

```bash
dockpipe -d --resolver claude --repo https://github.com/you/repo.git --branch claude/task -- claude -p "review this code"
```

**Resume a previous Claude session** (state lives in the data volume):

```bash
dockpipe --template agent-dev -- claude --resume <session-id> --dangerously-skip-permissions
```

**Custom image and workdir:**

```bash
dockpipe --isolate my-dev --workdir /path/to/repo -- bash -c "npm ci && npm test"
```

---

## When it fits

- You want **isolation** for one-off or repeated commands without maintaining a full orchestration stack.
- You're building **automation** (scripts, cron, local pipelines) and want a simple "run in container, then do X" step.
- You're **chaining AI tools** (or any CLI) and want a clean boundary: run in container, commit on host, no git in the container.
- You like **Unix-style composition**: small tools, pipes, and scripts rather than a big framework.
- You want **agnostic AI support**: add resolvers and templates without vendor lock-in.

---

## Summary

dockpipe gives you one primitive: **run → isolate → act** (host prep, container, host follow-up). Use it for isolated runs, automation, chained steps, or AI workflows. **0.6** ships the **Go CLI**, **resolvers** (agnostic AI), **worktree and commit on host**, **template init**, **native Windows** (`dockpipe.exe` + Docker Desktop), optional **multi-step `config.yml`**, and the same isolation story as `docker run` with less boilerplate: consistent workdir, optional user mapping on Unix, optional action phase, templates, resolvers, and a pipe-friendly CLI. It stays minimal and composable so you can plug it into your own scripts and tooling without adopting a framework.

If you try it and have ideas or feedback, the repo is [github.com/jamie-steele/dockpipe](https://github.com/jamie-steele/dockpipe). See [CONTRIBUTING.md](https://github.com/jamie-steele/dockpipe/blob/master/CONTRIBUTING.md) to add a resolver or template.
