# Init template

Copied to **`templates/<name>/`** when you run **`dockpipe init <name> --from init`**, together with merged **`templates/core/`** from a prior **`dockpipe init`** (no name) and optional example **`scripts/`** / **`images/example/`** when copying any non-blank **`--from`** template.

Edit **`config.yml`** to match your workflow. Resolver names in **`resolvers:`** resolve from **`templates/core/resolvers/`** unless you add **`templates/<name>/resolvers/`** overrides. For **codex** / **claude** stacks (API keys, Docker isolation), read **`templates/core/resolvers/codex/README.md`** and **`templates/core/resolvers/claude/README.md`**. **Learning path:** **[docs/onboarding.md](../../docs/onboarding.md)** · **[docs/architecture-model.md](../../docs/architecture-model.md)**.
