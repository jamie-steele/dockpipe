# Docs Generation

Read when building docs workflows, documentation orchestration, or agent-created repo guidance.

## Preferred Pattern

- User intent lives in workflow YAML.
- DorkPipe materializes request, plan, task, merge, verify, usage, halt, and approval artifacts.
- Worker prompts and access policy come from `steps[].agent`.
- Skills route reusable assistant behavior; docs keep repo-local truth.

## Avoid

- one shell script per docs workflow shape
- generated docs promoted without verification
- stale docs that reference deleted workflows/scripts
- target-specific skill routing
- burying access/cost policy in prompt prose only

## Required Outputs

Docs workflows should produce reviewable artifacts:

- request/plan/task graph
- per-task results with citations/claims
- merge synthesis
- verifier result
- approval record or explicit next action

## Checks

- `./src/bin/dockpipe workflow validate <config.yml>`
- search for stale workflow/script names
- verify generated claims against repo files
- report human approval boundary
