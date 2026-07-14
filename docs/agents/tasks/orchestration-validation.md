# TASK-002 Orchestration Validation And Source Walkers

## Still Open

- Add package-owned deterministic source walkers for broad mounted roots and external corpora so
  cheap/local lanes consume bounded fact packets instead of pretending they performed source-root
  discovery on their own.

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

## Implementation Update — Local Prompt Brief Artifacts (2026-07-13)

Local/custom lanes now materialize a bounded `prompt-brief.md` from declared required-artifact and
seed context before prompt assembly. It reuses the existing deterministic excerpt ordering and byte
limits, is recorded in `task.json`, and supplies the local prompt without creating any durable
normalized documentation tree. Focused coverage verifies deterministic ordering, truncation, local
guidance, and artifact persistence.

## Implementation Update — Fail-Closed Apply Coherence (2026-07-13)

Deterministic apply-coherence defects now fail before workspace writes: missing apply sources,
broken staged Markdown or YAML references, and contradictory validation claims produce
`verify.status: fail`. Existing qualitative/heuristic `review` findings remain review-only, so the
concrete-diff inspection path is unchanged when the staged apply set is coherent.
