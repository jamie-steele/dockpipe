# docs.optimize-orchestrate

`docs.optimize-orchestrate` is a Codex-led optimizer loop for the `docs.orchestrate` workflow.
It uses the DorkPipe orchestration harness, uses Ollama for cheap artifact-shape auditing, gives
Codex the repo agent instructions and routed DorkPipe docs, lets Codex make the code-change decision,
and writes reviewable proposal artifacts. It does not touch the working tree.

Run it after a `docs.orchestrate` attempt:

```bash
./src/bin/dockpipe --workflow docs.optimize-orchestrate --
```

Apply the validated Codex proposal to the working tree without committing:

```bash
DORKPIPE_OPTIMIZER_APPLY=1 ./src/bin/dockpipe --workflow docs.optimize-orchestrate --
```

Run repeated optimizer passes through the same workflow:

```bash
DORKPIPE_OPTIMIZER_ITERATIONS=15 ./src/bin/dockpipe --workflow docs.optimize-orchestrate --
```

The iteration count is declared in workflow YAML as `DORKPIPE_OPTIMIZER_ITERATIONS`. The default is
one pass so normal optimizer runs do not unexpectedly spend cloud budget. Setting it above one makes
the first step run earlier passes through the same workflow, then the current workflow completes the
final pass.

The optimizer workflow also sets `DORKPIPE_DEV_STACK_RELOAD=1` for its stack step. Each optimizer
pass rebuilds and recreates the requested Compose services so the DorkPipe MCP container sees the
latest local binaries and package-owned scripts instead of serving a stale stack.

Set `DORKPIPE_OPTIMIZER_REFRESH_TARGET_AFTER_APPLY=1` when you want each applied optimizer patch to
rerun the target `docs.orchestrate` workflow before the next optimizer pass. Target refresh runs
artifact-only with approval set to `auto-no` and apply skipped, so it regenerates evidence without
promoting docs.

Artifacts are written under:

```text
bin/.dockpipe/workflows/docs.orchestrate/dorkpipe/optimize/
```

Each run snapshots the previous optimizer proposal under:

```text
bin/.dockpipe/workflows/docs.orchestrate/dorkpipe/optimize/history/
```

Repeated-run snapshots are written under:

```text
bin/.dockpipe/workflows/docs.orchestrate/dorkpipe/optimize/iterations/
```

That handoff is included in the next Codex decision so repeated runs can build on earlier proposals
instead of starting cold.

The proposed patch is:

```text
bin/.dockpipe/workflows/docs.orchestrate/dorkpipe/optimize/proposed.patch
```

The workflow may propose changes only inside the optimizer workflow, the docs orchestration workflow,
and the DorkPipe optimizer/verifier scripts. By default it does not apply the patch and does not
create commits. With `DORKPIPE_OPTIMIZER_APPLY=1`, it applies the validated patch to the working tree
and still does not create commits.
