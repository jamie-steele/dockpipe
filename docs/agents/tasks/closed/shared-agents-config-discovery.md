# TASK-005 Shared Agents Config Discovery — Closed

Completed: 2026-07-13

## Shipped

- DorkPipe now resolves reusable `agents.yml` roles from the workflow's sibling file first, then the
  nearest parent workflow directory.
- Lookup is bounded: it stops at the owning `workflows/` directory for repository workflows or the
  package directory containing `package.yml` for package workflows. It never falls back to an
  arbitrary repository-level `agents.yml`.
- Focused regression tests cover parent inheritance, sibling override, package-root discovery, and
  the blocked outside-root fallback.
- Canonical workflow and orchestration-contract docs now describe the lookup order and boundary.

## Validation

- `go test ./orchestrationhelper` from `packages/dorkpipe/lib`
- `dockpipe package test --workdir . --only dorkpipe`
- `dockpipe package compile workflows --workdir . --from packages/dorkpipe --force`
- `dockpipe package compile resolvers --workdir . --from packages/dorkpipe --force`
