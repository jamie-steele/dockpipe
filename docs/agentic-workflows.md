# Governed AI Workflows

DockPipe is a governed cross-platform runtime and orchestration layer for commands, packages,
environments, CI jobs, AI workflows, and deployable tooling.

That distinction matters: AI agents are not the product surface by themselves. In DockPipe, an AI
worker is a specialized workflow/package stage that runs under an explicit contract:

- inputs
- runtime target
- resolver/model profile
- allowed commands
- denied commands
- output artifacts
- verification
- approval before promotion

This keeps AI work inside the same primitives DockPipe already uses everywhere else:

- workflows describe what happens
- runtimes describe where execution happens
- resolvers describe which tool/profile performs the work
- strategies wrap lifecycle before/after execution

## First tracked agent package

The repository now has a tracked first-party `packages/agent/` package.

It currently contains:

- promoted `claude` and `codex` resolver profiles from `.staging/packages/agent/`
- promoted `ollama` resolver profile for local-model use from `.staging/packages/agent/`
- a YAML-first workflow package at `packages/agent/workflows/docs.orchestrate/`

The staging copies remain in place for a separate reviewed cleanup step. Promotion makes the
resolvers discoverable from the main `packages/` compile root without changing the engine model.

## `docs.orchestrate`

`docs.orchestrate` is the current governed documentation example. It keeps user-facing intent in
workflow YAML and lets DorkPipe materialize the execution artifacts.

Its useful primitive is:

1. `steps[].agent` declares startup prompt, `AGENTS.md` context, readable paths, model settings,
   and orchestration policy.
2. `model_policy` declares cheap/local attempt, strong validation, and escalation intent.
3. DorkPipe selects model lanes from a package-owned catalog and writes lane-selection artifacts.
4. DorkPipe materializes request, plan, task, merge, verify, cloud-usage, halt, training, and
   approval artifacts.
5. Resolver profiles specialize worker execution for `ollama`, `codex`, and `claude`.
6. Cloud-backed workers share a budget ledger and halt marker.
7. Human approval remains explicit before any manual promotion.

The contract lives under the DorkPipe artifact root:

`bin/.dockpipe/packages/dorkpipe/orchestrate/docs.orchestrate/`

Resolvers still specialize execution, but the core enabling primitive is the declared graph and its
typed artifacts, not generic "agent" language.

## Current limits

The YAML surface can now express the intended prompt/context/model/access policy, but DorkPipe still
has TODOs before it fully hides physical execution planning:

- escalation currently selects lanes and records training metrics, but outcome-weighted learning is
  still future work
- `agent.access` should be compiled into stronger runtime policy, not just prompt/context artifacts
- internal fanout now has a package-owned runner, but richer dependency scheduling is still future
  work

## Tooling Surfaces

Agent tooling should expose the same contract from different angles, not create separate control
planes.

- DockPipe workflow YAML is the durable source of truth.
- DockPipe Language Support is the schema-facing editor layer for that YAML.
- The DorkPipe/Pipeon VS Code extension may provide richer chat, model browser, template designer,
  and run inspector surfaces, but those surfaces should read/write or import/export workflow YAML
  and package-owned catalogs.
- The template designer should be a visual editor for `model_policy`, `steps[].agent`,
  orchestration tasks, access declarations, verification gates, and approval gates.
- The model browser should show model lanes available to DorkPipe escalation: local models,
  CLI-backed cloud agents, capabilities, install/availability state, context limits, and budget
  policy.
- Extension-local state is acceptable for drafts, caches, and UI preferences, but should not become
  the only durable definition of a workflow, model lane, or escalation policy.

The intended user story is:

1. A user authors or opens a DockPipe workflow.
2. The editor and designer both surface the same `config.yml` contract.
3. Starting the stack prepares the declared local/cloud model lanes.
4. DorkPipe chooses the cheapest valid lane first and escalates according to `model_policy`.
5. Runs emit inspectable artifacts so the extension can explain what happened without owning the
   execution contract.

## Native Skill Surfaces

This repository now carries native provider-facing guidance over the same orchestration contract:

- Codex skill source:
  `packages/agent/skills/docs-orchestrate/SKILL.md`
- Codex UI metadata:
  `packages/agent/skills/docs-orchestrate/agents/openai.yaml`
- Claude-oriented guidance source:
  `packages/agent/skills/docs-orchestrate/references/claude-command.md`

These are thin onboarding/discovery layers over the DorkPipe contract. The contract remains the
source of truth; skills should not become a second control plane.
