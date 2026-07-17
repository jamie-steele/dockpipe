# TASK-006 Example Brain Baseline — Closed

Completed: 2026-07-17

## Shipped

- Inventoried package-owned native guidance workflows. `example.brain` is currently the only
  eligible workflow that synthesizes durable consumer-repository documentation.
- Replaced its duplicated baseline literal with the package-owned `baseline-rules.md` asset through
  the reusable `example_brain_baseline` collector.
- Made the baseline deterministic: it is rendered before other shared context and prepended to every
  task before repo-specific synthesis.
- Added a package-owned durable-output policy for materialized Markdown and YAML. Repo-native paths
  pass unchanged; guest or host paths are rewritten only when one explicit mapping proves exactly
  one repo-relative target; external, ambiguous, or root-only references fail closed.
- Blocked machine host paths, runtime mount labels, and orchestration-only artifact/lane/provider
  terminology from durable output while retaining stable guest display paths in source packets.
- Added focused fixtures for baseline ordering, source precedence, repo-native references, mapped
  runtime references, duplicate and external mappings, host-path non-disclosure, and forbidden
  terminology.

## Follow-up

TASK-007 is unblocked to design the generic software-development workflow and task-pack contract.
No TASK-007 implementation shipped with this task.
