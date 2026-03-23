# `shipyard/`

**Experimental / maintainer workflows** — quick iteration, CI, dogfood, and demos. Not the stable, user-facing catalog under **`templates/`** (future CDN bundles will ship separately).

**Workflows** (what happens) live here when you materialize them into a project: **`shipyard/workflows/<name>/config.yml`**. On a git checkout, **`--workflow`** looks at **`shipyard/workflows/<name>/`** before **`templates/<name>/`**.

**Core** files (**runtimes**, **resolvers**, **strategies**, **assets**) unpack to **`shipyard/core/`** in the user cache; **`dockpipe init`** merges **`templates/core/`** into your project when authoring.

## Internal workflows (this repository)

CI, demos, and other **dockpipe-the-tool** workflows live only under **`shipyard/workflows/<name>/`**, not under **`templates/`**. User-facing examples stay under **`templates/`** (e.g. **`run`**, **`run-apply`**, **`run-apply-validate`**).

There is **no** CLI shortcut to install these — they are not a product feature. To reuse one elsewhere, copy the directory or run **`dockpipe init &lt;name&gt; --from /path/to/shipyard/workflows/&lt;name&gt;`**. See **[AGENTS.md](../AGENTS.md)**.
