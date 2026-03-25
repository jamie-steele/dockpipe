# Pipeon — shortcuts (easy adoption)

Pipeon is **flag-gated** (`DOCKPIPE_PIPEON=1`); see **`src/apps/pipeon/scripts/README.md`** (or **`scripts/pipeon/README.md`** — symlink). Until release **0.6.5**, also set **`DOCKPIPE_PIPEON_ALLOW_PRERELEASE=1`**.

---

## 1. Shell alias (fastest)

Add to `~/.bashrc` or `~/.zshrc`:

```bash
# Pipeon — adjust path to your clone
export PIPEON_ROOT="$HOME/source/dockpipe"
pipeon() {
  DOCKPIPE_PIPEON=1 DOCKPIPE_PIPEON_ALLOW_PRERELEASE=1 \
    DOCKPIPE_WORKDIR="${DOCKPIPE_WORKDIR:-$(pwd)}" \
    "$PIPEON_ROOT/bin/pipeon" "$@"
}
```

Then from any repo:

```bash
pipeon status
pipeon bundle
pipeon chat "What CI signals do we have?"
```

---

## 2. VS Code / Cursor (workspace tasks)

This repo ships **`.vscode/tasks.json`**:

- **Pipeon: status** — check flags and artifacts
- **Pipeon: bundle context** — regenerate `.dockpipe/pipeon-context.md`
- **Pipeon: chat** — ask a question (prompt input)

Run via **Terminal → Run Task…** or Command Palette → **Tasks: Run Task**.

**Keyboard (optional):** VS Code does not load team keybindings from the repo. Merge **`src/apps/pipeon/docs/pipeon-vscode-keybindings.json.example`** into your **User** keybindings (Command Palette → **Preferences: Open Keyboard Shortcuts (JSON)**):

| Shortcut | Action |
|----------|--------|
| `Ctrl+Alt+Shift+P` | Pipeon: chat (prompt) |
| `Ctrl+Alt+Shift+B` | Pipeon: bundle context |
| `Ctrl+Alt+Shift+S` | Pipeon: status |

On macOS, use `cmd`/`ctrl` as you prefer; edit the `key` fields if they clash.

---

## 3. Make (from repo root)

```bash
make pipeon-status
make pipeon-bundle
make pipeon-chat   # passes PROMPT=... or first make arg — see Makefile
```

---

## 4. Ollama

- Start: `ollama serve` (default `http://127.0.0.1:11434`)
- Pull model: `ollama pull llama3.2` (or set `PIPEON_OLLAMA_MODEL`)

---

## 5. Desktop launcher (optional, Linux)

You can create a `.desktop` file that runs `x-terminal-emulator -e bash -lc 'cd /path/to/repo && DOCKPIPE_PIPEON=1 ... bin/pipeon chat'` — keep it local; not shipped by default.
