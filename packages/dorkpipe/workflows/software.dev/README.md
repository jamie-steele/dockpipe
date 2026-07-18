# software.dev

`software.dev` is the package-owned governed software-development workflow. A consumer selects one
repo-local workflow step as its task pack; DorkPipe compiles that soft contract beneath fixed package
access, budget, approval, apply, publish, and sync policy.

# Consumer invocation

Run `software.dev` from the consumer repository root and keep `--workdir` on that same root. The
task-pack path is relative to that root, and the step id must exactly identify the one workflow step
whose `agent.orchestration` block is the task pack.

Static task-pack invocation:

```bash
dockpipe --package dorkpipe --workflow software.dev --workdir . \
  --var DORKPIPE_SOFTWARE_DEV_TASK_PACK=workflows/software-dev/config.yml \
  --var DORKPIPE_SOFTWARE_DEV_TASK_PACK_STEP=software_dev \
  --var DORKPIPE_SOFTWARE_DEV_PLANNER_MODE=false --
```

Planner invocation uses the same task-pack identity and changes only the mode:

```bash
dockpipe --package dorkpipe --workflow software.dev --workdir . \
  --var DORKPIPE_SOFTWARE_DEV_TASK_PACK=workflows/software-dev/config.yml \
  --var DORKPIPE_SOFTWARE_DEV_TASK_PACK_STEP=software_dev \
  --var DORKPIPE_SOFTWARE_DEV_PLANNER_MODE=true --
```

`DORKPIPE_SOFTWARE_DEV_TASK_PACK` is required. `DORKPIPE_SOFTWARE_DEV_TASK_PACK_STEP` defaults to
`software_dev`, but consumers should pass it explicitly so the selected identity is visible and
copy/paste-safe. `DORKPIPE_SOFTWARE_DEV_PLANNER_MODE` defaults to `false`. The package also accepts
its governed budget and lane variables, but consumer values can only narrow package ceilings and
cannot enable publish or sync. Provider authentication is needed only for the worker lanes selected
by the compiled contract.

Static mode compiles the selected task-pack graph without running the bootstrap planner. Planner
mode first materializes only the bounded planner graph, runs it, strictly parses one proposal, and
then replaces it with the compiled executable graph. Proposed tasks cannot run in the first phase.

## Thin repo-owned wrapper

A consumer may keep durable selection defaults in a local workflow. This is the complete wrapper;
the task pack still owns the repo request, tasks, quality rules, and apply target, while
`software.dev` still owns orchestration stages and hard policy.

```yaml
name: repo.software-dev
namespace: consumer

steps:
  - id: software_dev
    workflow: software.dev
    package: dockpipeproject
    vars:
      DORKPIPE_SOFTWARE_DEV_TASK_PACK: workflows/software-dev/config.yml
      DORKPIPE_SOFTWARE_DEV_TASK_PACK_STEP: software_dev
      DORKPIPE_SOFTWARE_DEV_PLANNER_MODE: "false"
```

With that file at `workflows/repo.software-dev/config.yml`, run it from the same consumer root:

```bash
dockpipe --workflow repo.software-dev --workdir . --
```

`software.dev` currently publishes raw workflow vars, not a typed PipeLang model, so `vars` is the
supported existing binding. A consumer should use `inputs` only when a packaged child actually
publishes typed fields; it should not duplicate these vars behind a repo-local type surface.

The wrapper is useful only when the repo wants stable path, step, or planner-mode defaults and a
short repeatable command. A package-level `brain.optimize`-style alias would add another authored
surface and handoff without improving safety, validation, cost, traceability, or rerun behavior, so
no such wrapper is included.

## Run artifacts and normal apply

The orchestration root is the run's artifact `orchestrate` scope, exported during execution as
`DORKPIPE_ORCH_ROOT`. Inspect these paths beneath it:

- `request.json`, `plan.json`, and `task-graph.json`
- `tasks/<id>/task.json`, `tasks/<id>/prompt.md`, worker results, and materialized outputs
- `proposal/metadata.json`, plus `proposal/raw.*` and `proposal/normalized.json` in planner mode
- `merge/final.md`, `verify/result.json`, `approval.md`, and `apply/result.json`

Normal workflow approval and apply are one boundary. After merge and verification, the approval
step defaults to review and asks the consumer to inspect `merge/final.md` and `verify/result.json`.
Only an approve decision recorded in `approval.md` lets apply copy the verified explicit or inferred
bundle into the task pack's `apply.target_root`; the ordered `apply.required_artifacts` remain the
minimum output floor. A review decision, failed verification, missing floor, duplicate output, or
out-of-root target blocks apply. Publish and sync remain absent.

## Planner promotion commands

After a compiled planner run has a passing `verify/result.json`, the package helper can evaluate a
review-only promotion candidate offline:

```bash
repo_root="$(pwd)"
artifact_root="/absolute/path/from-the-software.dev-run/orchestrate"

orchestrate-helper software-dev-evaluate-promotion \
  "$repo_root" workflows/software-dev/config.yml software_dev "$artifact_root"
```

The command atomically writes `proposal/promotion-candidate.json`. It can propose only reusable
soft-layer guidance and required-artifact floor additions for the exact selected task-pack surface.
It records exact target source digests and never patches the consumer repository.

Build a deterministic review patch after inspecting an eligible candidate. This remains a separate
command and still does not mutate the consumer repository:

```bash
orchestrate-helper software-dev-build-promotion-patch "$repo_root" "$artifact_root"
```

This writes `proposal/promotion-patch.json` and `proposal/promotion.patch` only under the run artifact
root. The manifest binds the candidate digest, exact task-pack step, optional exact sibling
`agents.yml`, target before/after digests, assigned soft changes, and textual patch digest.

Application requires a separately created JSON approval artifact under the run artifact root. The
consumer owns this decision: review `proposal/promotion.patch` and
`proposal/promotion-patch.json`, then copy the exact patch and target digests from that manifest into
`proposal/promotion-approval.json`:

```json
{
  "contract_version": "software.dev.promotion-approval/v1",
  "decision": "approve",
  "approved": true,
  "patch_sha256": "sha256:<promotion.patch digest>",
  "targets": [
    {"path": "workflows/software-dev/config.yml", "before_sha256": "sha256:<before digest>"}
  ]
}
```

```bash
orchestrate-helper software-dev-apply-promotion \
  "$repo_root" "$artifact_root" "$artifact_root/proposal/promotion-approval.json"
```

The helper never creates approval. It replays all source evidence, stages and validates all target
after-images, and applies only the exact digest-bound target set transactionally. The result is
`proposal/promotion-apply-result.json`.

The normal workflow approval does not approve promotion, candidate evaluation does not build a
patch, and patch generation does not create approval. Promotion application replays the bound
evidence and exact source digests before changing only the approved repo-owned soft targets.
