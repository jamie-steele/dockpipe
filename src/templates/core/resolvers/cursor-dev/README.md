# cursor-dev

**Scripts** (**`cursor-dev-session.sh`**, **`cursor-prep.sh`**, **`cursor-dev-common.sh`**, **`cursor-print-next-steps.sh`**) live **in this directory** (same folder as **`config.yml`**). Workflows use **`run: scripts/cursor-dev/…`**; the runner resolves that to **`templates/core/resolvers/cursor-dev/…`** (see **`src/lib/dockpipe/infrastructure/paths.go`**).

## Pipeon Launcher

**Pipeon’s “Set up Cursor MCP”** button runs **`cursor-prep.sh` only** (writes **`.dockpipe/cursor-dev/`**). It does **not** start Docker or run DockPipe. **Double-click the `cursor-dev` app** in Basic mode to run **`dockpipe --workflow cursor-dev`** (full session: container + Cursor on the host).

## What it does

1. **Host (`run`):** **`cursor-dev-session.sh`** starts a **long-lived `dockpipe-base-dev` container** with your project at **`/work`**. The main process is **`sleep infinity`** until you stop the container. One startup line is printed to the container log so **Docker Desktop → Logs** isn’t empty.
2. **Waits on Docker:** **`docker wait`** blocks until the container exits — same *session* idea as **`vscode`**. **Ctrl+C** runs **`docker stop`** so the session ends cleanly.
3. **Cursor on the host:** Prints the repo path and **optionally launches Cursor** with that folder ( **`CURSOR_DEV_*`** — see below).

There is **no** supported headless “Cursor server” in this template. For a **browser-based** editor (code-server), use **`dockpipe --workflow vscode`** instead.

**Host cleanup (core):** After **`docker run`**, the script writes the container name to **`.dockpipe/cleanup/docker-session`** (one line) for **`ApplyHostCleanup`** in **`RunHostScript`** (see **`docs/workflow-yaml.md`** — **Host skip_container lifecycle**). It also writes **`.dockpipe/cursor-dev/session_container`** for older docs/tools. If **`DOCKPIPE_RUN_ID`** is set, **`.dockpipe/runs/<id>.container`** is written for **`dockpipe runs list`**. The session script registers **`trap … EXIT`** so **`set -e`** failures after **`docker run`** still run **`docker stop`**. When the host script exits, the Go runner applies **host cleanup** if markers remain (e.g. **`kill -9`** on bash).

**GUI hint:** Set **`DOCKPIPE_LAUNCH_MODE=gui`** in **`vars`** so the script prints that this flow opens the **desktop app** (not a remote Cursor server); dockpipe still **waits on this host script** until **`docker wait`** returns or you interrupt **`dockpipe`**.

## Session lifecycle

| Action | Effect |
|--------|--------|
| **Leave dockpipe running** | Waits on the container. Your project stays mounted at **`/work`** until it stops. |
| **Close Cursor** (only with **`CURSOR_DEV_WAIT=1`**) | Background watcher stops the container when the tracked launcher process exits; **`docker wait`** returns; dockpipe exits. |
| **`docker stop <name>`** | Container exits; **`docker wait`** returns; dockpipe exits. |
| **Ctrl+C** / **`kill` (SIGTERM)** / **SIGQUIT** to **`dockpipe`** | **`RunHostScript`** forwards these to the bash child (Unix/macOS; Windows forwards **Ctrl+C**). Host **`trap`** runs **`docker stop`**. |
| **Parent `dockpipe` exits** (Linux) | **`PR_SET_PDEATHSIG`**: bash gets **SIGTERM** when the parent dies — including many **`kill -9`** cases on the parent — so **`trap`** can still run. |
| **`kill -9` on the bash child** or **killing only `docker`** | **`trap`** may not run — run **`docker stop`** with the printed container name. |

The container name is printed as **`dockpipe-cursor-dev-…`** unless you set **`CURSOR_DEV_CONTAINER_NAME`**.

## Configuration

Use **`vars`** in **`config.yml`** (or **`dockpipe.yml`**), shell env, or **`.env`**. One-off: **`--var KEY=value`**.

| Variable | Default | Meaning |
|----------|---------|---------|
| **`CURSOR_DEV_LAUNCH`** | **`cli`** | **`cli`** — try **`cursor`** in `PATH`, then common install paths. **`none`** — only print instructions (container still runs until stopped). |
| **`CURSOR_DEV_SESSION_IMAGE`** | **`dockpipe-base-dev:latest`** | Image for the session container. Must exist (run any dockpipe **`--isolate base-dev`** once, or let the script build from **`DOCKPIPE_REPO_ROOT`**). |
| **`CURSOR_DEV_CONTAINER_NAME`** | *(auto)* | Optional fixed name for **`docker stop`**. |
| **`CURSOR_DEV_CMD`** | *(unset)* | Force a specific **`cursor`** or **`Cursor.exe`** path. |
| **`CURSOR_DEV_SKIP_DOCKER_CHECK`** | **`0`** | Set **`1`** only if you customize the workflow and skip the daemon check. |
| **`CURSOR_DEV_WAIT`** | **`0`** | **`0`** / **`none`** (default) — session stays up until **`docker stop`** or **Ctrl+C** in the terminal (matches Linux/macOS where the **`cursor`** CLI often exits immediately after spawning the GUI). **`1`** — background watcher **`wait`**s on the launcher PID, then **`docker stop`**s the session (useful on **Windows** when you want closing Cursor to end the session). |
| **`CURSOR_DEV_POLL_SEC`** | **`1`** | *(Windows, only when **`CURSOR_DEV_WAIT=1`**)* Seconds between **`tasklist`** checks if the launcher exited quickly while **`Cursor.exe`** is still running. |

When **`CURSOR_DEV_WAIT`** is **`1`**, a background task **`wait`**s on the launcher, then **`docker stop`**s the session container. On **Windows**, if the launcher returns quickly while **`Cursor.exe`** is still running, the script polls **`tasklist`** until no **`Cursor.exe`**. If that behavior is wrong for you, keep the default **`0`**.

### Performance

- **Dominant cost** is **Docker** and **Cursor** — the shell overhead is small.
- Default **`CURSOR_DEV_WAIT=0`** skips the background watcher (no extra process, no Windows **`tasklist`** poll). Set **`1`** only when you want that coupling.

## How to run

```bash
dockpipe --workflow cursor-dev
```

Use **`--workdir`** if you are not already in the project root.

## Experimental / caveats

- **Not** affiliated with Cursor or Anysphere.
- Does **not** configure Remote SSH, Dev Containers, or WSL automatically.
- Launcher detection is best-effort; if nothing matches, use **File → Open Folder** with the printed path.
- **`dockpipe-base-dev`** must be available locally (bundled first run, or build via **`DOCKPIPE_REPO_ROOT`**).
- **Git Bash on Windows:** MSYS can rewrite **`/work`** in **`docker run`**. The session sets **`MSYS2_ARG_CONV_EXCL=*`** for **`docker`** so container paths stay **`/work`**. Launches use **`cygpath -w`** for the folder when available.

## AI agent + MCP (“basic mode” for Cursor and any agent UI)

**`cursor-prep.sh`** runs at the start of **`cursor-dev-session.sh`** and **`cursor-print-next-steps.sh`**. It writes **`.dockpipe/cursor-dev/`**:

| File | Purpose |
|------|---------|
| **`README.txt`** | Short pointer to the folder and **AGENT-MCP.md**. |
| **`AGENT-MCP.md`** | **Repo root**, **`DOCKPIPE_REPO_ROOT`**, how **MCP / `mcpd`** connect to Cursor, links to **`docs/mcp-agent-trust.md`**, **`docs/mcp-host-hardening.md`**, **`AGENTS.md`**. |
| **`mcp.json.example`** | Drop-in MCP server block with **absolute** paths to **`src/bin/mcpd`**, **`dockpipe`**, **`dorkpipe`** when this tree is the **dockpipe** checkout (or a generic template otherwise). |

After prep, the session prints the path to **`AGENT-MCP.md`** — open it in Cursor (or `@` it) so the agent knows repo layout and MCP setup for demos.

For the **dockpipe** repository itself, prefer the committed **`.cursor/mcp.json`** at the repo root (same idea as **`mcp.json.example`**). Run **`make build`** before enabling MCP.

## What persists

- Files under **`.dockpipe/cursor-dev/`** from **`cursor-prep.sh`** (session start or print-next-steps).
- Stopping the container does not remove your repo; only the disposable container goes away (`--rm`).
