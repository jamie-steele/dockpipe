# Pipeon (local-first assistant)

**Pipeon** is a small shell layer that:

1. Builds **`.dockpipe/pipeon-context.md`** from CI signals, user insights, DorkPipe metadata, and self-analysis pointers.
2. Sends **one** chat turn to **local Ollama** with a fixed system prompt + that bundle.

**No cloud LLM required.** Default model: `llama3.2` (override with `PIPEON_OLLAMA_MODEL` or `DOCKPIPE_OLLAMA_MODEL`).

## Feature flags (release 0.6.5)

| Variable | Meaning |
|----------|---------|
| **`DOCKPIPE_PIPEON=1`** | **Required** to run `bundle` or `chat`. |
| **`DOCKPIPE_PIPEON_MIN_VERSION`** | Default **`0.6.5`**. Pipeon refuses if repo `VERSION` is lower, unless prerelease override is set. |
| **`DOCKPIPE_PIPEON_ALLOW_PRERELEASE=1`** | Allow Pipeon **before** `MIN_VERSION` (developers / CI). |

Until **0.6.5** ships, use **`DOCKPIPE_PIPEON=1`** and **`DOCKPIPE_PIPEON_ALLOW_PRERELEASE=1`** to try.

## Commands

```bash
export DOCKPIPE_PIPEON=1
export DOCKPIPE_PIPEON_ALLOW_PRERELEASE=1   # until VERSION >= 0.6.5
./bin/pipeon status
./bin/pipeon bundle
./bin/pipeon chat "Summarize security posture from available signals."
```

## Shortcuts

See **`docs/pipeon-shortcuts.md`** (VS Code/Cursor tasks, shell aliases, optional desktop).

## Desktop / code-server (this repo)

**Bundled Pipeon shell** (**`pipeon.sh`**, **`chat.sh`**, **`bundle-context.sh`**, **`lib/`**, **`prompts/`**) is canonical under **`templates/core/bundles/pipeon/assets/scripts/`**; this directory repeats those paths as **symlinks** for stable **`scripts/pipeon/…`** references (same idea as **`scripts/dorkpipe/`**).

Maintainer-only scripts live **here** (same folder as the harness):

| Script | Purpose |
|--------|---------|
| **`generate-pipeon-icons.py`** | **`make pipeon-icons`** — extension PNG, code-server ICO/SVG favicons |
| **`install-pipeon-desktop-shortcut.sh`** | Linux Freedesktop menu entry |
| **`install-pipeon-desktop-shortcut.ps1`** | Windows `.lnk` shortcuts |
| **`install-pipeon-shortcut-macos.sh`** | macOS `~/Applications/Pipeon.command` |
| **`pipeon-code-server-launch.sh`** / **`.ps1`** | Target for shortcuts; runs **`dockpipe --workflow vscode`** |

Prefer **`make install-pipeon-shortcut`** from repo root.

## Docs

- **`docs/pipeon-ide-experience.md`** — UX and tone
- **`docs/pipeon-shortcuts.md`** — keyboard and task shortcuts
