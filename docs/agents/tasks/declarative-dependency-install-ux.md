# TASK-010 Declarative Dependency Install UX

## Current State

- Workflow YAML and package manifests can declare host tool dependencies with `dependencies.host`.
- Workflow YAML and package manifests can declare supported host platforms with top-level
  `platforms` (`windows`, `macos`, `linux`, `deb`).
- Core preflights required host dependencies before workflow execution and can run the current
  platform's author-provided installer after explicit approval.
- `workflows/ci/ci-emulate` declares `docker` and `act`, so missing `act` fails before any CI
  emulator script runs.

## Still Open

- Define trusted install command policy by OS/package manager (`winget`, `brew`, apt/dnf/pacman,
  custom enterprise scripts) without hardcoding product-specific dependencies.
- Add marketplace validation for dependency installer commands/scripts before packages are published
  or promoted.
- Expand platform certification beyond the currently tested Windows and Debian paths.
- Add package install-time dependency checks so package installation can surface missing host tools
  before first run.
- Migrate the whole repo over time: every first-party workflow, package, resolver script, CI helper,
  and maintainer command should declare external host tools in `dependencies.host` instead of hiding
  them in script bodies or README-only setup notes.
- Add structured operation-result units for dependency preflight and future dependency install
  attempts.
- Decide whether optional dependencies should appear in catalog/doctor output even when they do not
  block execution.

## Contract Sketch

```yaml
platforms: [windows, deb]
dependencies:
  host:
    - id: act
      command: act
      description: Runs GitHub Actions workflows locally.
      required: true
      install:
        windows: winget install nektos.act
        deb: sudo apt-get update && sudo apt-get install -y act
```

## Rules

- `depends` remains package-to-package graph metadata.
- Top-level `platforms` declares the host platforms the workflow/package supports.
- `dependencies.host` is for external host tools and author-provided OS package-manager install
  commands.
- Core owns validation and preflight; packages own dependency declarations.
- If a workflow/package declares a supported platform, all required dependencies and host scripts
  must work on that platform.
- New first-party workflows/packages should add dependency declarations when they introduce a host
  executable requirement.
- Auto-install must always cross an approval boundary unless a future explicit policy file permits it.
