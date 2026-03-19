# AGENTS.md — dockpipe maintainer and agent guide

This file explains the repository’s purpose, architecture, coding standards, and how to extend it. It is intended for human maintainers and AI agents that modify the repo.

---

## Repo purpose

**dockpipe** is a small, open-source CLI that provides a single primitive:

1. **Spawn** a container from a chosen image  
2. **Run** a user-supplied command or script inside it  
3. **Act** on the result via an optional action script (e.g. commit, export patch)

The core is **agent-agnostic** and **command-agnostic**. AI tools (Claude, Codex, etc.) are supported via templates and examples, not by hardcoding them in the core. Commit/cherry-pick/export are implemented as **actions** or **example scripts**, not as built-in behavior.

---

## Architecture

- **CLI (Go)** — `cmd/dockpipe` (thin `main`) + **`lib/dockpipe/application`** (flags, subcommands, run orchestration, Windows `windows setup` / `windows doctor`, **Windows `dockpipe.exe` → WSL argv forward + cwd via `wslpath` + Windows-path translation for path-like flags / init positionals (not after `--`)**) + **`lib/dockpipe/domain`** (workflow model, env/resolver rules, YAML parse including optional **`group.mode: async`**) + **`lib/dockpipe/infrastructure`** (FS, docker, bash pre-scripts, git). Loads **`config.yml`** / **`steps:`** with `gopkg.in/yaml.v3`, merges workflow env, runs pre-scripts via **`bash -c 'source …; env -0'`**, builds `docker run`, optional host **git** commit. **`make`** or `go build -o bin/dockpipe.bin ./cmd/dockpipe`. **User-facing workflow spec:** **`docs/workflow-yaml.md`**.
- **Launcher** (`bin/dockpipe`) — Bash script: exec `bin/dockpipe.bin` if present, else `go run ./cmd/dockpipe`.
- **Legacy** (`scripts/dockpipe-legacy.sh`, `lib/runner.sh`, `lib/config-vars.sh`) — All-bash path kept for reference; not the default.
- **Entrypoint** (`lib/entrypoint.sh`) — Runs inside every image. Executes the command (argv or `DOCKPIPE_CMD`), then runs `DOCKPIPE_ACTION` if set, then exits with the command’s exit code.
- **Images** — Dockerfiles in `images/`; they `COPY lib/entrypoint.sh` from the **repo root** build context. Build with `-f images/<name>/Dockerfile .` from repo root.
- **Templates** — Named presets (e.g. `base-dev`, `dev`, `agent-dev`, `claude`) that map to an image name and a build path. Resolved in the CLI; no plugin system. Prefer `agent-dev` over `claude` in docs for command-agnostic appeal.
- **Actions** — Shell scripts that run inside the container after the user command. They receive `DOCKPIPE_EXIT_CODE` and `DOCKPIPE_CONTAINER_WORKDIR`. Shipped under `scripts/` (e.g. commit-worktree.sh).

Data flow: **Host CLI → Docker → container entrypoint → user command → action (if any) → exit.**

---

## Coding standards

- **Go:** `go fmt`, table-driven tests where helpful; keep **`lib/dockpipe/*`** free of unnecessary deps. Domain = pure types/rules; infrastructure = I/O and docker/bash; application wires them; pre/act scripts stay Bash. **File baseline:** see `lib/dockpipe/README.md` (split `run.go` / `run_steps.go` / `workflow_env.go` — don’t let `run.go` absorb everything).
- **Shell:** Pre/act scripts and launcher: Bash with `set -euo pipefail` where used.
- **Naming:** `DOCKPIPE_*` for env vars used by the tool. Scripts and paths: lowercase, hyphenated (e.g. `commit-worktree.sh`).
- **No vendor lock-in:** The core must not depend on Claude, Codex, or any specific AI tool. Such logic lives in `templates/` or `images/claude/` (or similar).
- **Simplicity:** Prefer obvious, boring code. No hidden magic; no framework or plugin layer unless clearly justified.
- **Composition:** Keep the core minimal; add integrations and examples in a modular way (templates, actions, example scripts).

---

## Adding templates / images / actions

### New image (e.g. another AI tool)

1. Add `images/<name>/Dockerfile`. Use `COPY lib/entrypoint.sh` and set `ENTRYPOINT ["/entrypoint.sh"]` so the generic flow is preserved.
2. Build from repo root: `docker build -t dockpipe-<name> -f images/<name>/Dockerfile .`
3. In **`lib/dockpipe/infrastructure/template.go`** (`TemplateBuild`), map the template name → image and Dockerfile dir so `--isolate <name>` builds/runs correctly.
4. Document in README and in the template's README if applicable.

### New action

1. Add a script under `scripts/` (e.g. `scripts/my-action.sh`). It will run inside the container; use `DOCKPIPE_EXIT_CODE` and `DOCKPIPE_CONTAINER_WORKDIR` as needed.
2. Document in README and in the script. Users can copy it with `dockpipe action init my-copy.sh --from my-action`.

### New workflow template

1. Add a directory under `templates/<name>/` with config.yml, resolvers/, and a README.
2. Do not put vendor-specific or commit-specific logic in `lib/` or `bin/`; keep it in the template or scripts.

---

## Philosophy

- **Core = primitive only:** Spawn → run → act. No hardcoded commit behavior, no hardcoded AI tool.
- **Templates and actions are the extension points:** Simple, obvious names and file locations.
- **Documentation is first-class:** README, AGENTS.md, **`docs/workflow-yaml.md`**, and the rest of **`docs/`** should make the primitive, YAML contract, and extension model clear so users and contributors can add their own images and actions without reading the whole codebase.

---

## Contributing: keep it primitive

Contributions should extend the primitive (templates, actions, examples) or fix bugs in the core—not turn the core into an open-ended workflow engine beyond the documented **`steps:`** contract (**`docs/workflow-yaml.md`**) or add first-class support for specific tools.

**Do:**
- Add or improve templates, actions, and example scripts.
- Fix bugs in CLI/runner/entrypoint; improve docs and tests.
- Use env vars and `--mount` / `--env` for one-off needs; document patterns in examples or docs.

**Don’t (examples of what we don’t want):**
- **Branch or workflow flags in the core** — e.g. `--branch`, `--worktree`, “create branch for me.” The user’s repo state (current branch, workdir) is the contract; orchestration belongs in scripts or the caller.
- **Vendor- or AI-specific behavior in `bin/` or `lib/`** — e.g. “if command is claude then …”. Keep that in templates and examples.
- **Built-in worktree/clone/commit logic** — Those are actions or example scripts that use the primitive, not core features.
- **Plugin/registry system** — Templates and actions are the extension points; no dynamic loading or plugin API unless the current model clearly can’t scale.
- **Heavy orchestration in the core** — Retries, arbitrary DAGs, plugin registries: script around dockpipe (Makefile, shell, CI). **Declarative `steps:`** in workflow YAML is intentionally narrow (see **`docs/workflow-yaml.md`**); don’t grow it into a general-purpose engine beyond that contract.

When in doubt: if it can be done by a script that runs `dockpipe` and passes `--mount` / `--env`, prefer that over adding new flags or core behavior.

---

## Running Docker (or dockpipe) from inside a container

You can run `docker` or dockpipe **from inside** a container by mounting the host’s Docker socket. The inner run creates **sibling** containers on the same host daemon (no nested daemon).

**How:** Pass the socket as an extra mount:

```bash
dockpipe --mount /var/run/docker.sock:/var/run/docker.sock --isolate agent-dev -- your-command
```

**Use cases:**
- **Contributors:** Test a newer or patched dockpipe inside a container while the host runs a stable install. Clone your fork in the container (or mount it), mount the socket, run your version’s `dockpipe` from inside; it will create sibling containers via the host’s Docker.
- **CI or automation:** A job runs in a container but needs to start other containers (e.g. sidecar, one-off build). Same pattern: socket mount + Docker CLI in the image.
- **Any “docker from inside” need:** Build images, run sibling services, or chain containerized steps without leaving the first container.

**Caveat:** The image must have the Docker CLI installed for the inner `docker` (or dockpipe) to work. The default agent-dev image does not ship it; use a custom image or add it to a template if you need this pattern.

---

## Tests

- `tests/unit-tests/` contains CLI and runner tests (argument parsing, template/action resolution, basic smoke tests). `tests/integration-tests/` contains Docker-based integration tests.
- Run from repo root. Prefer practical assertions (exit codes, expected output) over heavy mocking.
- Adding a new template or flag should be accompanied by a small test where appropriate.

**Cross-platform:** CI does **not** replace real installs. Checklists: **[docs/manual-qa.md](docs/manual-qa.md)** ([core](docs/manual-qa-core.md), [macOS](docs/manual-qa-macos.md), [Windows/WSL](docs/manual-qa-windows.md)). Contributor expectations: **[CONTRIBUTING.md — Platform testing](CONTRIBUTING.md#platform-testing-we-need-you)**.

**Releases:** **`VERSION`** at repo root + **`releasenotes/X.Y.Z.md`**; merge to **`main`** runs **Release** (see **[docs/branching.md](docs/branching.md)**). PRs must **bump `VERSION`** and touch the matching release notes file.

**CI / security:** **`govulncheck`**, **`gosec`** with **`.gosec.json`** (`global.exclude`: CLI-typical noise — subprocess/git, user-chosen paths, workspace `0o755`/`0o644`, template `WalkDir`); **CodeQL** uses **`.github/codeql/codeql-config.yml`** (`security-extended`) via **`.github/workflows/codeql.yml`** → **Security → Code scanning**. Plus tests and release-notes gate — see **`docs/branching.md`**.

---

## Limitations and escape hatches

- **UID/GID:** The runner passes `-u "$(id -u):$(id -g)"` so container-created files in the workdir are owned by the host user. Custom images or root-written volumes can still cause permission issues.
- **State between chained runs:** No env var bridge; use the shared workdir (files) or stdout/stdin. Documented in [docs/architecture.md](docs/architecture.md).
- **When the primitive isn’t enough:** Orchestration (retries, fan-out), rich multi-step state, or heavy tooling may require scripting around dockpipe (Makefile, shell, or an orchestrator). See “When the primitive isn’t enough” in architecture.md. Maintainers can note “most complex workflow” or escape-hatch experiences here as they come up.

---

## What to avoid

See **Contributing: keep it primitive** for best practices and examples of features we don’t want. In addition:

- Do not add Claude- or vendor-specific logic to `lib/runner.sh` or `lib/entrypoint.sh`.
- Do not make commit/cherry-pick/export a required or default behavior of the core.
- Do not introduce a plugin/registry system unless the current template + action model proves insufficient.
- Do not leave dead code or prototype-only paths in the core; keep `bin/` and `lib/` minimal and stable.
