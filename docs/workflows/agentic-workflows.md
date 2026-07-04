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

- `claude` and `codex` resolver profiles
- `ollama` resolver profile for local-model use
- portable cloud-lane diagnostics under `packages/agent/workflows/agent.cloud-lanes.doctor/`

The YAML-first docs orchestration dogfood now lives under root `workflows/` because it is specific
to this repository's docs, agent guidance, and DorkPipe artifact paths.

The package keeps the resolvers discoverable from the main `packages/` compile root without
changing the engine model.

## `docs.orchestrate`

`docs.orchestrate` is the current governed documentation example. It keeps user-facing intent in
workflow YAML and lets DorkPipe materialize the execution artifacts.

For repo guidance generation, pair this pattern with the package-owned example baseline at
`packages/dorkpipe/resolvers/dorkpipe/assets/docs/example-brain/`. That baseline seeds
repo-native wording, source precedence, conflict handling, and TODO/index patterns before any
repo-specific synthesis is written.

Its useful primitive is:

1. `steps[].agent` declares startup prompt, `AGENTS.md` context, readable paths, model settings,
   and orchestration policy.
2. `model_policy` declares cheap/local attempt, strong validation, and escalation intent.
3. DorkPipe selects model lanes from a package-owned catalog and writes lane-selection artifacts.
4. DorkPipe materializes request, plan, task, merge, verify, cloud-usage, halt, training, and
   approval artifacts.
5. Resolver profiles specialize worker execution for `ollama`, `codex`, and `claude`.
6. Cloud-backed workers share a budget ledger and halt marker.
7. Human approval remains explicit before any source-tree apply or manual promotion.

The contract lives under the DorkPipe artifact root:

`dockpipe scope workflow docs.orchestrate orchestrate`

Resolvers still specialize execution, but the core enabling primitive is the declared graph and its
typed artifacts, not generic "agent" language.

The local dev stack is the shared control-plane layer: `dorkpipe-stack` runs MCP/DorkPipe tooling,
Postgres stores pgvector-backed state, and Ollama provides local model lanes. Codex and Claude are
not persistent services in that stack; they stay isolated as ephemeral resolver workers for bounded
tasks.

## Generic Software-Dev Workflows

The intended reusable shape is a package-owned governed workflow plus repo-owned task packs. The
workflow owns orchestration mechanics; the repo owns source-of-truth rules, recurring task intent,
required artifacts, and quality bars.

Use YAML task packs first. They fit the existing authored surface and keep repo overrides readable.
PipeLang or a richer typed model can be added later only if it proves a reuse or composition benefit
that YAML cannot provide.

Planner output starts as a session artifact. Promote it into durable repo configuration only when it
fits the planner promotion model in `docs/agents/workflows/planner-promotion-model.md`. In short:

- hard runtime settings are DockPipe/DorkPipe-owned and not agent-designed at runtime
- soft repo settings can be proposed by agents and promoted after verification
- exact per-run task splits, lane choices, and experimental workflow rewrites remain artifacts until
  they pass promotion checks

Consumer repos should be able to invoke the generic workflow directly. Thin repo-local wrappers are
useful for local defaults, prompts, or convenience, and richer generated workflows can sit above the
generic contract after validation.

The future master-agent layer should be model-agnostic at the planning level. It may use Codex,
Ollama, Claude, or another configured lane, but privileged host actions must go through a governed
DockPipe MCP or host bridge that asks for approval and returns operation-result events. The model is
not the trust boundary; the bridge is. Build this CLI-first: terminal approval prompts and
non-interactive policy modes should use the same structured bridge requests and operation-result
events that a later UI can render.

Two executor modes are valid. Bridge mode is for containerized or non-host-sandboxed masters; all
host actions go through the bridge. Native sandbox mode is for executors such as Codex that already
have a trusted host sandbox and escalation path; safe host actions may run directly through that
sandbox, while privileged actions still require escalation. Both modes must produce the same event
stream so CLI, UI, logs, and artifacts stay consistent.

PipeDeck should sit on top of that same stream. It is launched through DockPipe, inherits
Pipeon-style launcher context, and surfaces workflow/agent/MCP/model-lane YAML through a modern UI.
It can run workflows, inspect artifacts/logs, show approvals, preview diffs and conflicts, and map
agents to markdown guidance without becoming a full IDE.

## Current limits

The YAML surface can now express the intended prompt/context/model/access policy, but DorkPipe still
has TODOs before it fully hides physical execution planning:

- escalation currently selects lanes and records training metrics, but outcome-weighted learning is
  still future work
- `agent.access` should be compiled into stronger runtime policy, not just prompt/context artifacts
- internal fanout now has a package-owned runner, but richer task splitting and dependency
  scheduling are still future work


## Tooling Surfaces

Agent tooling should expose the same contract from different angles, not create separate control
planes.

- DockPipe workflow YAML is the durable source of truth.
- DockPipe Language Support is the schema-facing editor layer for that YAML.
- The DorkPipe/Pipeon VS Code extension may provide richer chat, model browser, template designer,
  and run inspector surfaces, but those surfaces should read/write or import/export workflow YAML
  and package-owned catalogs.
- PipeDeck may provide the primary workflow control surface, but it should still
  read/write YAML-backed contracts and subscribe to the same operation-result/event stream as CLI
  runs.
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
  `workflows/agent/docs.orchestrate/skills/docs-orchestrate/SKILL.md`
- Codex UI metadata:
  `workflows/agent/docs.orchestrate/skills/docs-orchestrate/agents/openai.yaml`
- Claude-oriented guidance source:
  `workflows/agent/docs.orchestrate/skills/docs-orchestrate/references/claude-command.md`

These are thin onboarding/discovery layers over the DorkPipe contract. The contract remains the
source of truth; skills should not become a second control plane.
