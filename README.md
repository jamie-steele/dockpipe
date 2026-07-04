# DockPipe

DockPipe runs commands and workflows in disposable isolated environments.

Start with a command. Turn repeatable commands into workflows. Compile/package when
you want reusable artifacts. Security policy and Docker image artifacts are
available when you need stricter or faster runs, but they do not have to be the
first thing you learn.

## Quick Start

Most users should start with the packaged install flow, not a source checkout:

1. Install DockPipe from [GitHub Releases](https://github.com/jamie-steele/dockpipe/releases) using [docs/install.md](docs/install.md).
2. Run `dockpipe -- pwd`.
3. Read [docs/onboarding.md](docs/onboarding.md) for the first workflow path.

If you are contributing from a source checkout, use:

```bash
make dev-install
dockpipe init
dockpipe -- pwd
```

Requires **Docker** and **bash**. `make dev-install` is a contributor shortcut
for this repository; `dockpipe doctor` checks your setup.

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
| Write workflow YAML | [docs/workflows/workflow-authoring.md](docs/workflows/workflow-authoring.md) |
| Full workflow reference | [docs/workflows/workflow-yaml.md](docs/workflows/workflow-yaml.md) |
| Compile/package reusable artifacts | [docs/packages/package-quickstart.md](docs/packages/package-quickstart.md) |
| Package/store model | [docs/packages/package-model.md](docs/packages/package-model.md) |
| Security policy | [docs/security/security-policy.md](docs/security/security-policy.md) |
| Docker image artifacts | [docs/runtime/image-artifacts.md](docs/runtime/image-artifacts.md) |
| CLI reference | [docs/cli-reference.md](docs/cli-reference.md) |
| Architecture terms | [docs/concepts/architecture-model.md](docs/concepts/architecture-model.md) |

Full index: [docs/README.md](docs/README.md).

## Development

```bash
make dev-deps
make dev-install
make test        # build + Go tests + DockPipe package/workflow tests
make test-quick  # Go tests + package/workflow tests + path guard + bash unit tests
make ci          # full Linux CI mirror
```

After `make build`, this repo can dogfood DockPipe like any project:

```bash
./src/bin/dockpipe --workflow <name> --workdir . --
```

Maintainer-specific tools such as DorkPipe, Pipeon, and MCP live under
`packages/`; see [docs/packages/core-tools.md](docs/packages/core-tools.md) when working on those
first-party packages.

## Disclaimer

DockPipe is open-source (Apache-2.0). It runs commands in containers and can run
scripts on the host; review what you execute. Pre-1.0: flags and behavior may
change between releases.
