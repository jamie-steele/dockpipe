# AI Workflow Value Bar

Read when designing or changing DorkPipe AI workflows, direct edit workers, artifact DAGs, local/cloud lanes, or repo-mounted agent runs.

## Bar

An AI workflow is worth keeping only if DorkPipe makes it better than one strong direct worker on at least one dimension:

| Value | Means |
| --- | --- |
| Breadth | finds more relevant sources, files, logs, docs, or hypotheses |
| Safety | isolates writes, blocks bad apply/publish, preserves approval boundaries |
| Cost | uses cheap/local lanes only for non-authoritative work |
| Validation | catches errors before apply with checks that affect the run |
| Rerunability | reruns failed nodes instead of restarting everything |
| Traceability | leaves useful artifacts, metrics, and root-cause evidence |

If orchestration only adds handoffs, latency, token spend, or unused artifacts, collapse it back to one strong worker.

## Boundary

Users express intent, mounts, context, access, outputs, and domain guardrails. Do not make users author meta-justification such as `value_case` just to get governed AI.

DorkPipe owns:

- lane planning/escalation
- local/cloud budgets and halt state
- task/result/merge/verify/approval artifacts
- worker isolation and edit/apply boundaries
- validation hooks and deterministic checks
- DAG/node-level evaluation artifacts
- follow-up rerun mechanics

## Shapes

| Shape | Use when |
| --- | --- |
| Direct strong worker | task is coherent and source set is manageable |
| Direct edit + validation | worker should modify `/work`, but apply/publish needs checks |
| Artifact DAG | intermediate outputs must be reviewable before apply |
| Parallel discovery + strong synthesis | corpus/search space is broad |
| Evaluator-optimizer | criteria are explicit and repairable |
| Mixed artifact/edit | draft as artifacts, then targeted edit repair/apply |

Subagents/parallel workers are explicit choices, not defaults. Split only when a node has distinct
role authority, context, output contract, validation surface, or rerun value.

## Lane Policy

| Work class | Default | Escalate when |
| --- | --- | --- |
| inventory | deterministic walker, or local lane over supplied source packets | missed sources, malformed output, or no tool access |
| extraction/summarization | local or cheap cloud over supplied excerpts | source separation or exactness fails |
| source selection | strong cloud lane | source precedence affects durable output |
| architecture/routing | strongest practical reasoning lane | workflow shape, safety, or apply policy matters |
| authoring | strong lane sized to complexity | coherence/schema failures appear |
| code/edit repair | strongest needed coding lane | tests/checks require precise edits |
| deterministic checks | script/local | never, once scripted |
| LLM judge | bounded strong validation lane | qualitative correctness is needed |

Record requested lane, actual lane, budget, token use, fallback state, and escalation reason. Silent frontier-model use is a defect.

## Model Notes

Use role-tier aliases in workflows and lane catalogs instead of hardcoding rapidly changing model
names into agent docs:

- `strong_reasoning` for architecture, source precedence, validation, and design critique.
- `strong_coder` for implementation, refactor, repair, and tests.
- `cheap_extractor` for bounded inventory and summarization.
- `validator` for independent review and failure classification.

Resolve aliases to current Codex, Claude, Ollama, or other provider/model choices in package-owned
lane catalogs and environment config. Treat model/version names as operational configuration, not
durable repo guidance.

## Evaluation

Treat every run as an evaluation DAG. Use existing artifacts:

- `task-graph.json`
- `tasks/<id>/task.json`
- `tasks/<id>/result.json`
- `tasks/<id>/response.md`
- `merge/result.json`
- `verify/result.json`
- `cloud-usage.json`
- `halt.json`
- `approval.md`

Run summaries should include end-state status, per-node status, failure class, likely root-cause task, repair/rerun target, token/elapsed cost, and whether orchestration appears to beat the direct-worker baseline.

Prefer deterministic checks for syntax, links, schemas, tests, forbidden paths, and apply coherence. Use LLM judges for qualitative judgment after mechanical failures are removed.

## Local Lane Gap

Prompt-only local lanes should not be treated as source scouts. They can summarize bounded source
packets, but they cannot inspect mounted roots unless DorkPipe provides deterministic walker output.
Future package work should add source walkers that turn allowed roots into compact, authority-labeled
packets for cheap local extraction.

## Source

Long-form research and citations: `docs/research/agentic-workflow-research-2026.md`.
