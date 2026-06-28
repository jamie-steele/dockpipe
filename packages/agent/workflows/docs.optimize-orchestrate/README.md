# docs.optimize-orchestrate

`docs.optimize-orchestrate` is a Codex-led optimizer loop for the `docs.orchestrate` workflow.
It uses the DorkPipe orchestration harness, uses Ollama for cheap artifact-shape auditing, gives
Codex the repo agent instructions and routed DorkPipe docs, lets Codex make the code-change decision,
and writes reviewable proposal artifacts. It does not touch the working tree.

Run it after a `docs.orchestrate` attempt:

```bash
./src/bin/dockpipe --package agent --workflow docs.optimize-orchestrate --
```

Apply the validated Codex proposal to the working tree without committing:

```bash
DORKPIPE_OPTIMIZER_APPLY=1 ./src/bin/dockpipe --package agent --workflow docs.optimize-orchestrate --
```

Artifacts are written under:

```text
bin/.dockpipe/packages/dorkpipe/optimize/docs.orchestrate/
```

The proposed patch is:

```text
bin/.dockpipe/packages/dorkpipe/optimize/docs.orchestrate/proposed.patch
```

The workflow may propose changes only inside the optimizer workflow, the docs orchestration workflow,
and the DorkPipe optimizer/verifier scripts. By default it does not apply the patch and does not
create commits. With `DORKPIPE_OPTIMIZER_APPLY=1`, it applies the validated patch to the working tree
and still does not create commits.
