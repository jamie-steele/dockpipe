---
name: docs-orchestrate
description: Use when a request in this repository should be handled as a DorkPipe orchestration task graph rather than a single worker pass, especially for planning, bounded fanout, merge, verification, and approval over Codex, Claude, or Ollama worker backends.
---

# Docs Orchestrate

Use this skill when the right primitive is:

1. plan
2. task graph
3. worker task artifacts
4. merge
5. verify
6. approve

Do not use this skill for a simple one-shot edit or explanation that does not benefit from task
decomposition.

## Workflow

1. Read the orchestration contract at `packages/dorkpipe/resolvers/dorkpipe/assets/docs/orchestration-contract.md`.
2. Check the current workflow wrapper at `workflows/agent/docs.orchestrate/`.
3. Inspect or generate artifacts under `bin/.dockpipe/workflows/docs.orchestrate/dorkpipe/orchestrate/`.
4. Treat `codex`, `claude`, and `ollama` as worker specializations under the same contract.
5. Prefer declarative `steps[].agent` YAML over workflow-specific shell scripts.
6. Change the shared contract scripts only when the primitive itself needs a new generic behavior.

## Read These Files

- `packages/dorkpipe/resolvers/dorkpipe/assets/docs/orchestration-contract.md`
- `packages/dorkpipe/resolvers/dorkpipe/assets/docs/request-contract.md`
- `workflows/agent/docs.orchestrate/README.md`

## Key Paths

- Workflow wrapper:
  `workflows/agent/docs.orchestrate/config.yml`
- DorkPipe planner:
  `packages/dorkpipe/resolvers/dorkpipe/assets/scripts/orchestrate-plan.sh`
- Generic worker runner:
  `packages/dorkpipe/resolvers/dorkpipe/assets/scripts/orchestrate-run-task.sh`
- Merge:
  `packages/dorkpipe/resolvers/dorkpipe/assets/scripts/orchestrate-merge-results.sh`
- Verify:
  `packages/dorkpipe/resolvers/dorkpipe/assets/scripts/orchestrate-verify-results.sh`
- Approval:
  `packages/dorkpipe/resolvers/dorkpipe/assets/scripts/orchestrate-approve.sh`

## Rules

- Keep the core primitive artifact-based.
- Keep workflow intent in `config.yml` rather than new shell glue or sidecar specs.
- If a change cannot be explained as task graph execution with typed artifacts, be suspicious.
- Put provider/backend differences in resolver execution, not in the contract shape.
- Keep human approval explicit before apply/publish behavior.
