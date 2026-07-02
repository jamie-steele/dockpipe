# Docs TODO

Cross-cutting follow-ups that should not live only inside one feature doc.

## Operations And Results

- Add a shared operation-result contract for meaningful units of work so session/bootstrap/apply
  behavior stops inventing one-off status lines and ad hoc result shapes.
- Make the Go result type the canonical source for operation status, timing, IDs, CLI rendering,
  and structured event mapping.
- Split preflight from mutation for important runtime actions such as session creation, auth
  discovery, volume seed/sync, and publish so failures point at the real broken step.
- Keep the interactive CLI spinner/loading UX for in-flight work, but finalize each completed unit
  into a stable result line with duration and key identifiers.

## Orchestration And Validation

- Expand deterministic verification around orchestration apply/publish paths so missing sources,
  broken references, and contradictory validation claims fail before writes.
- Add package-owned deterministic source walkers for broad mounted roots and external corpora so
  cheap/local lanes consume bounded fact packets instead of pretending they performed source-root
  discovery on their own.
