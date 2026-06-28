# vscode (desktop app compatibility + dev container)

**Model:** **`vscode`** is a **resolver** bundle (**profile** + **`config.yml`**) that sets **`DOCKPIPE_RESOLVER_WORKFLOW=vscode`** — this folder holds the **delegate YAML** the runner loads (also addressable via **`--workflow vscode`** as a convenience to the same file).

**Scripts:** **`vscode-session.sh`**, **`session-idle.sh`**, and **`vscode-common.sh`** live in this directory. Workflows use **`run: scripts/vscode/vscode-session.sh`**.

## What it does

Starts a long-lived **`dockpipe-base-dev`** session container with your project at **`/work`**, then launches the **locally installed Visual Studio Code desktop app** on the host attached to that container using a **`vscode-remote://dev-container+…/work`** URI.

This means:

- `vscode` = your normal local Visual Studio Code / VS Code install, attached to a session container
- `Pipeon` = the branded browser/editor stack

The workflow does not expose a separate standalone browser `code-server` app. It uses desktop VS Code plus a session container and the Dev Containers / remote flow.

## Why use it

- Opens stock VS Code instead of the Pipeon-branded browser app.
- Reuses your host extensions/settings while keeping repo execution inside a container.
- Matches the “Open a Remote Window” flow already present in desktop VS Code.

## How to run

From your project (or set `--workdir`):

```bash
dockpipe --workflow vscode
```

**Configuration** — use **`vars`** in the workflow YAML or set **`VSCODE_*`** in the shell / repo `.env`.

One-off CLI overrides: **`--var KEY=value`** (locks the key for that run).

- **`VSCODE_CMD`** — explicit path or command name if `code` is not on `PATH`.
- **`VSCODE_WAIT=1`** (default) — keep DockPipe attached until you quit VS Code or the remote session ends.
- **`VSCODE_SESSION_SHUTDOWN=both`** (default) — stop on host quit or remote idle. Set `host` for host-only shutdown.
- **`VSCODE_SESSION_IMAGE`** — override the base-dev image.

## Caveats

- This compatibility resolver is DockPipe-authored and is not affiliated with, sponsored by, or endorsed by Microsoft.
- DockPipe does not ship Visual Studio Code, VS Code logos, Microsoft services, extension marketplace content, credentials, or editor auth state in this package.
- This workflow expects desktop VS Code plus the Dev Containers / remote support needed to open a dev-container URI.
- This uses your normal desktop VS Code profile, so your existing extensions/theme/profile will appear.
- For the browser/server flow, use Pipeon instead.
