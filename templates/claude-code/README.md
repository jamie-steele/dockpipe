# claude-code

## What it does

1. **Host (`run`):** Writes **`.dockpipe/claude-code/README.txt`** with notes about using **Claude Code** on the host and **`/work`** in containers.
2. **Container (`isolate`):** Runs a tiny **base-dev** smoke test so **`/work`** is your project (prints a **count** of top-level entries, not filenames).
3. **Host (second step):** Prints install/run instructions and **optionally runs `claude`** from your project directory (same **vars / env** pattern as **cursor-dev** / **vscode** templates).

This template does **not** install **@anthropic-ai/claude-code** for you. It does **not** configure API keys — use Anthropic’s docs for **`ANTHROPIC_API_KEY`** / credentials.

## Configuration

Use **`vars`** on **`host-next-steps`** in **`config.yml`** (or **`dockpipe.yml`**), or **`CLAUDE_CODE_*`** via shell / **`.env`**. **`--var KEY=value`** for one-off overrides.

| Variable | Default | Meaning |
|----------|---------|---------|
| **`CLAUDE_CODE_LAUNCH`** | **`cli`** | **`cli`** — try **`claude`** on `PATH`, then common **npm global** paths (including Windows **`%AppData%\npm\claude.cmd`**). **`none`** — instructions only. |
| **`CLAUDE_CODE_WAIT`** | **`0`** | **`1`** — **`wait`** on the background **`claude`** PID (best-effort). |
| **`CLAUDE_CODE_CMD`** | *(unset)* | Explicit path to the **`claude`** binary if detection fails. |

## How to run

```bash
dockpipe --workflow claude-code
```

Use **`--workdir`** if you are not already in the project root.

## Related workflows

| Goal | Command |
|------|---------|
| **Claude Code in Docker** (worktree / repo flows) | **`dockpipe --workflow llm-worktree --resolver claude --repo <url> -- claude -p "…"`** |
| **Browser editor** (code-server) | **`dockpipe --workflow vscode`** |
| **Cursor desktop** | **`dockpipe --workflow cursor-dev`** |

## Caveats

- **Not** affiliated with Anthropic.
- Host **`claude`** launch is best-effort; TTY/interactive behavior may differ when backgrounded.
- For permission / sandbox flags inside containers, see **`docs/cli-reference.md`** (Claude Code, **`IS_SANDBOX`**, etc.).

## What persists

- Files under **`.dockpipe/claude-code/`** from the prep step.
- The isolate container is ephemeral; your tree only gains those files.
