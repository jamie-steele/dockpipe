# docs.optimize-orchestrate

`docs.optimize-orchestrate` is a local-first optimizer loop for the `docs.orchestrate` workflow.
It uses the DorkPipe orchestration harness, keeps workers on Ollama by default, writes reviewable
artifacts, asks for approval, then applies a constrained patch to the working tree without committing.

Run it after a `docs.orchestrate` attempt:

```bash
./src/bin/dockpipe --package agent --workflow docs.optimize-orchestrate --
```

Artifacts are written under:

```text
bin/.dockpipe/packages/dorkpipe/optimize/docs.orchestrate/
```

The proposed patch is:

```text
bin/.dockpipe/packages/dorkpipe/optimize/docs.orchestrate/proposed.patch
```

The workflow may edit only the docs orchestration workflow and the DorkPipe verifier heuristics. It
does not create commits.
