# DockPipe

DockPipe runs commands and workflows in disposable isolated environments.

Start with a command. Turn repeatable commands into workflows. Compile/package when
you want reusable artifacts. Security policy and Docker image artifacts are
available when you need stricter or faster runs, but they do not have to be the
first thing you learn.

## Quick Start

```bash
make dev-install
dockpipe init
dockpipe -- pwd
```

Requires **Docker** and **bash**. Use a
[release binary](https://github.com/jamie-steele/dockpipe/releases) instead of
`make dev-install` when you are not in a clone. `dockpipe doctor` checks your
setup.

Your project is mounted at `/work` in a disposable container. When the command
exits, the container is gone.

## Product Story

| Need | Start here |
|------|------------|
| Run one command in isolation | `dockpipe -- <command>` |
| Reuse a sequence of commands | `workflows/<name>/config.yml` |
| Pick where/how it runs | `runtime` + `resolver` |
| Run something on the host | `kind: host` |
| Reuse/share workflows | `dockpipe build` and package metadata |
| Harden or speed up container runs | `security` and image artifacts |

## Tiny Workflow

```yaml
name: test
runtime: dockerimage
resolver: codex

steps:
  - cmd: npm test
```

Run it with:

```bash
dockpipe --workflow test --
```

## Docs

| Goal | Doc |
|------|-----|
| First run | [docs/onboarding.md](docs/onboarding.md) |
| Write workflow YAML | [docs/workflow-authoring.md](docs/workflow-authoring.md) |
| Full workflow reference | [docs/workflow-yaml.md](docs/workflow-yaml.md) |
| Compile/package reusable artifacts | [docs/package-quickstart.md](docs/package-quickstart.md) |
| Package/store model | [docs/package-model.md](docs/package-model.md) |
| Security policy | [docs/security-policy.md](docs/security-policy.md) |
| Docker image artifacts | [docs/image-artifacts.md](docs/image-artifacts.md) |
| CLI reference | [docs/cli-reference.md](docs/cli-reference.md) |
| Architecture terms | [docs/architecture-model.md](docs/architecture-model.md) |

Full index: [docs/README.md](docs/README.md).

## Development

```bash
make dev-deps
make dev-install
make test        # fastest: Go tests only
make test-quick  # Go + path guard + bash unit tests
make ci          # full Linux CI mirror
```

After `make build`, this repo can dogfood DockPipe like any project:

```bash
./src/bin/dockpipe --workflow <name> --workdir . --
```

Maintainer-specific tools such as DorkPipe, Pipeon, and MCP live under
`packages/`; see [docs/core-tools.md](docs/core-tools.md) when working on those
first-party packages.

## Disclaimer

DockPipe is open-source (Apache-2.0). It runs commands in containers and can run
scripts on the host; review what you execute. Pre-1.0: flags and behavior may
change between releases.
