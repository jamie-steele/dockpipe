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
It never patches the consumer repository; approval-gated patch generation and application are a
later boundary.

Publish and sync are disabled. Apply always requires approval and uses only the repo-selected
`apply.target_root` plus the compiled materialized output bundle.
