# TODO-002 Orchestration Validation And Source Walkers

## Still Open

- Expand deterministic verification around orchestration apply/publish paths so missing sources,
  broken references, and contradictory validation claims fail before writes.
- Add package-owned deterministic source walkers for broad mounted roots and external corpora so
  cheap/local lanes consume bounded fact packets instead of pretending they performed source-root
  discovery on their own.
- Add automatic prompt-brief compression for custom or weaker lanes such as Ollama so DorkPipe can
  derive bounded context artifacts from canonical docs before prompt assembly instead of requiring a
  permanent normalized duplicate docs tree in the repo.
