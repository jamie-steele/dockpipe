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

| | |
|--|--|
| Install | [docs/install.md](docs/install.md) |
| Workflow YAML | [docs/workflow-yaml.md](docs/workflow-yaml.md) |
| CLI | [docs/cli-reference.md](docs/cli-reference.md) |
| Terms (full definitions) | [docs/architecture-model.md](docs/architecture-model.md) |
| Onboarding | [docs/onboarding.md](docs/onboarding.md) |
| Contributing | [CONTRIBUTING.md](CONTRIBUTING.md) |

## Development

```bash
make dev-deps
make dev-install
make test        # fastest: Go tests only
make test-quick  # Go + path guard + bash unit tests (no Docker)
make ci          # full Linux CI mirror (govulncheck, gosec, Docker, integration — see scripts/ci-local.sh)
```

**Accelerator (this repo):** run DorkPipe self-analysis from DockPipe — isolated container, **`.dockpipe/paste-this-prompt.txt`** for Cursor, optional Compose sidecars. From repo root after **`make build`**:

| Command | What it does |
|---------|----------------|
| **`make self-analysis`** | `dorkpipe-self-analysis` — analysis only |
| **`make self-analysis-stack`** | Compose up → analysis → compose down (set **`DORKPIPE_DEV_STACK_AUTODOWN=0`** to keep sidecars) |
| **`make self-analysis-host`** | Host-only, no Docker |

See **`dockpipe/workflows/dorkpipe-self-analysis/README.md`** and **`docs/dorkpipe.md`**.

```bash
make self-analysis
```

Contributors: **`make dev-deps`** installs **govulncheck** and **gosec** (CI parity) and tries **user-level** installs for **asciinema** + **agg** (for **`make demo-record`**). None of this is required to use DockPipe. For demo tools only: **`make install-record-deps`**.

Optional **Codex** workflows in CI (when **`DOCKPIPE_CI_CODEX=true`**): `DOCKPIPE_CI_CODEX=true OPENAI_API_KEY=... make ci`.

## Disclaimer

DockPipe is **open-source** (Apache-2.0). It runs **commands in containers** and can run **scripts on the host**; review what you execute. **Pre-1.0:** flags and behavior may change between releases.
