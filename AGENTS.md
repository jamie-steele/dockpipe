# AGENTS.md - DockPipe Root Router

DockPipe is the governed, cross-platform runtime for commands, packages, environments, CI jobs, AI workflows, and deployable tooling.
The engine has one action: spawn -> run -> act.
DorkPipe is a DockPipe package and harness, not a replacement for DockPipe.
This file is a lightweight router. Load only the focused docs needed for the task.

Machine-readable routing: `docs/agents/index.yaml`.

## Global Hard Rules

| Rule | Where to read |
| --- | --- |
| Keep `src/lib/` and `src/cmd/` generic. No repo-specific package/workflow/staging knowledge in engine code. | `docs/agents/engine-boundary.md` |
| Preserve the architecture model: workflow/template = what, runtime = where, resolver = tool/profile, strategy = lifecycle wrapper. | `docs/agents/architecture.md` |
| Use package/store helpers for project/global paths. Do not hand-write bare `.dockpipe/internal` paths. | `docs/agents/core-package-model.md` |
| Prefer the scope model for generated paths: `cwd: artifacts` for simple producers, `cwd: repo` plus `scopes` only when checkout cwd is required. | `docs/agents/path-scopes.md` |
| Treat Git lifecycle as runtime-owned session behavior. Agents request checkpoint/sync/publish; they do not run raw Git. | `docs/agents/git-runtime-sessions.md` |
| Template/workflow work should stay in YAML/assets/scripts unless a general primitive is needed. | `docs/agents/yaml-workflows.md` |
| Package-specific behavior belongs inside package YAML/assets/scripts/docs/tests. | `docs/agents/package-authoring.md` |
| Keep authored YAML/schema/editor docs in sync when changing workflow/config surfaces. | `docs/agents/yaml-workflows.md` |
| Secrets must be references only. Never commit plaintext secrets or generated resolved templates. | `docs/agents/safety-guardrails.md` |
| Treat `bin/.dockpipe/` and `.dorkpipe/` as generated/read-only grounding unless the user asks to refresh. | `docs/agents/artifacts-and-mcp.md` |
| AI workflows must beat one strong direct worker on quality, safety, cost, review effort, or rerun value. DorkPipe owns the lower-level proof through artifacts/metrics, not user boilerplate. | `docs/agents/ai-workflow-value-bar.md` |

## Task Routing

| Task | Read first | Recommended skills |
| --- | --- | --- |
| Engine or CLI behavior | `docs/agents/engine-boundary.md`, `docs/agents/architecture.md`, `docs/agents/validation-commands.md` | `dorkpipe-core-review` |
| Workflow YAML or authored surface | `docs/agents/yaml-workflows.md`, `docs/agents/safety-guardrails.md` | `dorkpipe-yaml-workflows` |
| Path/scope/artifact migration | `docs/agents/path-scopes.md`, `docs/agents/yaml-workflows.md`, `docs/agents/core-package-model.md` | `dorkpipe-yaml-workflows`, `dorkpipe-package-authoring` |
| Git runtime sessions or workspace lifecycle | `docs/agents/git-runtime-sessions.md`, `docs/agents/architecture.md`, `docs/agents/path-scopes.md` | `dorkpipe-core-review`, `dorkpipe-yaml-workflows` |
| Package authoring | `docs/agents/package-authoring.md`, `docs/agents/core-package-model.md` | `dorkpipe-package-authoring` |
| Package promotion | `docs/agents/package-promotion.md`, `docs/agents/validation-commands.md` | `dorkpipe-package-authoring`, `dorkpipe-core-review` |
| Agentic/DorkPipe workflows | `docs/agents/ai-workflow-value-bar.md`, `docs/agents/model-escalation.md`, `docs/agents/docs-generation.md`, `docs/agents/yaml-workflows.md` | `dorkpipe-agentic-yaml`, `dorkpipe-yaml-workflows` |
| Docs or agent guidance | `docs/agents/token-optimization.md`, `docs/agents/skills.md` | `dorkpipe-token-optimization` |
| Artifacts or MCP | `docs/agents/artifacts-and-mcp.md`, `docs/agents/safety-guardrails.md` | `dorkpipe-core-review` |

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
- `docs/agents/repo-map.md`
- `docs/agents/architecture.md`
- `docs/agents/engine-boundary.md`
- `docs/agents/core-package-model.md`
- `docs/agents/path-scopes.md`
- `docs/agents/git-runtime-sessions.md`
- `docs/agents/git-runtime-auth.md`
- `docs/agents/yaml-workflows.md`
- `docs/agents/package-authoring.md`
- `docs/agents/package-promotion.md`
- `docs/agents/ai-workflow-value-bar.md`
- `docs/agents/model-escalation.md`
- `docs/agents/docs-generation.md`
- `docs/agents/artifacts-and-mcp.md`
- `docs/agents/validation-commands.md`
- `docs/agents/safety-guardrails.md`
- `docs/agents/token-optimization.md`
- `docs/agents/skills.md`

DockPipe runs anything, anywhere, in isolation. Keep it simple. Keep it composable.
