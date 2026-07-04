# Example Brain Baseline

Use this package-owned baseline when a workflow generates durable repo guidance for a consumer
checkout.

Seed these rules before repo-specific synthesis so the output reads like native repo guidance rather
than runtime or orchestration commentary.

## Read order

1. `baseline-rules.md` for deterministic rules that should be present before repo-specific facts.
2. `high-value-patterns.md` for the most useful guidance shapes to seed into a consumer repo.
3. `todo-pattern.md` for a minimal backlog/index pattern that keeps brain outputs maintainable.

## Purpose

- Give DorkPipe-native workflows a stable, package-owned baseline for example brain docs.
- Prevent runtime terms such as mounts, artifact roots, lanes, or worker mechanics from leaking into
  durable repo guidance unless the consumer repo explicitly treats them as product concepts.
- Encourage high-value durable outputs such as source-of-truth rules, conflict handling, and small
  maintained TODO systems instead of one-off summaries.
