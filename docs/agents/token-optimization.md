# Token Optimization

Read when editing `AGENTS.md`, `docs/agents/`, skills, handoffs, or generated guidance.

## Goal

Route tasks to the smallest sufficient context set while preserving repo-specific safety rules.

## Pattern

- `AGENTS.md` is the short router.
- `docs/agents/index.yaml` is the machine-readable map.
- `docs/agents/*.md` are focused rule files.
- DorkPipe skills hold reusable assistant behavior.
- Canonical docs hold deeper reference material.

## Compression Rules

- Prefer tables and checklists.
- Keep one topic per file.
- Link instead of copying long docs.
- Keep forbidden artifacts centralized.
- Avoid generic AI-agent advice.
- Avoid target-specific skill routing.

## Review

- Can an agent decide what to read from `AGENTS.md` plus `index.yaml`?
- Are repeated rules reduced to links?
- Are hard safety rules preserved somewhere obvious?
- Are skills listed as target-independent ids?
