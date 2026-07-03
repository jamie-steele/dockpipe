# TODO Pattern

Repos can benefit from a small indexed TODO system when brain docs identify real gaps that
should survive one run.

## Recommended shape

- Keep one open-only index file as the AI/backlog entrypoint.
- Link each active topic to its own markdown file.
- Keep status detail in the topic file, not in the index.
- Move completed topics to a closed folder or archive path instead of mixing open and closed items.

## Good consumer-repo topics

- source-of-truth gaps
- missing repo/design parity
- architecture ambiguities
- places where generated docs still leak runtime or tooling abstractions
- subsystem areas that need better durable guidance before more automation is added

## Things to avoid

- giant mixed TODO files with unrelated topics
- indexes that repeat the full contents of every topic file
- AI-only backlog wording that is meaningless to human maintainers

## Minimal contract

Each topic file should capture:

- the problem
- why it matters
- the current state
- the desired end state
- the next concrete step
