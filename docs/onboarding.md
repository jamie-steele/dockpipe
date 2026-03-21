# Onboarding

**Prerequisites:** [Docker](https://docs.docker.com/get-docker/) and **bash** on the host — see **[install.md](install.md)**.

---

## 1. First run

From any directory:

```bash
dockpipe -- pwd
```

If Docker or bash is misconfigured, run **`dockpipe doctor`** for a quick diagnostic.

See **[README § Try it](../README.md#try-it)** for copy-paste examples.

---

## 2. Core model (run → isolate → act)

| Phase | Where | What |
|--------|--------|------|
| **Run** | Host | Optional scripts *before* the container (e.g. prepare a worktree). |
| **Isolate** | Container | Your command after `--`; project at **`/work`**. Ephemeral — container is removed when the command exits. |
| **Act** | Host | Optional script *after* the container (e.g. commit, notify). |

**Named strategies** (optional): **`--strategy <name>`** or **`strategy:`** in workflow YAML load **`templates/core/strategies/<name>`** (or per-workflow overrides) and run **before** / **after** hooks around the body (e.g. **`git-worktree`**, **`git-commit`**). See **[workflow-yaml.md § Named strategies](workflow-yaml.md#named-strategies)**.

Most days: **`dockpipe -- <command>`** only (isolate).

---

## 3. Try a bundled workflow

From the **dockpipe repo root** (or any checkout with `templates/`):

```bash
dockpipe --workflow chain-test
```

Shows two steps and env handoff. No API keys.

**Simple git (optional):** **`--workflow commit-run`** runs your command in a container, then **one commit on your current branch** if anything changed — no worktrees or branch automation. See **[templates/commit-run/README.md](../templates/commit-run/README.md)**.

**Advanced git / AI:** **`--workflow run-worktree`** adds isolated branches, worktrees, and resolvers — **[templates/run-worktree/README.md](../templates/run-worktree/README.md)**.

---

## 4. Customize YAML

- **Single-file workflows:** [workflow-yaml.md](workflow-yaml.md) (`run` / `isolate` / `act`, optional `steps:`).
- **Flags:** [cli-reference.md](cli-reference.md).

---

## Terms

| Term | Meaning |
|------|---------|
| **Workflow** | Named preset: **`--workflow <name>`** loads **`templates/<name>/config.yml`**. |
| **Resolver** | Small file under **`resolvers/`** naming a tool profile (default image, scripts, env) — optional. |
| **Strategy** | Optional lifecycle wrapper: **`templates/core/strategies/<name>`** (KEY=value) — before/after host scripts. |

---

## Deeper reading

| Doc | When |
|-----|------|
| [architecture.md](architecture.md) | Internals, commit-on-host exception, data flow |
| [chaining.md](chaining.md) | Multiple `dockpipe` invocations, same workdir |
| [wsl-windows.md](wsl-windows.md) | Optional: WSL↔Windows **git bundle** handoff (advanced; most users skip) |
