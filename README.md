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
```

Contributors: `make dev-deps` installs **govulncheck** and **gosec** (same checks as CI). Optional — not required to use DockPipe.

## Disclaimer

DockPipe is **open-source** (Apache-2.0). It runs **commands in containers** and can run **scripts on the host**; review what you execute. **Pre-1.0:** flags and behavior may change between releases.
