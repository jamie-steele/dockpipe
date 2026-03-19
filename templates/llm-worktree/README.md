# LLM worktree template

**Run → Isolate → Act.** This template has:

- **config.yml** — run, isolate, act point to repo **scripts/**; optional **vars:** for workflow defaults (merged with `templates/llm-worktree/.env`, repo `.env`, `--env-file`, `$DOCKPIPE_ENV_FILE`, `--var`). Optional **`steps:`** for multi-step or parallel async groups — see **[docs/workflow-yaml.md](../../docs/workflow-yaml.md)** and [docs/cli-reference.md](../../docs/cli-reference.md).
- **isolate/** — References shared **images/** at repo root. See isolate/README.md.
- **resolvers/** — Resolver definitions (claude, codex): image, command, env.

**Use the template:** `dockpipe --workflow llm-worktree --repo <url> -- claude -p "Your prompt"`. Config sets run/isolate/act; override with `--run`, `--isolate`, `--act` as needed.

**Copy and edit:** `dockpipe template init my-ai` (or `--from llm-worktree`). Run `dockpipe --workflow my-ai --repo <url> [--resolver claude|codex] -- claude -p "task"`. Config points to scripts/; leave BRANCH empty for a new branch each run. Needs: dockpipe, Docker, ANTHROPIC_API_KEY or OPENAI_API_KEY; for private repos set GIT_PAT.
