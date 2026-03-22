# `dockpipe/`

**Workflows** (what happens) live here when you materialize them into a project: **`dockpipe/workflows/<name>/config.yml`**. On a git checkout, **`--workflow`** looks at **`dockpipe/workflows/<name>/`** before **`templates/<name>/`**.

**Core** files (**runtimes**, **resolvers**, **strategies**, **assets**) unpack to **`dockpipe/core/`** in the user cache; **`dockpipe init`** merges **`templates/core/`** into your project when authoring.

## Internal workflows (this repository)

CI, demos, and other **dockpipe-the-tool** workflows live only under **`dockpipe/workflows/<name>/`**, not under **`templates/`**. User-facing examples stay under **`templates/`** (e.g. **`run`**, **`run-apply`**, **`run-apply-validate`**).

There is **no** CLI shortcut to install these — they are not a product feature. To reuse one elsewhere, copy the directory or run **`dockpipe init &lt;name&gt; --from /path/to/dockpipe/workflows/&lt;name&gt;`**. See **[AGENTS.md](../AGENTS.md)**.
