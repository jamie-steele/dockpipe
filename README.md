# dockpipe

```
    ██████╗  ██████╗ ██████╗██╗  ██╗██████╗ ██╗██████╗ ███████╗
    ██╔══██╗██╔═══██╗██╔═══╝██║ ██╔╝██╔══██╗██║██╔══██╗██╔════╝
    ██║  ██║██║   ██║██║    █████╔╝ ██████╔╝██║██████╔╝█████╗
    ██║  ██║██║   ██║██║    ██╔═██╗ ██╔═══╝ ██║██╔═══╝ ██╔══╝
    ██████╔╝╚██████╔╝██████╗██║  ██╗██║     ██║██║     ███████╗
    ╚═════╝  ╚═════╝ ╚═════╝╚═╝  ╚═╝╚═╝     ╚═╝╚═╝     ╚══════╝
                      Run  →  Isolate  →  Act
```

**Run any command in a disposable container. Optionally run an action on the result.**

One primitive: run → isolate → act. Use it for isolated tests, one-off scripts, builds, or piping—no workflow engine. AI (Claude, Codex) and git/worktree flows are optional examples built on the same primitive; the core is command-agnostic.

---

## Try it (15 seconds)

**Linux:** [Download the .deb](https://github.com/jamie-steele/dockpipe/releases) for your arch (`*_amd64.deb` or `*_arm64.deb`) → `sudo dpkg -i` that file.  
**Windows:** `irm https://raw.githubusercontent.com/jamie-steele/dockpipe/main/packaging/windows/install.ps1 | iex` (MSI + checksum), or grab **`.msi` / `.zip`** from [Releases](https://github.com/jamie-steele/dockpipe/releases) — then `dockpipe windows setup`. See [docs/install.md](docs/install.md).  
Or from source: `git clone … && cd dockpipe && make && export PATH="$PWD/bin:$PATH"` (needs **Go 1.22+** to build; or run without `make` if Go is installed — `bin/dockpipe` uses `go run` as a fallback).

**First run:**

```bash
dockpipe -- make test
```

Runs `make test` in a clean container. Your current directory is at `/work`. When it exits, the container is removed. Closing the terminal also tears down the container (attached run). Use `-d` to run in the background. Same for `npm test`, `cargo test`, or any command.

---

## What you can do

Same primitive for everything. Examples:

| Use case | Command |
|----------|--------|
| **Run tests in isolation** | `dockpipe -- make test` |
| **Run a script** | `dockpipe -- ./scripts/generate-docs.sh` |
| **Pipe stdin** | `echo "input" \| dockpipe -- cmd` |
| **Run then act** (e.g. commit) | `dockpipe --act scripts/commit-worktree.sh -- ./my-script.sh` |
| **AI in worktree** (optional) | `dockpipe --workflow llm-worktree --repo https://github.com/you/repo.git -- claude -p "Fix the bug"` |

You pick the image, the command, and the action. **Init a workspace:** `dockpipe init [<dest>]` creates **scripts/**, **images/**, **templates/** (templates folder has a README only). `dockpipe init my-template [<dest>]` adds a new template at **templates/my-template/** and copies sample scripts/images so it can point at them. Use `--from <url>` to clone a repo with that layout. Scaffold: `dockpipe action init my-action.sh`, `dockpipe template init my-workflow --from llm-worktree`. **YAML demo:** `templates/workflow-demo/` (async group + outputs + join) — `dockpipe --workflow workflow-demo` from repo root; see **[templates/workflow-demo/README.md](templates/workflow-demo/README.md)**.

---

## How it works

1. **Spawn** — Start a container (default: small dev image, built if needed).
2. **Run** — Execute the command you pass after `--`.
3. **Act** — Optionally run a script after the command (e.g. commit, notify, export a patch).

No separate orchestrator service—just one primitive you compose. Optional **multi-step** flows live in **`config.yml`** (`steps:`), including parallel **async** groups and **`outputs:`** handoff; see **[docs/workflow-yaml.md](docs/workflow-yaml.md)**. Tests, scripts, builds, or AI tools all use the same flow.

**Data:** Default volume `dockpipe-data` at `/dockpipe-data` (HOME). `--data-dir /path` or `--no-data`. `--reinit` to wipe the volume.

---

## Why not just `docker run`?

Same isolation, less boilerplate. You’d otherwise write:

```bash
docker run --rm -v "$(pwd):/work" -w /work -u "$(id -u):$(id -g)" some-image make test
```

dockpipe does that + action phase, templates, pipe-friendly CLI. Your UID/GID so files are yours.

---

## Platforms

| Platform | Notes |
|----------|-------|
| **Linux** | Primary platform. **CLI is a Go binary** (orchestration + YAML); pre/act scripts stay Bash. Install via [.deb](https://github.com/jamie-steele/dockpipe/releases) (**amd64** or **arm64**) or `make` from source. **Docker** + **Bash** required; **git** for commit-on-host. |
| **Windows (WSL2)** | Install via **MSI**, **`install.ps1`** (see [install.md](docs/install.md)), or zip. `dockpipe.exe` forwards runs into WSL (**cwd** + path-like flags). Run `dockpipe windows setup` once. [docs/wsl-windows.md](docs/wsl-windows.md). |
| **macOS** | Homebrew-friendly formula is included (`packaging/homebrew/dockpipe.rb`). Preferred install: `brew tap jamie-steele/dockpipe && brew install dockpipe`. Source fallback still supported (`make`). |

---

## Install (details)

| Platform | How |
|----------|-----|
| **Linux** | [Releases](https://github.com/jamie-steele/dockpipe/releases) → `sudo dpkg -i dockpipe_*_amd64.deb` or `*_arm64.deb` (match your CPU) |
| **macOS** | `brew tap jamie-steele/dockpipe && brew install dockpipe` (or source fallback). |
| **Windows** | Run `dockpipe windows setup` (PowerShell/CMD) to configure WSL distro + bootstrap; optionally pass `--distro` and `--install-command` for automation. |

Requirements: **Bash**, **Docker**. [More in docs/install.md](docs/install.md).

---

## Usage

```text
dockpipe [options] -- <command> [args...]
dockpipe action init [--from <bundled>] <filename>
dockpipe template init [--from <bundled>] <dirname>
dockpipe windows setup [--distro <name>] [--install-command <cmd>] [--non-interactive]
dockpipe windows doctor
```

| Option | Description |
|--------|-------------|
| `--workflow <name>` | Load `templates/<name>/config.yml` (run, isolate, act, optional **vars:**). |
| `--run <path>` | **Run:** script(s) on host before container. Can be repeated. |
| `--isolate <name>` | **Isolate:** image or template (`base-dev`, `dev`, `agent-dev`, `claude`, `codex`). Builds if template. |
| `--act <path>` | **Act:** script after command (e.g. commit-worktree). |
| `--resolver <name>` | `claude` or `codex`; use with `--repo`/`--branch`. Add more in the template’s `resolvers/` (e.g. `templates/llm-worktree/resolvers/`). |
| `--repo <url>` | With `--branch`: worktree on host, commit on host. |
| `--branch <name>` | Work branch for `--repo` (optional). Omit for a new branch each run; set to use or resume a specific branch. |
| `--work-path <path>` | Subfolder inside repo to open in container (full repo at `/work`; command cwd = `/work/<path>`). |
| `--work-branch <name>` | When on main/master and committing on host, create/use this branch (default: `dockpipe/agent-<timestamp>`). |
| `--bundle-out <path>` | After commit-on-host, write a git bundle at this path (for WSL→Windows fetch). |
| `--workdir <path>` | Host path mounted at `/work` (default: current dir). |
| `--data-vol <name>` | Named volume for persistent data (default: `dockpipe-data`). Same volume each run = state persists. |
| `--data-dir <path>` | Bind mount host path for persistent data (e.g. `$HOME/.dockpipe`). For `--repo`/`--branch`, defaults to `~/.dockpipe` if not set. |
| `--no-data` | Do not mount the data volume (minimal run). |
| `--reinit` | Remove the named data volume before running (fresh volume). Prompts to confirm; use `-f` to skip. |
| `-f`, `--force` | With `--reinit`, skip confirmation (warning still shown). |
| `--mount`, `--env` | Extra volumes or env vars (into container). |
| `--env-file <path>` | With `--workflow`: merge `KEY=VAL` from file if unset. Repeatable. |
| `--var KEY=VAL` | With `--workflow`: set workflow var (overrides yml / `.env`). Repeatable. |
| `-d`, `--detach` | Run container in background; don't attach. Container stays up until command exits. |
| `--help` | Help. |

---

## Examples

**Single run (isolation, tests, scripts):**

```bash
dockpipe -- ls -la
dockpipe -- bash -c "npm test"
dockpipe --isolate dev -- make test
```

**Run a script, then run an action** (e.g. commit):

```bash
dockpipe --act scripts/commit-worktree.sh -- ./my-script.sh
```

**Chaining** — Multiple steps, same workdir; each step in a fresh container. See [docs/chaining.md](docs/chaining.md):

```bash
WORKDIR="/path/to/your/project"
dockpipe --workdir "$WORKDIR" -- make lint && dockpipe --workdir "$WORKDIR" -- make test && dockpipe --workdir "$WORKDIR" -- make build
```

Same pattern with AI: plan → implement → review (same folder). One doc: [Chaining](docs/chaining.md).

**AI in a worktree** (optional; same primitive):

```bash
dockpipe --resolver claude --repo https://github.com/you/repo.git --branch task -- claude -p "Fix the bug"
```

**AI in current dir + commit on host:**

```bash
cd /path/to/repo
dockpipe --isolate agent-dev --act scripts/commit-worktree.sh -- claude -p "Your prompt"
```

**Detach** (container keeps running after you close the terminal):

```bash
dockpipe -d -- make test
```

**Docs:** [docs/workflow-yaml.md](docs/workflow-yaml.md) (`steps:`, async, `outputs:`), [docs/cli-reference.md](docs/cli-reference.md), [docs/chaining.md](docs/chaining.md), [docs/wsl-windows.md](docs/wsl-windows.md). **Workflow template:** [templates/llm-worktree](templates/llm-worktree/README.md) — use `--workflow llm-worktree --repo <url> -- claude -p "…"`.

---

## Resolvers and worktree (optional)

The same primitive can drive AI or git workflows. **Resolvers:** one file per tool in the template’s `resolvers/` (e.g. `templates/llm-worktree/resolvers/claude`, `codex`) → template + cmd; add a file, no core change. **`--resolver` + `--repo` + `--branch`:** worktree on host, run in container, commit on host. **Template init:** `dockpipe template init my-workflow [--from llm-worktree]` copies the workflow (config, resolvers, isolate). Run `dockpipe --workflow my-workflow --repo <url> -- ...`; config points to scripts/. Use `--resolver claude` or `codex`.

---

## Templates

| Template | Description |
|----------|-------------|
| `base-dev` | Light: git, curl, bash, ripgrep, jq. |
| `dev` | base-dev + build-essential, ssh, etc. |
| `agent-dev` | Node + Claude Code. `claude` is an alias. One of several templates. |
| `codex` | Node + OpenAI Codex CLI. |

---

## Actions

Scripts that run after your command. Use them for anything: commit, export a patch, notify. Bundled **commit-worktree** runs on the host when you use it (current dir or `--repo`/`--branch`). `dockpipe action init my-action.sh --from commit-worktree` (or export-patch, print-summary).

---

## Docs & repo

- [Blog: Run, Isolate, and Act](https://dev.to/jamie-steele/run-isolate-and-act-a-minimal-primitive-for-container-workflows-553m)
- [Contributing](CONTRIBUTING.md) (includes **[platform testing](CONTRIBUTING.md#platform-testing-we-need-you)** — we can’t validate every OS/arch; help on yours) · **[Manual QA](docs/manual-qa.md)** ([core](docs/manual-qa-core.md) · [macOS](docs/manual-qa-macos.md) · [Windows/WSL](docs/manual-qa-windows.md)) · [Architecture](docs/architecture.md) · **[Workflow YAML](docs/workflow-yaml.md)** · [CLI reference](docs/cli-reference.md) · [Chaining](docs/chaining.md) · [Install](docs/install.md) · [Releasing](docs/releasing.md) · [Branching & CI](docs/branching.md) · [AGENTS.md](AGENTS.md)
- **Tests:** `bash tests/run_tests.sh` (unit tests, from repo root). **Integration tests** (Docker + agent-dev): [tests/integration-tests/README.md](tests/integration-tests/README.md) → `bash tests/integration-tests/run.sh`

## Disclaimer

**Not legal advice.** dockpipe is **open-source software under active development** (pre-1.0): APIs, flags, and behavior can change between releases.

It is provided **“as is”** under the [Apache License 2.0](LICENSE), which includes **no warranty** and **limits liability** — read the license for the exact terms.

**Use at your own risk.** The tool runs **commands in containers** and can be configured to run **scripts on the host** (e.g. git actions, mounts). You are responsible for what you execute, for **reviewing** workflows, mounts, env, and actions before use, and for **backing up** important data. Do not rely on it for safety- or compliance-critical systems without your own review and testing.

---

**License:** Apache-2.0. See [LICENSE](LICENSE).
