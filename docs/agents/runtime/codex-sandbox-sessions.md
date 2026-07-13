# Codex Sandbox Session Routing

Read [the canonical runtime guide](../../runtime/codex-sandbox-sessions.md) before starting a
Codex workspace-sandbox session that needs Docker, remote Git, credentials, downloads, or another
host capability.

- Treat effective sandbox capability as authoritative; `PATH` visibility is not capability proof.
- Use the sandbox for workspace-scoped inspection, edits, and available local checks.
- When Docker daemon access, networked Git, or another unavailable host capability is required,
  stop and request one reviewed, narrow host operation.
- For managed sessions, request runtime-owned checkpoint, sync, or publish; never issue raw Git
  lifecycle commands from the agent.
- Never bypass sandboxing, copy/log credentials, silently fall back to another lane, or replay an
  interrupted remote mutation.
