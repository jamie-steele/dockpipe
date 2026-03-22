# run-apply

**Purpose:** A **two-step** pipeline **run → apply** (plan/apply style without a separate validate step). Each step runs in a disposable **alpine** container by default — replace **`cmd:`** with your real commands (e.g. **`terraform plan`**, **`terraform apply`**). For **run → apply → validate**, use **`run-apply-validate`** instead.

**Secrets and API keys:** Do **not** put vendor-specific keys in this template. Pass them from **where you run dockpipe** — the same tree as **`--workdir`** (usually your repo root): put a **`.env`** there (gitignored at the repo; see root **`.gitignore`**), or **`export ...`** before **`dockpipe`**, or **`--var KEY=VAL`** / **`--env`**. Dockpipe merges **repo-root** **`.env`** with workflow **`vars:`** per **[docs/cli-reference.md](../../docs/cli-reference.md)**.

If you add steps that use **resolver images** (**`codex`**, **`claude`**, …), see **`templates/core/resolvers/codex/README.md`** and **`templates/core/resolvers/claude/README.md`** for which env vars are forwarded into Docker and how to avoid nested sandbox issues with **Codex** inside containers.

```bash
dockpipe --workflow run-apply
```

Needs Docker + **`alpine`**. See **[docs/workflow-yaml.md](../../docs/workflow-yaml.md)** for **`outputs:`**, **`vars:`**, and **`imports:`** if phases need to share state.
