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

## Implementation Update — Local Source Packets (2026-07-13)

For a local lane with explicit `context.source_roots`, the orchestration helper now materializes a
bounded deterministic source packet under the task artifact directory and appends it to that local
prompt. Host-resolved `/work` and declared external mounts are accepted only when inside
`access.read`; denied roots, generated/cache directories, symlinks, and non-text files are excluded.
Planning fails closed when declared source roots do not have readable authority. Focused fixtures cover
allowed evidence and access-boundary rejection; broad prompt-brief compression remains open.
