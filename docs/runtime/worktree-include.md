# Worktree include files (local / gitignored config)

When dockpipe creates a **git worktree** from your checkout (`clone-worktree.sh` with `DOCKPIPE_USER_REPO_ROOT`), the new tree starts from **HEAD**. Files that are **only on disk** and **gitignored** (e.g. `.env`, `appsettings.Development.json`) are **not** part of that checkout.

Dockpipe can copy extra paths from your **main** working tree into the **worktree** using an include list (same *idea* as some other tools’ `.worktreeinclude`).

## Which file is used

1. **`.dockpipe-worktreeinclude`** in the **repository root** — preferred (dockpipe-owned name).
2. Otherwise **`.worktreeinclude`** — compatibility alias (same format).

If **both** exist, only **`.dockpipe-worktreeinclude`** is read.

## Format version

Optional **first line**:

```text
# dockpipe-worktreeinclude-format: 1
```

- If this line is present and the number is **not `1`**, dockpipe **skips** the entire file and prints a warning (so a future or foreign format does not apply half-broken rules).
- If the first line is **not** a format line, the file is treated as **v1** (matches typical `.worktreeinclude` usage without a header).

## Patterns (v1)

- One **glob pattern per line**, relative to the repo root (no leading `/`).
- Lines starting with `#` are comments; blank lines are ignored.
- Patterns follow bash pathname rules; use **bash 4+** with `globstar` for `**` (Git Bash on Windows is fine).
- Avoid **spaces** inside a pattern line (one glob per line).
- After uncommitted carry (`git diff` + untracked copy), include patterns are applied **last**, so they can overlay matching paths in the worktree.

## Examples

`.dockpipe-worktreeinclude`:

```text
# dockpipe-worktreeinclude-format: 1
.env
.env.local
**/appsettings.Development.json
```

## Stash mode

If you set **`DOCKPIPE_STASH_UNCOMMITTED=1`**, git stash still does not reliably include **ignored** files unless you use stash `-a`. Include files are still copied **after** the worktree exists so local secrets can be present in the worktree.
