# `dockpipe/`

**Workflows** (what happens) live here when you materialize them into a project: **`dockpipe/workflows/<name>/config.yml`**. On a git checkout, **`--workflow`** looks at **`dockpipe/workflows/<name>/`** before **`templates/<name>/`**.

**Core** files (**runtimes**, **resolvers**, **strategies**, **assets**) unpack to **`dockpipe/core/`** in the user cache; **`dockpipe init`** merges **`templates/core/`** into your project when authoring.

## Dogfood presets (this repository)

From the repo root, point **`DOCKPIPE_REPO_ROOT`** here so **`init`** copies from **`templates/`**, then:

```bash
export DOCKPIPE_REPO_ROOT="$(pwd)"
make build
./bin/dockpipe init --dogfood-test --dogfood-codex-pav --dogfood-codex-security
```

That installs **`test`**, **`dogfood-codex-pav`**, and **`dogfood-codex-security`** under **`dockpipe/workflows/`**. Existing dirs are skipped—remove one first to refresh.

See **[docs/cli-reference.md](../docs/cli-reference.md)** (**`dockpipe init`**).
