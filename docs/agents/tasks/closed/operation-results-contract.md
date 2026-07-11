# TASK-001 Operation Results Contract Rollout — Closed

Completed: 2026-07-11

## Shipped

- Canonical Go operation results, human rendering, and `dockpipe.operation_event.v1` JSONL mirroring now cover the targeted runtime, build, package, install, workflow, release, session, and DorkPipe paths.
- The final bounded image-artifact persistence warnings now emit `run.image_artifact.manifest`, `run.image_artifact.cache`, and `run.image_artifact.index` results. Focused tests cover success and failure human rendering plus JSONL mirroring.

## Rehomed follow-up

- PipeDeck/Postgres projection remains with the PipeDeck/query backlog (TASK-008).
- Provider/session/approval events remain with TASK-013, TASK-007, and host-sandbox work.
- New package scripts that perform meaningful work must use `dockpipe result` or a shared package wrapper.

No further general CLI-output sweep is part of this closed task.