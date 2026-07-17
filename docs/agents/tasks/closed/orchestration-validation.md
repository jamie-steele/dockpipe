# TASK-002 Orchestration Validation And Source Walkers

Status: Closed

Completed: 2026-07-16

## Shipped Summary

DorkPipe now provides package-owned deterministic context for local/custom lanes without treating
those lanes as source scouts. Compiled task prompts carry explicit operational lane metadata;
`prompt-brief.md` preserves bounded declared briefing excerpts; and `source-packet.md` walks declared
repo or mounted roots under `access.read` with `access.deny`, text-only, generated/cache, symlink,
file-count, per-file byte, and total-byte enforcement.

Mounted external corpora retain stable guest-oriented packet paths such as
`/DesignNotes/reference/file.md`; machine-specific host mount paths are used only for filesystem
resolution and are never rendered into packet headings. Declared multi-root order is preserved and
each root is walked lexically. Focused fixtures cover external mount resolution, guest/host
equivalence, host-path non-disclosure, deterministic ordering, deny boundaries, binary and symlink
exclusion, all packet bounds, and fail-closed read authority. The package-owned `example.brain`
inventory task dogfoods a five-file `context.source_roots` declaration.

Final validation passed:

- `go test ./packages/dorkpipe/lib/orchestrationhelper`
- `./src/bin/dockpipe package test --workdir . --only dorkpipe`
- `./src/bin/dockpipe package compile workflows --workdir . --from packages/dorkpipe --force`
- `./src/bin/dockpipe package compile resolvers --workdir . --from packages/dorkpipe --force`
- `git diff --check`

## Implementation History — Prompt Lane Context (2026-07-13)

The compiled task prompt makes its execution selection explicit: requested and selected lane,
provider, model, task class/authority, selection rationale, and task `model_policy`. It labels those
values as operational run metadata, prohibits turning them into durable repository policy, and
requires source evidence rather than treating the lane as authority.

## Implementation History — Local Source Packets (2026-07-13)

For a local lane with explicit `context.source_roots`, the orchestration helper materializes a
bounded deterministic source packet under the task artifact directory and appends it to that local
prompt. Planning fails closed when declared source roots do not have readable authority.

## Implementation History — Local Prompt Brief Artifacts (2026-07-13)

Local/custom lanes materialize a bounded `prompt-brief.md` from declared required-artifact and seed
context before prompt assembly. It reuses deterministic excerpt ordering and byte limits, is
recorded in `task.json`, and supplies the local prompt without creating a durable normalized
documentation tree.

## Implementation History — Fail-Closed Apply Coherence (2026-07-13)

Deterministic apply-coherence defects fail before workspace writes: missing apply sources, broken
staged Markdown or YAML references, and contradictory validation claims produce
`verify.status: fail`. Existing qualitative/heuristic `review` findings remain review-only, so the
concrete-diff inspection path is unchanged when the staged apply set is coherent.
