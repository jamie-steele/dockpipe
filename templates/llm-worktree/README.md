# LLM worktree template

**Run → Isolate → Act.** This template has:

- **config.yml** — run, isolate, act point to repo **scripts/**; optional **vars:** for workflow defaults (merged with `templates/llm-worktree/.env`, repo `.env`, `--env-file`, `$DOCKPIPE_ENV_FILE`, `--var`). Optional **`steps:`** for multi-step or parallel async groups — see **[docs/workflow-yaml.md](../../docs/workflow-yaml.md)** and [docs/cli-reference.md](../../docs/cli-reference.md).
- **isolate/** — References shared **images/** at repo root. See isolate/README.md.
- **resolvers/** — Resolver definitions (claude, codex): image, command, env.

## Host `bash` and `git` (by design)

- **Bash on the host** — Dockpipe **always** requires **`bash`** on `PATH` (it runs host tooling via bash). **Git for Windows** is the usual way to get **`bash.exe`** on Windows.
- **Git on the host** — Clone, worktree, and commit-on-host flows call your normal **`git`** on the machine running dockpipe (not inside the container). Install **git** and ensure it is on `PATH` (e.g. **Git for Windows**, Linux `git`, macOS via Xcode CLT/Homebrew). Dockpipe exits early with a clear error if `git` is missing when those flows are used.
- **Auth is yours** — Dockpipe does **not** replace **Git Credential Manager**, SSH agent/keys, or HTTPS PAT setup. Whatever makes `git clone` / `git fetch` / `git push` work from a shell on **this OS** is what dockpipe relies on (e.g. GCM on Windows, `ssh-agent`, `GIT_PAT` / `GIT_ASKPASS` if you wire them in).
- **Line endings / scripts** — Bundled **`.sh`** pre-scripts are Unix-oriented (LF). They work with Git for Windows’ Bash; plain Notepad is not the target editor.

**Use the template:** `dockpipe --workflow llm-worktree --repo <url> -- claude -p "Your prompt"`. Config sets run/isolate/act; override with `--run`, `--isolate`, `--act` as needed.

**Copy and edit:** `dockpipe template init my-ai` (or `--from llm-worktree`). Run `dockpipe --workflow my-ai --repo <url> [--resolver claude|codex] -- claude -p "task"`. Config points to scripts/; leave BRANCH empty for a new branch each run. Needs: dockpipe, Docker, ANTHROPIC_API_KEY or OPENAI_API_KEY; for private repos set GIT_PAT (or rely on your normal git HTTPS/SSH auth).
