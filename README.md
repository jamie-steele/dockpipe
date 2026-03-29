# DockPipe

DockPipe runs anything, anywhere, in isolation.

## Quick start

```bash
make dev-install
dockpipe init
dockpipe --workflow test --runtime docker
```

Requires **Docker** and **bash**. Use a [release binary](https://github.com/jamie-steele/dockpipe/releases) instead of `make dev-install` when you are not in a clone. **`dockpipe doctor`** checks your setup.

Your project is mounted at **`/work`** in a disposable container; when the command exits, the container is gone.

## Core tools in this repo

The **DockPipe** CLI is **`src/cmd/`** (entry) and **`src/lib/`**. **Pipeon Launcher** (DockPipe GUI) is **`src/apps/pipeon-launcher/`**. **Pipeon**, **DorkPipe**, and **MCP** are first-party under **`packages/`** (`pipeon`, `dorkpipe`, `dockpipe-mcp`). Optional **IDE** resolver trees may live under **`packages/`** or maintainer-only dirs — see **[docs/core-tools.md](docs/core-tools.md)**. **`make build`** produces **`src/bin/dockpipe.bin`** (launcher **`src/bin/dockpipe`**). **`make maintainer-tools`** builds **`packages/dorkpipe/bin/dorkpipe`** and **`packages/dockpipe-mcp/bin/mcpd`**. Running this repo on itself is **`./src/bin/dockpipe --workflow <name> --workdir . --`** once packages are compiled into **`.dockpipe/`** like any project. See also **[src/apps/README.md](src/apps/README.md)**.

## Concepts

| Term | Meaning |
|------|---------|
| **Workflow** | What happens — steps and structure in `config.yml`. |
| **Runtime** | Where execution runs. |
| **Resolver** | Which tool or platform. |
| **Strategy** | Optional before/after hooks on the host. |
| **Assets** | Shared scripts, images, and compose (bundled with DockPipe). |

Single command: **`dockpipe -- <command>`**. Add **`--workflow`**, **`--runtime`**, or **`--resolver`** when you use named presets.

## Example

```bash
dockpipe --isolate agent-dev -- npm test
```

## Docs

**Index:** [docs/README.md](docs/README.md)

| | |
|--|--|
| Install | [docs/install.md](docs/install.md) |
| Workflow YAML | [docs/workflow-yaml.md](docs/workflow-yaml.md) |
| CLI | [docs/cli-reference.md](docs/cli-reference.md) |
| Terms (full definitions) | [docs/architecture-model.md](docs/architecture-model.md) |
| Capabilities & resolver packages | [docs/capabilities.md](docs/capabilities.md) |
| Onboarding | [docs/onboarding.md](docs/onboarding.md) |
| Contributing | [CONTRIBUTING.md](CONTRIBUTING.md) |

## Development

```bash
make dev-deps
make dev-install
make test        # fastest: Go tests only
make test-quick  # Go + path guard + bash unit tests (no Docker)
make ci          # full Linux CI mirror (govulncheck, gosec, Docker, integration — see src/scripts/ci-local.sh)
```

**Accelerator (this repo):** same as any DockPipe project — compile what you need into **`.dockpipe/`**, then run by workflow name. After **`make build`**:

| Workflow | Example |
|----------|---------|
| **`dorkpipe-self-analysis`** | `./src/bin/dockpipe --workflow dorkpipe-self-analysis --workdir . --` |
| **`dorkpipe-self-analysis-stack`** | Compose sidecars (set **`DORKPIPE_DEV_STACK_AUTODOWN=0`** to leave Postgres+Ollama up) |
| **`dorkpipe-self-analysis-host`** | Host-only, no Docker |
| **`compliance-handoff`** | Print CI + self-analysis **signal paths** — **`docs/artifacts.md`** |

See the **`dorkpipe`** maintainer package **`README.md`** (resolver **`dorkpipe-self-analysis`**).

```bash
./src/bin/dockpipe --workflow dorkpipe-self-analysis --workdir . --
```

Contributors: **`make dev-deps`** installs **govulncheck** and **gosec** (CI parity) and tries **user-level** installs for **asciinema** + **agg** (for **`make demo-record`**). None of this is required to use DockPipe. For demo tools only: **`make install-record-deps`**.

Optional **Codex** workflows in CI (when **`DOCKPIPE_CI_CODEX=true`**): `DOCKPIPE_CI_CODEX=true OPENAI_API_KEY=... make ci`.

## Disclaimer

DockPipe is **open-source** (Apache-2.0). It runs **commands in containers** and can run **scripts on the host**; review what you execute. **Pre-1.0:** flags and behavior may change between releases.
