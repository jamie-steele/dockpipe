# cursor-dev

**Scripts** (**`cursor-dev-session.sh`**, **`cursor-prep.sh`**, **`cursor-dev-common.sh`**, **`cursor-print-next-steps.sh`**) live **in this directory** (same folder as **`config.yml`**). Workflows use **`run: scripts/cursor-dev/…`**; the runner resolves that to **`templates/core/resolvers/cursor-dev/…`** (see **`src/lib/dockpipe/infrastructure/paths.go`**).

## What it does

1. **Host (`run`):** **`cursor-dev-session.sh`** starts a **long-lived `dockpipe-base-dev` container** with your project at **`/work`**. The main process is **`sleep infinity`** until you stop the container. One startup line is printed to the container log so **Docker Desktop → Logs** isn’t empty.
2. **Waits on Docker:** **`docker wait`** blocks until the container exits — same *session* idea as **`vscode`**. **Ctrl+C** runs **`docker stop`** so the session ends cleanly.
3. **Cursor on the host:** Prints the repo path and **optionally launches Cursor** with that folder ( **`CURSOR_DEV_*`** — see below).

There is **no** supported headless “Cursor server” in this template. For a **browser-based** editor (code-server), use **`dockpipe --workflow vscode`** instead.

## Session lifecycle

| Action | Effect |
|--------|--------|
| **Leave dockpipe running** | Waits on the container. Your project stays mounted at **`/work`** until it stops. |
| **Close Cursor** (with **`CURSOR_DEV_WAIT=1`**) | Background watcher stops the container when Cursor exits; **`docker wait`** returns; dockpipe exits. |
| **`docker stop <name>`** | Container exits; **`docker wait`** returns; dockpipe exits. |
| **Ctrl+C** in the dockpipe terminal | Stops the container, then exits. |

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
| **`CURSOR_DEV_WAIT`** | **`1`** | **`1`** — when Cursor was launched as a tracked process, **closing Cursor stops the session container**. **`0`** / **`none`** — only wait on Docker (**`docker stop`** / Ctrl+C). Use **`0`** if you keep **multiple Cursor windows** or the wait misbehaves. |
| **`CURSOR_DEV_POLL_SEC`** | **`1`** | *(Windows quick-launcher path only)* Seconds between **`tasklist`** checks while waiting for **`Cursor.exe`** to exit. |

When **`CURSOR_DEV_WAIT`** is **`1`**, a background task **`wait`**s on the launcher, then **`docker stop`**s the session container. On **Windows**, if the launcher returns quickly while **`Cursor.exe`** is still running, the script polls **`tasklist`** until no **`Cursor.exe`** (all Cursor windows count — use **`CURSOR_DEV_WAIT=0`** if that’s wrong).

### Performance

- **Dominant cost** is **Docker** and **Cursor** — the shell overhead is small.
- **`CURSOR_DEV_WAIT=0`** skips the background watcher (no extra process, no Windows poll).

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

## What persists

- Files under **`.dockpipe/cursor-dev/`** from **`cursor-prep.sh`** (session start).
- Stopping the container does not remove your repo; only the disposable container goes away (`--rm`).
