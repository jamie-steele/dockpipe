# cursor-dev

## What it does

1. **Host (`run`):** Writes **`.dockpipe/cursor-dev/README.txt`** with notes about how Dockpipe mounts **`/work`**.
2. **Container (`isolate`):** Runs a tiny **base-dev** command to prove **`/work`** is your project (disposable environment).
3. **Host (second step):** Prints the folder to open and **optionally launches Cursor** on the host (same configuration style as the **vscode** template: **`vars`**, env, **`CURSOR_DEV_*`**).

There is **no** supported headless “Cursor server” in this template. Cursor is a **desktop app** (Anysphere). For a **browser-based** editor (code-server), use **`dockpipe --workflow vscode`** instead.

## Configuration

Use **`vars`** on the **`host-next-steps`** step in **`config.yml`** (or **`dockpipe.yml`**), or set **`CURSOR_DEV_*`** via shell env / **`.env`** (omit a key in YAML to use the environment). One-off overrides: **`--var KEY=value`**.

| Variable | Default | Meaning |
|----------|---------|---------|
| **`CURSOR_DEV_LAUNCH`** | **`cli`** | **`cli`** — try **`cursor`** in `PATH`, then common install paths (Windows **`Cursor.exe`**, macOS **`open -a Cursor`** or app bundle `cursor` binary). **`none`** — only print instructions. |
| **`CURSOR_DEV_WAIT`** | **`0`** | **`1`** — after starting the launcher, **`wait`** on its PID (best-effort; Cursor may detach to an already-running app). |
| **`CURSOR_DEV_CMD`** | *(unset)* | Force a specific **`cursor`** or **`Cursor.exe`** path if auto-detection fails. |

## How to run

```bash
dockpipe --workflow cursor-dev
```

Use **`--workdir`** if you are not already in the project root.

## Why someone would use it

- A **grounded** example of **run → isolate → host follow-up** without inventing vendor integrations.
- Optional starting point if you later add your own devcontainer or SSH docs.

## Experimental / caveats

- **Not** affiliated with Cursor or Anysphere.
- Does **not** configure Remote SSH, Dev Containers, or WSL automatically.
- The isolate step is intentionally minimal (smoke test only).
- Launcher detection is best-effort; if nothing matches, you still get printed **File → Open Folder** instructions.

## What persists

- Files under **`.dockpipe/cursor-dev/`** created by the prep script.
- Container is ephemeral; your source tree is unchanged except for those files.
