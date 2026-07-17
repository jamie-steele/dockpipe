# Docs Generation

Read when building docs workflows, documentation orchestration, or agent-created repo guidance.

## Preferred Pattern

- User intent lives in workflow YAML.
- DorkPipe materializes request, plan, task, merge, verify, usage, halt, and approval artifacts.
- Worker prompts and access policy come from `steps[].agent`.
- Skills route reusable assistant behavior; docs keep repo-local truth.
- Planner output is a session artifact first. Promote it to durable repo guidance only through the
  rules in `docs/agents/workflows/planner-promotion-model.md`.
- Use `docs/agents/workflows/ai-workflow-value-bar.md` for the direct-worker baseline, lane routing, and
  DAG/node evaluation rules before splitting docs work across agents.
- When generating consumer-repo guidance, seed package-owned deterministic baseline rules before
  repo-specific synthesis. Use
  `packages/dorkpipe/resolvers/dorkpipe/assets/docs/example-brain/index.md`.

## Avoid

- one shell script per docs workflow shape
- generated docs promoted without verification
- stale docs that reference deleted workflows/scripts
- target-specific skill routing
- burying access/cost policy in prompt prose only
- forcing every docs workflow into a rigid evidence ledger; use source/citation artifacts only when
  they improve the result over one strong worker
- leaking runtime mount points, artifact labels, or orchestration mechanics into durable
  consumer-repo guidance
- persisting fully compiled per-run prompts, exact task splits, or lane choices as repo config
  without promotion checks

## Required Outputs

Docs workflows should produce reviewable artifacts:

- request/plan/task graph
- per-task results with citations/claims
- merge synthesis
- verifier result
- failure class/root-cause or explicit reason no deeper evaluation was needed
- approval record or explicit next action

## Checks

- `./src/bin/dockpipe workflow validate <config.yml>`
- search for stale workflow/script names
- verify generated claims against repo files
- report human approval boundary
