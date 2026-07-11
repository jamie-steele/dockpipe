# Codex Workspace-Sandbox Sessions

This guide applies when Codex starts in a workspace sandbox. Treat the sandbox's effective
capabilities as the authority: a host executable may be visible but unavailable, and network or
daemon access may be denied. A host request is a reviewed escalation, not an automatic fallback.

## Session initialization

1. Work within the declared workspace and use the sandbox for source inspection, edits, formatting,
   and tests that do not need unavailable host capabilities.
2. Check a capability before planning around it. Do not assume that Docker, the Docker daemon,
   outbound network access, a user-profile tool shim, or a credential helper is usable merely
   because it appears on `PATH`.
3. If the required capability is unavailable, stop that operation and request one narrowly scoped
   host action. State the command or lifecycle operation, its purpose, and expected effect.
4. Do not bypass the sandbox, copy credentials, weaken approval settings, or retry through a
   different transport just to make the operation succeed.

## Docker and other host daemons

Docker CLI discovery inside the sandbox does not prove that its daemon socket or Windows named pipe
is reachable. When a workflow, package test, image build, or container lane needs Docker, first use
a harmless capability check such as `docker info`. If it is unavailable, request a host action to
start or use Docker Desktop; do not silently substitute an ungoverned local or cloud lane.

The host request must remain minimal. For example, request permission to run a particular
DockPipe package check after Docker Desktop is running, rather than requesting unrestricted Docker
access. A rejected or failed request leaves the work blocked; it does not authorize a sandbox
bypass.

## Git and networked operations

Sandbox sessions may intentionally have no outbound network access. Local, read-only Git inspection
such as `git status`, `git diff`, and `git log` is acceptable only when the executable is usable and
the repository policy permits it. Do not infer that `git fetch`, `pull`, `push`, `clone`, remote
credential helpers, package downloads, or provider logins will work.

For a managed DockPipe session, agents request the runtime-owned lifecycle action—checkpoint,
sync, or publish—rather than raw Git commands. The runtime performs any required host-side Git or
network work under its normal authorization and operation-result ledger. For an ordinary checkout,
request the smallest reviewed host command needed for the remote operation. Never expose, copy, or
log Git credentials.

## Recording and recovery

Record the capability result and host-request outcome in the task's normal artifact or operation
result. Report an unavailable capability as blocked, including the exact missing boundary (for
example, Docker daemon reachability or outbound network), rather than calling it a product failure.
Do not replay pending prompts or remote mutations after a disconnect; ask for a new reviewed host
request.

See [Git Runtime Sessions](git-runtime-sessions.md) for the managed-session ownership model and
[Sandbox Toolchain Determinism](../agents/tasks/sandbox-toolchain-determinism.md) for executable
discovery issues on sandboxed hosts.
