# dockpipe

**Run any command in a disposable container. Optionally run an action on the result (e.g. commit, export patch).**

General-purpose: run tests, one-off scripts, codegen, or AI tools—same flow. Your dir is mounted, your user owns the files, container is gone when done. Not an AI framework; AI is one use case.

---

## Try it (15 seconds)

**Install:** [Download the .deb](https://github.com/jamie-steele/dockpipe/releases) → `sudo dpkg -i dockpipe_*_all.deb`  
Or from source: `git clone https://github.com/jamie-steele/dockpipe.git` and add `bin` to your PATH.

**First run:**

```bash
dockpipe -- make test
```

Runs `make test` in a clean container. Your current directory is at `/work`. When it exits, the container is removed. Same for `npm test`, `cargo test`, or any command.

---

## What you can do

| Use case | Command |
|----------|--------|
| **Run tests in isolation** | `dockpipe -- make test` |
| **Run a script and commit the result** | `dockpipe --action examples/actions/commit-worktree.sh -- ./scripts/generate-docs.sh` |
| **Pipe stdin** | `echo "input" \| dockpipe -- cmd` |
| **AI + commit** | `dockpipe --template agent-dev --action examples/actions/commit-worktree.sh -- claude -p "Your prompt"` |

Scaffold your own action: `dockpipe action init my-action.sh` → then `--action my-action.sh`.

---

## How it works

1. **Spawn** — Start a container (default: small dev image, built if needed).
2. **Run** — Execute the command you pass after `--`.
3. **Act** — Optionally run a script after the command (e.g. commit, notify).

You pick the image, the command, and the action. No workflow engine—just one primitive you compose.

---

## Why not just `docker run`?

Same isolation, less boilerplate. You’d otherwise write:

```bash
docker run --rm -v "$(pwd):/work" -w /work -u "$(id -u):$(id -g)" some-image make test
```

dockpipe does that by default and adds: **action phase** (run then act in one go), **templates** (`--template dev` / `agent-dev`), **pipe-friendly CLI**. Files created in the container are owned by you (UID/GID passed through).

---

## Install (details)

| Platform | How |
|----------|-----|
| **Linux** | [Releases](https://github.com/jamie-steele/dockpipe/releases) → `sudo dpkg -i dockpipe_*_all.deb` |
| **macOS** | Clone repo, add `bin` to PATH. Requires Bash + Docker. |
| **Windows** | Not supported; [WSL](https://docs.microsoft.com/en-us/windows/wsl/) + source may work. |

Requirements: **Bash**, **Docker**. [More in docs/install.md](docs/install.md).

---

## Usage

```text
dockpipe [options] -- <command> [args...]
dockpipe action init <filename>
```

| Option | Description |
|--------|-------------|
| `--image <name>` | Docker image (default: dockpipe-base-dev). |
| `--template <name>` | Preset: `base-dev`, `dev`, `agent-dev` (or `claude`). |
| `--action <script>` | Script run inside container after the command. |
| `--workdir <path>` | Host path mounted at `/work` (default: current dir). |
| `--mount`, `--env` | Extra volumes or env vars. |
| `--help` | Help. |

---

## Examples

**Generic:**

```bash
dockpipe -- ls -la
dockpipe -- bash -c "npm test"
dockpipe --template dev -- make test
```

**Run script then commit:**

```bash
dockpipe --action examples/actions/commit-worktree.sh -- ./my-script.sh
```

**AI (agent-dev template) + commit:**

```bash
cd /path/to/repo
dockpipe --template agent-dev --action examples/actions/commit-worktree.sh \
  --env "DOCKPIPE_COMMIT_MESSAGE=agent: my task" \
  -- claude --dangerously-skip-permissions -p "Your prompt"
```

**Chained (each step in a fresh container):**

```bash
dockpipe -- make lint && dockpipe -- make test && dockpipe -- make build
```

**Full runnable examples:** [chained non-AI](examples/chained-non-ai/README.md) · [chained multi-AI](examples/chained-multi-ai/README.md) · [Claude worktree](examples/claude-worktree/README.md)

---

## Templates

| Template | Description |
|----------|-------------|
| `base-dev` | Light: git, curl, bash, ripgrep, jq. |
| `dev` | base-dev + build-essential, ssh, etc. |
| `agent-dev` | Node + Claude Code (AI/agent workflows). `claude` is an alias. |

---

## Actions

Scripts that run **inside** the container after your command. They get `DOCKPIPE_EXIT_CODE` and `DOCKPIPE_CONTAINER_WORKDIR`. Create one: `dockpipe action init my-action.sh`. Bundled: [commit-worktree](examples/actions/commit-worktree.sh), [export-patch](examples/actions/export-patch.sh), [print-summary](examples/actions/print-summary.sh).

---

## Docs & repo

- [Blog: Run, Isolate, and Act](https://dev.to/jamie-steele/run-isolate-and-act-a-minimal-primitive-for-container-workflows-553m)
- [Architecture](docs/architecture.md) · [Install](docs/install.md) · [AGENTS.md](AGENTS.md)
- **Tests:** `bash tests/run_tests.sh` (from repo root)

**License:** Apache-2.0. See [LICENSE](LICENSE).
