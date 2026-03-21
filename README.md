# dockpipe

Run the CLI commands you already use in a disposable container. Your project is at **`/work`**; when the process exits, the container is gone.

Dockpipe runs any CLI command in a disposable container with your project at **`/work`**. It handles the usual Docker wiring (mount, workdir, user) so you don’t have to, and removes the container when the command finishes. Use it when you want a clean, throwaway environment for tests, linters, or other tools—without leftover containers on your machine.

**At a glance**

- **`dockpipe -- <command>`** — project at **`/work`**, your **uid/gid**, container removed when the command exits.
- **No flag soup** — same defaults every time instead of hand-rolling `docker run -v … -w … -u … --rm …`.
- **Optional power** — workflows (`--workflow`), runtimes / resolvers, and strategies only when you need them.

**Troubleshooting:** `dockpipe doctor` checks **bash**, **Docker**, and bundled assets.

## Try it

```bash
dockpipe -- pwd
dockpipe -- ls
dockpipe -- echo ok
```

You need **Docker** and **bash** on the host. See **[Install](#install)**.

No manual cleanup—you keep using the same commands (`make`, tests, linters—whatever you pass after `--`).

---

## Install

[docs/install.md](docs/install.md) · [GitHub Releases](https://github.com/jamie-steele/dockpipe/releases)

---

**Under the hood:** Dockpipe follows a simple flow—**run** (optional prep on your machine), **isolate** (your command in the container), **act** (optional script on your machine after the container exits). Most of the time you only use the middle step: **`dockpipe -- <command>`**.

**Conceptual model:** **workflow** = execution intent · **runtime** = isolated environment (platform-agnostic) · **resolver** = tool/platform adapter · **strategy** = lifecycle before/after · **`runtime.type`** = behavior class (`execution` / `ide` / `agent`). Canonical definitions and examples: **[docs/architecture-model.md](docs/architecture-model.md)**. Refactor plan: **[docs/runtime-architecture.md](docs/runtime-architecture.md)**.

Optional **named workflows** in YAML — bundled **scaffolding** via **`--workflow <name>`**, or any path with **`--workflow-file`**. **Templates do not define architecture** — see **[docs/architecture-model.md](docs/architecture-model.md)**. Core **workflow** *names* in this repo include **`init`**, **`test`**, **`run`**, **`run-apply-validate`** (under **`templates/`**). **Clone + worktree + commit** with resolvers uses the **`worktree`** **strategy** (**`templates/core/strategies/worktree`**) in **your** workflow YAML — see **[docs/workflow-yaml.md](docs/workflow-yaml.md#named-strategies)**. **`templates/core/resolvers/`** holds **resolver** bundles only. See **[docs/isolation-layer.md](docs/isolation-layer.md)**. **Learning path:** **[docs/onboarding.md](docs/onboarding.md)** · **[docs/workflow-yaml.md](docs/workflow-yaml.md)** · **[docs/architecture-model.md](docs/architecture-model.md)**.

---

## Why not just `docker run`?

Less typing for the common case:

```bash
docker run --rm -v "$(pwd):/work" -w /work -u "$(id -u):$(id -g)" some-image make test
```

dockpipe does the wiring, optional **act** phase, and a pipe-friendly CLI. On Linux/macOS, file ownership matches your user.

---

## Usage

```text
dockpipe [options] -- <command> [args...]
dockpipe workflow validate [path]
dockpipe action init [--from <bundled>] <filename>
dockpipe template init [--from <bundled>] <dirname>
dockpipe windows setup [--distro <name>] [--install-command <cmd>] [--non-interactive]
dockpipe windows doctor
dockpipe doctor
```

**All flags** (`--workflow`, `--isolate`, `--act`, `--repo`, `--data-dir`, …): **[docs/cli-reference.md](docs/cli-reference.md)**

---

## Examples

**Single run:**

```bash
dockpipe -- ls -la
dockpipe -- bash -c "npm test"
dockpipe --isolate dev -- make test
```

**Run a script, then a host script after the container exits:**

```bash
dockpipe --act scripts/commit-worktree.sh -- ./my-script.sh
```

**Chaining** — same workdir, fresh container each time; see [docs/chaining.md](docs/chaining.md):

```bash
WORKDIR="/path/to/your/project"
dockpipe --workdir "$WORKDIR" -- make lint && dockpipe --workdir "$WORKDIR" -- make test
```

**AI + worktree** (optional):

```bash
dockpipe --resolver claude --repo https://github.com/you/repo.git --branch task -- claude -p "Fix the bug"
```

**AI in repo dir + commit on host:**

```bash
cd /path/to/repo
dockpipe --isolate agent-dev --act scripts/commit-worktree.sh -- claude -p "Your prompt"
```

**Detach** (container runs in background):

```bash
dockpipe -d -- make test
```

---

## How it works

**Bundled assets:** **`templates/`** (including **`templates/core/`**: runtimes, resolvers, strategies, **`assets/`** — scripts, images, compose) and **`lib/entrypoint.sh`** are embedded in the binary and unpacked to your **user cache** on first use (override with **`DOCKPIPE_REPO_ROOT`** for development). **`dockpipe init`** scaffolds the **current project** (and optional **`templates/<name>/`**); **`dockpipe template init`** creates a workflow folder (often with **`--from`** a bundled name). Project-local scripts live under top-level **`scripts/`** after init (samples); YAML **`scripts/…`** paths resolve there first, then bundled **`templates/core/assets/scripts/`**. See **[docs/templates-core-assets.md](docs/templates-core-assets.md)** for the bundled surface and compliance notes.

**Data:** By default a named volume **`dockpipe-data`** is mounted at **`/dockpipe-data`** with **`HOME`** there so tool state can persist between runs. Use **`--data-dir`**, **`--no-data`**, or **`--reinit`** to change that.

---

## Resolvers and worktree (optional)

**Resolvers** map a name (e.g. `claude`, `codex`) to an image and defaults — or, for **`cursor-dev`** / **`vscode`**, to a **bundled delegate** (`DOCKPIPE_RESOLVER_WORKFLOW`), same as **`--workflow cursor-dev`** / **`vscode`** but under the **`worktree`** **strategy** with clone/commit. **`--resolver code-server`** uses the **`vscode`** Docker image for a normal container command. **`--resolver`** with **`--repo`** / **`--branch`** drives worktree-on-host flows. Add **`strategy: worktree`** (and **`strategies:`** / **`default_resolver:`** as needed) in **`templates/<name>/config.yml`** — see **[docs/workflow-yaml.md](docs/workflow-yaml.md#named-strategies)**.

---

## Templates

| Template | Description |
|----------|-------------|
| `base-dev` | Light: git, curl, bash, ripgrep, jq. |
| `dev` | base-dev + build-essential, ssh, etc. |
| `agent-dev` | Node + Claude Code. `claude` is an alias. |
| `codex` | Node + OpenAI Codex CLI. |
| `vscode` | OSS code-server **image** stack (`codercom/code-server` + dockpipe entrypoint) for **`TemplateBuild`**. **`--resolver code-server`** uses workflow **`code-server`** (container isolate). Browser IDE on the host is the **`vscode`** **runtime** profile, not a separate workflow row. |

---

## Bundled workflows

YAML presets via **`--workflow <name>`** (bundled under **`templates/`**; resolver delegate YAML also loads via the same flag from **`templates/core/resolvers/<name>/config.yml`** — see **[docs/workflow-yaml.md](docs/workflow-yaml.md)**):

| Workflow | Role |
|----------|------|
| `test` | Multi-step **outputs** chain (`.dockpipe/outputs.env`) — [templates/test/README.md](templates/test/README.md) |
| `run` | **Simple git:** run in a container, then **one commit on your current branch** (no worktrees) — [templates/run/README.md](templates/run/README.md) |
| `run-apply-validate` | **run → apply → validate** pipeline (three steps; customize cmds) — [templates/run-apply-validate/README.md](templates/run-apply-validate/README.md) |

**Resolvers** (tool/platform integration — not separate top-level templates): **`claude`**, **`codex`**, **`code-server`**, **`cursor-dev`**, **`vscode`** live under **[templates/core/resolvers/](templates/core/resolvers/)** (each **`profile`** + optional **`config.yml`**). **Clone + worktree + commit** uses **strategy `worktree`** in your workflow YAML — **[docs/workflow-yaml.md](docs/workflow-yaml.md#named-strategies)**.

---

## Act scripts

Scripts for the **act** phase (after the main command in the container). Bundled **commit-worktree** is a common choice. **`dockpipe action init my-action.sh --from commit-worktree`** (or `export-patch`, `print-summary`).

---

## Docs & repo

- [Blog: Run, Isolate, and Act](https://dev.to/jamie-steele/run-isolate-and-act-a-minimal-primitive-for-container-workflows-553m) · **Source draft:** [docs/releases/blog-dockpipe-primitive.md](docs/releases/blog-dockpipe-primitive.md)
- **[Onboarding](docs/onboarding.md)** (learning path) · **[Messaging](docs/messaging.md)** (GitHub About / canonical copy) · [Contributing](CONTRIBUTING.md) (includes **[platform testing](CONTRIBUTING.md#platform-testing-we-need-you)**) · **[Security](SECURITY.md)** · **[Manual QA](docs/qa/manual-qa.md)** ([core](docs/qa/manual-qa-core.md) · [macOS](docs/qa/manual-qa-macos.md) · [Windows/WSL](docs/qa/manual-qa-windows.md)) · [Architecture](docs/architecture.md) · **[Workflow YAML](docs/workflow-yaml.md)** · [CLI reference](docs/cli-reference.md) · [Chaining](docs/chaining.md) · [Install](docs/install.md) · [Releasing](docs/releases/releasing.md) · [Branching & CI](docs/releases/branching.md) · [AGENTS.md](AGENTS.md)
- **Tests:** `bash tests/run_tests.sh` (unit tests, from repo root). **Integration tests** (Docker + agent-dev): [tests/integration-tests/README.md](tests/integration-tests/README.md) → `bash tests/integration-tests/run.sh`

## Disclaimer

**Not legal advice.** dockpipe is **open-source software under active development** (pre-1.0): APIs, flags, and behavior can change between releases.

It is provided **“as is”** under the [Apache License 2.0](LICENSE), which includes **no warranty** and **limits liability** — read the license for the exact terms.

**Use at your own risk.** The tool runs **commands in containers** and can be configured to run **scripts on the host** (e.g. git actions, mounts). You are responsible for what you execute, for **reviewing** workflows, mounts, env, and actions before use, and for **backing up** important data. Do not rely on it for safety- or compliance-critical systems without your own review and testing.

---

**License:** Apache-2.0. See [LICENSE](LICENSE).
