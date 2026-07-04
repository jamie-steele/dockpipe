# Safety Guardrails

Read when touching secrets, generated artifacts, package promotion, destructive changes, or cloud model usage.

## Secrets

- Template files contain references only, such as `op://...`.
- Never commit plaintext secrets.
- Keep private vault templates local/gitignored.
- `op inject` output for workflow env is read into process memory.
- DockPipe does not write a second resolved template file for that merge.
- Never use shell redirects like `> -`; that creates a file named `-`.

## Filesystem Safety

- Do not revert user changes unless explicitly requested.
- Do not delete staging or generated trees unless requested.
- Do not create repo-root shadow script trees such as `scripts/dockpipe/...`.
- Do not write outside declared target/output directories in renderers.
- Avoid destructive commands. Ask before destructive cleanup.

## AI/Cloud Safety

- Track cloud token/cost policy for Codex/Claude lanes.
- Use halt markers when budget is exceeded.
- Keep approval explicit before promotion/apply/publish.
- Treat local model lanes as cheaper attempts, not automatic authority.

## Final Check

- `git status --short`
- inspect generated outputs
- report skipped checks and risks
