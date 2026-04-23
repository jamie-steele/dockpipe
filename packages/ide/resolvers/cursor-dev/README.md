# cursor-dev

**Scripts** (**`cursor-dev-session.sh`**, **`session-idle.sh`** (container PID 1 helper), **`cursor-prep.sh`**, **`cursor-dev-common.sh`**, **`cursor-print-next-steps.sh`**) live **in this directory** (same folder as **`config.yml`**). Workflows use logical script ids like **`run: scripts/cursor-dev/…`**; DockPipe resolves those to the compiled or materialized resolver asset for this package/resolver.

## DockPipe Launcher

**DockPipe Launcher’s “Set up Cursor MCP”** button runs **`cursor-prep.sh` only** (writes **`bin/.dockpipe/packages/cursor-dev/`**). It does **not** start Docker or run DockPipe. **Double-click the `cursor-dev` app** in Basic mode to run **`dockpipe --workflow cursor-dev`** (full session: container + Cursor on the host).

## What it does

1. **Host (`run`):** **`cursor-dev-session.sh`** starts a **long-lived `dockpipe-base-dev` container** with your project at **`/work`**. **`session-idle.sh`** runs inside the container: bootstrap line, a **background monitor** that polls for remote-server processes, writes **`0`/`1`** to **`bin/.dockpipe/packages/cursor-dev/remote_active`** on the bind mount (so the **host** can read session state without **`docker exec`**), prints **remote session started / ended** lines to container stdout (visible with **`docker logs`**), then **`sleep infinity`** (main shell stays alive so the monitor keeps running). See **§ Why docker logs** and **§ No official “remote attached” API** below.
2. **Waits on Docker:** **`docker wait`** blocks until the container exits — same *session* idea as **`vscode`**. **Ctrl+C** runs **`docker stop`** so the session ends cleanly.
3. **Cursor on the host:** Waits until the session container is **running**, then **optionally launches Cursor** attached to that container with workspace **`/work`** ( **`--folder-uri`** Dev Containers style — **`CURSOR_DEV_REMOTE_URI`**, default on) or opens the host folder if disabled.

There is **no** supported headless “Cursor server” in this template. For a **browser-based** editor, use **Pipeon** instead.

### Why `docker logs` — what you should see

**Cursor / Dev Containers** still log **detail** under **`.cursor-server/`** on the repo. **`session-idle.sh`** adds **high-level** lines you can rely on for “is anything attached?”:

- Bootstrap (one line).
- **`[dockpipe] cursor-dev: remote session started`** when the monitor first sees remote-server processes.
- **`[dockpipe] cursor-dev: remote session ended`** when those processes go away.

**Reliable shutdown on the host** combines the marker with the same *ideas* as **`vscode-code-server.sh`** (counting live sessions): **(1)** **ESTABLISHED TCP** on the host that references the **container’s IP** (host ↔ remote traffic), **(2)** **recent writes** under **`.cursor-server/`** inside the package-scoped home (like “something is still talking”), **(3)** **`pgrep` / `docker exec`** if needed. **`bin/.dockpipe/packages/cursor-dev/remote_active`** is **`0`** or **`1`**, rewritten every **`CURSOR_DEV_SESSION_POLL_SEC`** (default **2**). If the marker is **fresh** (mtime within **`CURSOR_DEV_MARKER_MAX_AGE_SEC`**, default **60s**) and **`1`**, the host trusts it immediately; if it is **`0`**, the host **still** checks TCP / **`.cursor-server`** / **`docker exec`** so a missed **`pgrep`** in the monitor does not block detection.

**Where to look for deep logs:** **`/work/bin/.dockpipe/packages/cursor-dev/home/.cursor-server/`** in the container = **`bin/.dockpipe/packages/cursor-dev/home/.cursor-server/`** on the host.

To inspect manually: **`docker exec -it <container> ps aux`** or files under **`<repo>/.cursor-server/`**.

### No official “remote attached” API (Cursor / VS Code)

**Cursor and Microsoft do not publish** a supported contract for a **host shell** or **separate process** to ask “is the Dev Containers / remote window still attached?” The documented surface is **inside the editor**: e.g. **`vscode.env.remoteName`** (VS Code Extension API) — not something **`dockpipe`** can call from bash.

So **`cursor-dev`** layers **best-effort heuristics**: **TCP to the container IP**, **localhost TCP** from Cursor, **`.cursor-server/`** file activity in the package-scoped home, bind-mounted **`remote_active`**, then **`pgrep` / full `ps` / `/proc/…/cmdline`** / **`docker exec`**. **Default** **`CURSOR_DEV_SESSION_SHUTDOWN=both`** stops the session when **you quit Cursor on the host** *or* when the **in-container remote** session goes idle (e.g. you closed only the remote window). Set **`CURSOR_DEV_SESSION_SHUTDOWN=host`** if remote detection misbehaves and you only care about **quit Cursor entirely**.

**Host cleanup (core):** After **`docker run`**, the script writes the container name to **`bin/.dockpipe/cleanup/docker-session`** (one line) for **`ApplyHostCleanup`** in **`RunHostScript`** (see **`docs/workflow-yaml.md`** — **Host `kind: host` lifecycle**). It also writes **`bin/.dockpipe/packages/cursor-dev/session_container`**. If **`DOCKPIPE_RUN_ID`** is set, **`bin/.dockpipe/runs/<id>.container`** is written for **`dockpipe runs list`**. The session script registers **`trap … EXIT`** so **`set -e`** failures after **`docker run`** still run **`docker stop`**. When the host script exits, the Go runner applies **host cleanup** if markers remain (e.g. **`kill -9`** on bash).

**GUI hint:** Set **`DOCKPIPE_LAUNCH_MODE=gui`** in **`vars`** so the script prints that this flow opens the **desktop app** (not a remote Cursor server); dockpipe still **waits on this host script** until **`docker wait`** returns or you interrupt **`dockpipe`**.

## Session lifecycle

| Action | Effect |
|--------|--------|
| **Leave dockpipe running** | Waits on the container. Your project stays mounted at **`/work`** until it stops. |
| **Close only the remote / dev-container window** (leave Cursor running) | With **`CURSOR_DEV_SESSION_SHUTDOWN=both`** (default) and **`CURSOR_DEV_REMOTE_URI=1`**, dockpipe tries to stop the container when the **remote** session ends (heuristic). With **`host`**, this **does not** stop the session — use **`both`** for this case. |
| **Quit Cursor entirely** (all windows) | With **`CURSOR_DEV_WAIT=1`**, the watcher stops the container when the **host** Cursor process is gone (**`both`** and **`host`** both cover this). |
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
| **`CURSOR_DEV_SESSION_IMAGE`** | **`dockpipe-base-dev:latest`** | Image for the session container. Must exist locally; set this to another image tag if you do not use `dockpipe-base-dev`. |
| **`CURSOR_DEV_REMOTE_URI`** | **`1`** | **`1`** (default) — after the session container is running, launch Cursor with **`--folder-uri vscode-remote://dev-container+…/work`** so it attaches and opens **`/work`** (avoids manually using the bottom-left remote menu). **`0`** — open the host repo folder only (previous behavior). |
| **`CURSOR_DEV_SESSION_SHUTDOWN`** | **`both`** | **`both`** (default) — stop when **Cursor on the host** exits **or** the **in-container remote** session goes idle. Runs **`cursor_dev_wait_dual_session_end`** for the session container (**does not** require **`CURSOR_DEV_FOLDER_URI`** — attach can be manual or URI build can fail). **`host`** — host exit only. |
| **`CURSOR_DEV_REMOTE_SERVER_IDLE_POLLS`** | **`3`** | With **`CURSOR_DEV_SESSION_SHUTDOWN=both`**, require this many consecutive polls with no remote “up” signal after one was seen, before treating the remote session as ended. |
| **`CURSOR_DEV_SESSION_POLL_SEC`** | **`2`** | **Inside the container** (`session-idle.sh`): seconds between polls for remote-server PIDs; also how often **`remote_active`** is rewritten. |
| **`CURSOR_DEV_SESSION_LOG_HEARTBEAT_SEC`** | **`0`** | If **> 0**, print a **`monitor heartbeat`** line to **`docker logs`** every N seconds (**`active`**, **`proc`**, **`seen_proc_ever`**) so you can tell the monitor is alive. |
| **`CURSOR_DEV_CONTAINER_MONITOR`** | **`1`** | **`1`** — run the in-container monitor (extra **`docker logs`** lines + **`remote_active`**). **`0`** — bootstrap and **`sleep`** only (no marker file updates). |
| **`CURSOR_DEV_MARKER_MAX_AGE_SEC`** | **`60`** | On the **host**, trust **`bin/.dockpipe/packages/cursor-dev/remote_active`** only if its mtime is newer than this many seconds; otherwise fall back to **`docker exec`** (stale = monitor dead or old session). |
| **`CURSOR_DEV_REMOTE_TCP_SIGNAL`** | **`1`** | Host: **ESTABLISHED** TCP involving the **container bridge IP** (`docker inspect` …). **`0`** disables. |
| **`CURSOR_DEV_REMOTE_HOST_LOCALHOST_TCP`** | **`1`** | Host: **ESTABLISHED** TCP from a **Cursor** process (`pgrep` …) to **`127.0.0.1`** / **`::1`** (Dev Containers port-forward path — same *idea* as **`vscode`**’s **`127.0.0.1:PORT`**). Set **`0`** if this stays true after you disconnect (e.g. other localhost connections) and the container will not stop. |
| **`CURSOR_DEV_REMOTE_FS_SIGNAL`** | **`1`** | Host + container: treat as **up** if **any** file under **`.cursor-server/`** was modified within **`CURSOR_DEV_REMOTE_FS_QUIET_SEC`**. **`0`** disables. |
| **`CURSOR_DEV_REMOTE_FS_QUIET_SEC`** | **`90`** | If nothing under **`.cursor-server/`** is newer than this many seconds, the FS signal is **off** (disconnect-ish). Increase (e.g. **180**) if long idle edits without disk writes stop the session too early. |
| **`CURSOR_DEV_WAIT_DEBUG`** | **`0`** | Set **`1`** to stderr-log **`host_seen`**, **`remote_running`**, **`empty_streak`** each poll (dual shutdown mode). |
| **`CURSOR_DEV_DOCKER_WAIT_SEC`** | **`120`** | Poll this many seconds for **`docker info`** before failing (useful if Docker Desktop is still starting). **`0`** — fail immediately if the daemon is down. |
| **`CURSOR_DEV_CONTAINER_NAME`** | *(auto)* | Optional fixed name for **`docker stop`**. |
| **`CURSOR_DEV_CMD`** | *(unset)* | Force a specific **`cursor`** or **`Cursor.exe`** path. |
| **`CURSOR_DEV_SKIP_DOCKER_CHECK`** | **`0`** | Set **`1`** only if you customize the workflow and skip the daemon check. |
| **`CURSOR_DEV_WAIT`** | **`1`** | **`1`** (default) — when you **quit Cursor**, a background watcher stops the session container (polls for the GUI; works on Linux/macOS/Windows). **`0`** / **`none`** — session stays up until **`docker stop`** or **Ctrl+C** in the terminal (useful if you close the IDE window but want the container to keep running). |
| **`CURSOR_DEV_POLL_SEC`** | **`1`** | Seconds between **`tasklist`** / **`pgrep`** / **`ps`** checks while waiting for Cursor to exit. Use **`0.5`** for faster detection after **abrupt** app kill (dual mode + host). |
| **`CURSOR_DEV_GUI_APPEAR_SEC`** | **`90`** | If the Cursor GUI never appears within this many seconds (after launch), the watcher **does not** stop the container — leave **`CURSOR_DEV_WAIT=0`** or fix **`CURSOR_DEV_CMD`** if that happens. |

When **`CURSOR_DEV_WAIT`** is **`1`**, the script records the **`cursor`** binary path, optionally **`wait`**s on the launcher shell, then **`cursor_dev_wait_for_cursor_gui_exit`** waits until **Cursor.exe** / **Cursor.app** / the **`cursor`** process is gone, then **`docker stop`**s the session ( **`docker wait`** in the main script then returns).

### Performance

- **Dominant cost** is **Docker** and **Cursor** — the shell overhead is small.
- Default **`CURSOR_DEV_WAIT=1`** runs a small background poll loop until **`docker stop`** fires after you close Cursor. Set **`0`** if you prefer a long-lived session container without that coupling.

## How to run

```bash
dockpipe --workflow cursor-dev
```

Use **`--workdir`** if you are not already in the project root.

## Experimental / caveats

- **Not** affiliated with Cursor or Anysphere.
- Does **not** configure Remote SSH or WSL automatically. It **can** launch Cursor already attached to the session container (Dev Containers URI — **`CURSOR_DEV_REMOTE_URI`**).
- Launcher detection is best-effort; if nothing matches, use **File → Open Folder** with the printed path.
- **`dockpipe-base-dev`** must be available locally unless you override **`CURSOR_DEV_SESSION_IMAGE`**. Rebuild the image after upgrades (**`docker rmi dockpipe-base-dev:latest`** then run **`cursor-dev`** or **`dockpipe --isolate base-dev -- echo ok`**) so the container has a valid **`HOME`** under **`bin/.dockpipe/packages/cursor-dev/home`** (Cursor/VS Code remote server installs to **`$HOME/.cursor-server`**; without **`HOME`**, **`docker run -u uid:gid`** can yield permission errors) and the GNU **`base64`** shim (install scripts may call **`base64 -D`**).
- **Git Bash on Windows:** MSYS can rewrite **`/work`** in **`docker run`**. The session sets **`MSYS2_ARG_CONV_EXCL=*`** for **`docker`** so container paths stay **`/work`**. Launches use **`cygpath -w`** for the folder when available.

## AI agent + MCP (“basic mode” for Cursor and any agent UI)

**`cursor-prep.sh`** runs at the start of **`cursor-dev-session.sh`** and **`cursor-print-next-steps.sh`**. It writes **`bin/.dockpipe/packages/cursor-dev/`**:

| File | Purpose |
|------|---------|
| **`README.txt`** | Short pointer to the folder and **AGENT-MCP.md**. |
| **`AGENT-MCP.md`** | Project root, **`DOCKPIPE_WORKDIR`**, how **MCP / `mcpd`** connect to Cursor, links to **`docs/mcp-agent-trust.md`**, **`docs/mcp-host-hardening.md`**, **`AGENTS.md`**. |
| **`mcp.json.example`** | Drop-in MCP server block with **absolute** paths to **`src/bin/mcpd`**, **`dockpipe`**, **`dorkpipe`** when this tree is the **dockpipe** checkout (or a generic template otherwise). |

After prep, the session prints the path to **`AGENT-MCP.md`** — open it in Cursor (or `@` it) so the agent knows repo layout and MCP setup for demos.

For the **dockpipe** repository itself, prefer the committed **`.cursor/mcp.json`** at the repo root (same idea as **`mcp.json.example`**). Run **`make build`** before enabling MCP.

## What persists

- Files under **`bin/.dockpipe/packages/cursor-dev/`** from **`cursor-prep.sh`** (session start or print-next-steps).
- Stopping the container does not remove your repo; only the disposable container goes away (`--rm`).
