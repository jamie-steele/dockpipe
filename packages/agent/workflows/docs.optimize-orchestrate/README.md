# docs.optimize-orchestrate

`docs.optimize-orchestrate` is a Codex-led optimizer loop for the `docs.orchestrate` workflow.
It uses the DorkPipe orchestration harness, uses Ollama for cheap artifact-shape auditing, lets Codex
make the code-change decision, and writes reviewable proposal artifacts. It does not touch the working
tree.

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
does not apply the patch and does not create commits.
