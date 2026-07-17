# software.dev

`software.dev` is the package-owned governed software-development workflow. A consumer selects one
repo-local workflow step as its task pack; DorkPipe compiles that soft contract beneath fixed package
access, budget, approval, apply, publish, and sync policy.

Direct invocation:

```bash
dockpipe --package dorkpipe --workflow software.dev --workdir . \
  --var DORKPIPE_SOFTWARE_DEV_TASK_PACK=workflows/software-dev/config.yml \
  --var DORKPIPE_SOFTWARE_DEV_TASK_PACK_STEP=software_dev --
```

Set `DORKPIPE_SOFTWARE_DEV_PLANNER_MODE=true` to run the single bounded bootstrap planner before
strictly parsing and compiling its proposal. Static task packs compile without planner execution.

After a compiled planner run has a passing `verify/result.json`, the package helper can evaluate a
review-only promotion candidate offline:

```text
orchestrate-helper software-dev-evaluate-promotion \
  <repo-root> <repo-relative-task-pack.yml> <selected-step-id> <artifact-root>
```

The command atomically writes `proposal/promotion-candidate.json`. It can propose only reusable
soft-layer guidance and required-artifact floor additions for the exact selected task-pack surface.
It records exact target source digests and never patches the consumer repository.

Build a deterministic review patch after inspecting an eligible candidate:

```text
orchestrate-helper software-dev-build-promotion-patch <repo-root> <artifact-root>
```

This writes `proposal/promotion-patch.json` and `proposal/promotion.patch` only under the run artifact
root. The manifest binds the candidate digest, exact task-pack step, optional exact sibling
`agents.yml`, target before/after digests, assigned soft changes, and textual patch digest.

Application requires a separately created JSON approval artifact under the run artifact root:

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

```text
orchestrate-helper software-dev-apply-promotion \
  <repo-root> <artifact-root> <approval.json>
```

The helper never creates approval. It replays all source evidence, stages and validates all target
after-images, and applies only the exact digest-bound target set transactionally. The result is
`proposal/promotion-apply-result.json`.

Publish and sync are disabled. Apply always requires approval and uses only the repo-selected
`apply.target_root` plus the compiled materialized output bundle.
