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

## Canonical Rule

- Prefer improving the canonical main docs over creating normalized duplicate docs.
- Keep `docs/agents/` as a compressed routing/policy layer, not a second full documentation tree.
- Only add agent-only normalized text when the content is truly routing, safety, or maintenance
  policy that humans do not need in the main reference docs.

## Compression Rules

- Prefer tables and checklists.
- Keep one topic per file.
- Link instead of copying long docs.
- Keep forbidden artifacts centralized.
- Avoid generic AI-agent advice.
- Avoid target-specific skill routing.
- Avoid shadow copies of `docs/*.md` under `docs/agents/`.

## Review

- Can an agent decide what to read from `AGENTS.md` plus `index.yaml`?
- Are repeated rules reduced to links?
- Are hard safety rules preserved somewhere obvious?
- Are skills listed as target-independent ids?
