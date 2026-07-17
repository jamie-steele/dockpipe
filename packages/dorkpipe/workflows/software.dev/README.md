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

Publish and sync are disabled. Apply always requires approval and uses only the repo-selected
`apply.target_root` plus the compiled materialized output bundle.
