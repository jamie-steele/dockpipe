# Git Runtime Auth

Read when changing provider detection, helper-container auth mounts, SSH/HTTPS support, or host-side
Git/SSH discovery for runtime-owned sessions.

## Boundary

- Keep helper containers Git-generic.
- Keep provider detection and auth-path discovery on the host side.
- Mount auth material into runtime helper containers only, never AI worker containers.
- Treat auth adapters as runtime infrastructure, not workflow/package logic.

## Day 1 Scope

- GitHub SSH
- Azure DevOps SSH

Support additional auth modes later as separate adapters instead of expanding the first helper path.

## Host Discovery

Use host-side inspection to determine provider and mount inputs before starting the helper
container:

- `git remote get-url origin`
- `git config --get core.sshCommand`
- `git config --get-regexp '^url\.'`
- `git config --get credential.helper`
- `git config --list --show-origin`
- `ssh -G <host-or-alias>` for SSH-backed remotes

## Helper Rules

- Worker editing surface stays at `/work`.
- AI workers edit files only. They do not clone, branch, checkpoint, or publish.
- Runtime helper tools may clone, fetch, checkout, checkpoint, inspect, and publish.
- Keep session metadata and audit logs under `bin/.dockpipe/sessions/...` even when the active Git
  checkout lives inside a volume workspace.

## Validation

- `git diff --check`
- `go test ./src/lib/...`
- verify provider-specific docs/schema updates when authored workflow surfaces change
