# DorkPipe Core Review

Use this skill before and after DockPipe engine edits.

## Read First

- `AGENTS.md`
- `docs/architecture-model.md`
- `docs/package-model.md`
- `docs/workflow-yaml.md` when workflow YAML changes
- `src/lib/infrastructure/paths.go` when script/package resolution changes

## Hard Rules

- Keep `src/lib/` and `src/cmd/` generic.
- Do not hardcode repo-root `packages/`, `workflows/`, or `.staging/` paths into engine behavior.
- Do not add Codex, Claude, DorkPipe, or maintainer workflow special cases to core.
- Prefer package YAML/assets/scripts for product-specific behavior.
- Update schema and language support when authored YAML surface changes.

## Review Pass

1. Identify whether the change is engine, workflow, package, resolver, or docs.
2. Check that any core change is a general primitive.
3. Verify path construction uses infrastructure helpers or existing generic resolution.
4. Look for stale docs, schema, CLI help, or tests.
5. Run the narrowest meaningful validation and report what was not run.
