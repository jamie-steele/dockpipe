# DorkPipe Agentic YAML

Use this skill when building agentic workflows through DockPipe.

## Core Position

DorkPipe is the harness. DockPipe remains the governed runtime. Agentic behavior should be declared
in YAML and materialized into artifacts by package scripts.

## Contract Shape

- `model_policy` expresses attempt, validation, and escalation intent.
- `steps[].agent.startup_prompt` declares starting instructions.
- `steps[].agent.include_agents_md` controls repo context inclusion.
- `steps[].agent.access.read/write/deny` declares access policy.
- `steps[].agent.orchestration` declares request, tasks, merge, and verify.

## Hard Rules

- Do not create one shell script per workflow shape.
- Do not hide cost escalation; require policy and approval boundaries.
- Keep output artifacts reviewable.
- Treat local models as cheap attempt lanes and cloud models as governed spend lanes.
