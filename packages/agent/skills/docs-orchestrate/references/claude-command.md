Use the DorkPipe orchestration contract instead of a single-worker flow.

Follow this sequence:

1. Read `packages/dorkpipe/resolvers/dorkpipe/assets/docs/orchestration-contract.md`.
2. Inspect `packages/agent/workflows/docs.orchestrate/`.
3. Treat the work as:
   - plan
   - task graph
   - worker task artifacts
   - merge
   - verify
   - approve
4. Keep `codex`, `claude`, and `ollama` as worker specializations beneath the same contract.
5. Prefer editing `steps[].agent` in `config.yml` before creating any new workflow shell glue.
6. Only change `packages/dorkpipe/resolvers/dorkpipe/assets/scripts/orchestrate-*.sh` when the generic primitive itself needs to grow.

Primary files:

- `packages/agent/workflows/docs.orchestrate/config.yml`
- `packages/agent/workflows/docs.orchestrate/README.md`
- `packages/dorkpipe/resolvers/dorkpipe/assets/scripts/orchestrate-plan.sh`
- `packages/dorkpipe/resolvers/dorkpipe/assets/scripts/orchestrate-run-task.sh`
- `packages/dorkpipe/resolvers/dorkpipe/assets/scripts/orchestrate-merge-results.sh`
- `packages/dorkpipe/resolvers/dorkpipe/assets/scripts/orchestrate-verify-results.sh`
- `packages/dorkpipe/resolvers/dorkpipe/assets/scripts/orchestrate-approve.sh`
