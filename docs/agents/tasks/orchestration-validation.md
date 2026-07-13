# TASK-002 Orchestration Validation And Source Walkers

## Still Open

- Expand deterministic verification around orchestration apply/publish paths so missing sources,
  broken references, and contradictory validation claims fail before writes.
- Add package-owned deterministic source walkers for broad mounted roots and external corpora so
  cheap/local lanes consume bounded fact packets instead of pretending they performed source-root
  discovery on their own.
- Add automatic prompt-brief compression for custom or weaker lanes such as Ollama so DorkPipe can
  derive bounded context artifacts from canonical docs before prompt assembly instead of requiring a
  permanent normalized duplicate docs tree in the repo.

## Implementation Update — Prompt Lane Context (2026-07-13)

The compiled task prompt now makes its execution selection explicit: requested and selected lane,
provider, model, task class/authority, selection rationale, and task `model_policy`. It labels those
values as operational run metadata, prohibits turning them into durable repository policy, and
requires source evidence rather than treating the lane as authority. Focused helper coverage fixes
that prompt contract; source-packet walkers and broader prompt-brief compression remain open.
