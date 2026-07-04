# Architecture

Read when changing concepts, docs, workflow semantics, packages, or engine behavior.

## Core Action

DockPipe's engine has one action:

1. spawn an isolated environment
2. run a command inside it
3. optionally act on the result

Capabilities are separate dotted ids documented in `docs/concepts/capabilities.md`; they are not this three-step action.

## Core Concepts

| Concept | Meaning | Authoring location |
| --- | --- | --- |
| Workflow/template | What happens. YAML, steps, scripts. | `src/core/workflows/`, repo `workflows/`, package `workflows/` |
| Runtime | Where execution runs. | `src/core/runtimes/`, installed core |
| Resolver | Which tool/profile performs work. | `src/core/resolvers/`, package `resolvers/` |
| Strategy | Lifecycle wrapper around execution. | `src/core/strategies/` |

## Hard Rules

- Do not confuse runtimes and resolvers.
- Packaged workflow invocation is a step form: `workflow:` plus `package:`.
- Not runtimes: Kubernetes, cloud APIs, Terraform, AI providers. Use runtimes/resolvers plus scripts.
- `runtime.type: agent` classifies behavior. It is not a separate DockPipe product model.
- DorkPipe is a package/harness on top of DockPipe primitives.
- DockPipe is extended with workflows, runtimes, resolvers, and strategies; not plugins, core branching, or special-case flags.
- Runtime-owned Git sessions are runtime lifecycle behavior. Agents and resolvers request lifecycle
  transitions; they do not own raw Git commands.

## `src/core` Shape

Valid top-level categories in this repo's `src/core/`:

- `runtimes/`
- `resolvers/`
- `strategies/`
- `assets/`
- `workflows/` for bundled examples only

Invalid examples: `src/core/claude`, `src/core/docker`, `src/core/test`.
If it is a tool/platform, put it under `resolvers/`. If it is an execution substrate, put it under `runtimes/`.

## Mental Model

- Top-level `runtime` and `resolver` set workflow defaults.
- Step-level `runtime` and `resolver` override defaults.
- `isolate` is an advanced low-level image/template override.
- `kind: host` means host execution outside container isolation.
- `workflow` plus `package` enters a packaged child workflow.

## Canonical Docs

- `docs/concepts/architecture-model.md`
- `docs/concepts/capabilities.md`
- `docs/runtime/operation-results.md`
- `docs/runtime/git-runtime-sessions.md`
- `docs/workflows/workflow-yaml.md`
