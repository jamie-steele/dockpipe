# cursor-dev

## What it does

1. **Host (`run`):** Writes **`.dockpipe/cursor-dev/README.txt`** with non-claims about how Dockpipe mounts **`/work`**.
2. **Container (`isolate`):** Runs a tiny **base-dev** command to prove **`/work`** is your project (disposable environment).
3. **Host (second step):** Prints a reminder to open **Cursor** on the machine and **File → Open Folder** to this repo.

There is **no** supported headless “Cursor server” in this template. Cursor is a **desktop app** (Anysphere); remote workflows (SSH, Dev Containers, etc.) are **out of scope** here unless you configure them yourself.

## Why someone would use it

- A **grounded** example of **run → isolate → host follow-up** without inventing vendor integrations.
- Optional starting point if you later add your own devcontainer or SSH docs.

## How to run

```bash
dockpipe --workflow cursor-dev
```

Use **`--workdir`** if you are not already in the project root.

## Experimental / caveats

- **Not** affiliated with Cursor or Anysphere.
- Does **not** configure Remote SSH, Dev Containers, or WSL automatically.
- The isolate step is intentionally minimal (smoke test only).

## What persists

- Files under **`.dockpipe/cursor-dev/`** created by the prep script.
- Container is ephemeral; your source tree is unchanged except for those files.
