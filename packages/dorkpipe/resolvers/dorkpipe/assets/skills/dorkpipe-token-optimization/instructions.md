# DorkPipe Token Optimization

Use this skill when editing agent-facing guidance.

## Goal

Keep routing precise while preserving repository-specific safety rules.

## Pattern

- Put the short router in `AGENTS.md`.
- Put focused task guidance under `docs/agents/`.
- Put reusable assistant behavior in DorkPipe skills.
- Use `docs/agents/index.yaml` to map task type to docs and skill ids.

## Compression Rules

- Prefer tables, checklists, and path maps.
- Keep one topic per file.
- Link to canonical docs instead of copying them.
- Avoid generic AI-agent advice.
- Avoid target-specific skill routing keys; use `skills`.
- Keep forbidden artifacts centralized instead of repeated everywhere.

## Review Pass

1. Identify which task types need the guidance.
2. Remove duplicated product philosophy.
3. Preserve hard rules exactly once.
4. Confirm skill ids are target-independent.
5. Check that an agent can decide what to read without loading every doc.
