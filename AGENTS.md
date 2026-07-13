# AGENTS.md - DockPipe Root Router

DockPipe is the governed, cross-platform runtime for commands, packages, environments, CI jobs, AI workflows, and deployable tooling.
The engine has one action: spawn -> run -> act.
DorkPipe is a DockPipe package and harness, not a replacement for DockPipe.
This file is a lightweight router. Load only the focused docs needed for the task.

Machine-readable routing: `docs/agents/index.yaml`.

## Global Hard Rules

| Rule | Where to read |
| --- | --- |
| Keep `src/lib/` and `src/cmd/` generic. No repo-specific package/workflow/staging knowledge in engine code. | `docs/agents/core/engine-boundary.md` |
| Preserve the architecture model: workflow/template = what, runtime = where, resolver = tool/profile, strategy = lifecycle wrapper. | `docs/agents/core/architecture.md` |
| Use package/store helpers for project/global paths. Do not hand-write bare `.dockpipe/internal` paths. | `docs/agents/core/core-package-model.md` |
| Prefer the scope model for generated paths: `cwd: artifacts` for simple producers, `cwd: repo` plus `scopes` only when checkout cwd is required. | `docs/agents/core/path-scopes.md` |
| Treat Git lifecycle as runtime-owned session behavior. Agents request checkpoint/sync/publish; they do not run raw Git. | `docs/agents/runtime/git-runtime-sessions.md` |
| For Codex workspace-sandbox sessions, verify effective capabilities. Request a narrow reviewed host operation for unavailable Docker, remote Git, or network access; never bypass the sandbox. | `docs/agents/runtime/codex-sandbox-sessions.md` |
| Template/workflow work should stay in YAML/assets/scripts unless a general primitive is needed. | `docs/agents/workflows/yaml-workflows.md` |
| Package-specific behavior belongs inside package YAML/assets/scripts/docs/tests. | `docs/agents/packages/package-authoring.md` |
| Keep authored YAML/schema/editor docs in sync when changing workflow/config surfaces. | `docs/agents/workflows/yaml-workflows.md` |
| Secrets must be references only. Never commit plaintext secrets or generated resolved templates. | `docs/agents/runtime/safety-guardrails.md` |
| Treat `bin/.dockpipe/` and `.dorkpipe/` as generated/read-only grounding unless the user asks to refresh. | `docs/agents/runtime/artifacts-and-mcp.md` |
| AI workflows must beat one strong direct worker on quality, safety, cost, review effort, or rerun value. DorkPipe owns the lower-level proof through artifacts/metrics, not user boilerplate. | `docs/agents/workflows/ai-workflow-value-bar.md` |
| Main docs stay canonical for repo facts and public behavior. `docs/agents/` is the compressed routing/safety layer, not a shadow documentation tree. | `docs/agents/docs/docs-system.md` |
| Treat `docs/agents/task-index.yaml` as the AI entrypoint for the cross-cutting backlog and keep the linked task files current when a task materially completes or advances one of those items. | `docs/agents/task-index.yaml` |
| At each completed slice, ask whether to commit the current branch; commit only after explicit approval. Offer a compact linked next-slice prompt, or ask what is next when no work remains. | `docs/agents/docs/session-handoffs.md` |

## Task Routing

| Task | Read first | Recommended skills |
| --- | --- | --- |
| Engine or CLI behavior | `docs/agents/core/engine-boundary.md`, `docs/agents/core/architecture.md`, `docs/agents/core/validation-commands.md` | `dorkpipe-core-review` |
| Workflow YAML or authored surface | `docs/agents/workflows/yaml-workflows.md`, `docs/agents/runtime/safety-guardrails.md` | `dorkpipe-yaml-workflows` |
| Path/scope/artifact migration | `docs/agents/core/path-scopes.md`, `docs/agents/workflows/yaml-workflows.md`, `docs/agents/core/core-package-model.md` | `dorkpipe-yaml-workflows`, `dorkpipe-package-authoring` |
| Git runtime sessions or workspace lifecycle | `docs/agents/runtime/git-runtime-sessions.md`, `docs/agents/core/architecture.md`, `docs/agents/core/path-scopes.md` | `dorkpipe-core-review`, `dorkpipe-yaml-workflows` |
| Package authoring | `docs/agents/packages/package-authoring.md`, `docs/agents/core/core-package-model.md` | `dorkpipe-package-authoring` |
| Package promotion | `docs/agents/packages/package-promotion.md`, `docs/agents/core/validation-commands.md` | `dorkpipe-package-authoring`, `dorkpipe-core-review` |
| Agentic/DorkPipe workflows | `docs/agents/workflows/ai-workflow-value-bar.md`, `docs/agents/workflows/model-escalation.md`, `docs/agents/workflows/docs-generation.md`, `docs/agents/workflows/planner-promotion-model.md`, `docs/agents/workflows/yaml-workflows.md` | `dorkpipe-agentic-yaml`, `dorkpipe-yaml-workflows` |
| Tooling app or UI surface | `docs/agents/workflows/tooling-surfaces.md`, `docs/agents/workflows/planner-promotion-model.md`, `docs/agents/tasks/agentic-app-ui.md` | `dorkpipe-agentic-yaml`, `dorkpipe-yaml-workflows` |
| Docs or agent guidance | `docs/agents/docs/token-optimization.md`, `docs/agents/docs/skills.md` | `dorkpipe-token-optimization` |
| Docs system or sync rules | `docs/agents/docs/docs-system.md`, `docs/agents/docs/token-optimization.md` | `dorkpipe-token-optimization`, `dorkpipe-core-review` |
| Artifacts or MCP | `docs/agents/runtime/artifacts-and-mcp.md`, `docs/agents/runtime/safety-guardrails.md` | `dorkpipe-core-review` |

## Skill Routing

Use target-independent skill ids. Do not write target-specific skill routing keys.
DorkPipe renders the same curated skills to Codex, Claude, or generic targets:

```bash
./src/bin/dockpipe --package dorkpipe --workflow skills.render -- --list
./src/bin/dockpipe --package dorkpipe --workflow skills.render -- --target codex
```

Installed Codex skills expected here:

- `dorkpipe-agentic-yaml`
- `dorkpipe-core-review`
- `dorkpipe-package-authoring`
- `dorkpipe-token-optimization`
- `dorkpipe-yaml-workflows`

## Forbidden Artifact Summary

Do not add or commit:

- plaintext secrets, resolved vault templates, local `.env` material with private values
- cache/build/generated state unless explicitly intended: `bin/.dockpipe/`, `.dorkpipe/`, `.staging` outputs, local assistant output
- repo-root one-off script shadows such as `scripts/dockpipe/...`
- engine references to checkout-only `packages/`, `workflows/`, or `.staging/` paths/names
- target-specific skill routing in AGENTS or `docs/agents/index.yaml`

## Before Editing

1. Identify task type in `docs/agents/index.yaml`.
2. Load only the routed docs and relevant skill instructions.
3. Check whether the work is engine, workflow, package, resolver, strategy, docs, or generated artifact.
4. If editing `src/`, verify the change is a general primitive.
5. If changing authored YAML semantics, update schema, docs, and language support together.
6. If touching packages, keep logic inside package assets and prefer repo-local binaries over `PATH`.
7. If moving generated paths, decide first whether the file is workflow artifact, package state, resolver scope, or source.
8. Check `git status --short` and do not revert unrelated user changes.

## Final Report Checklist

Report:

- what changed and why
- files or areas touched
- validations run and results
- any generated artifacts created
- risks, TODOs, or skipped checks
- whether package/engine boundaries were preserved

## Focused Docs

- `docs/agents/index.yaml`
- `docs/agents/core/repo-map.md`
- `docs/agents/core/architecture.md`
- `docs/agents/core/engine-boundary.md`
- `docs/agents/core/core-package-model.md`
- `docs/agents/core/path-scopes.md`
- `docs/agents/runtime/git-runtime-sessions.md`
- `docs/agents/runtime/codex-sandbox-sessions.md`
- `docs/agents/runtime/git-runtime-auth.md`
- `docs/agents/workflows/yaml-workflows.md`
- `docs/agents/packages/package-authoring.md`
- `docs/agents/packages/package-promotion.md`
- `docs/agents/workflows/ai-workflow-value-bar.md`
- `docs/agents/workflows/model-escalation.md`
- `docs/agents/workflows/docs-generation.md`
- `docs/agents/workflows/planner-promotion-model.md`
- `docs/agents/runtime/artifacts-and-mcp.md`
- `docs/agents/core/validation-commands.md`
- `docs/agents/runtime/safety-guardrails.md`
- `docs/agents/docs/token-optimization.md`
- `docs/agents/docs/skills.md`
- `docs/agents/docs/docs-system.md`
- `docs/agents/task-index.yaml`

DockPipe runs anything, anywhere, in isolation. Keep it simple. Keep it composable.
