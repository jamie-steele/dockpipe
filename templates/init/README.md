# Init template

Copied to **`templates/<name>/`** when you run **`dockpipe init <name>`**, together with merged **`templates/core/`** (runtimes, resolvers, strategies, **`assets/`** — scripts, images, compose) and example **`scripts/`** / **`images/example/`** at the project root from the scaffold.

Edit **`config.yml`** to match your workflow. Resolver names in **`resolvers:`** resolve from **`templates/core/resolvers/`** unless you add **`templates/<name>/resolvers/`** overrides. **Learning path:** **[docs/onboarding.md](../../docs/onboarding.md)** · **[docs/architecture-model.md](../../docs/architecture-model.md)**.
