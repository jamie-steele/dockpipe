# dockpipe/

Repo-local workflows live under **`workflows/`** (one directory per workflow with **`config.yml`**). **`--workflow`** checks **`dockpipe/workflows/<name>/`** before **`templates/<name>/`** on a checkout.

## Populate these workflows (dockpipe source tree)

From the repository root, point **`DOCKPIPE_REPO_ROOT`** at this tree so **`init`** copies from **`templates/`** here (not only the materialized bundle cache), then run **`init`** with the dogfood flags:

```bash
export DOCKPIPE_REPO_ROOT="$(pwd)"
make build
./bin/dockpipe init --dogfood-test --dogfood-codex-pav --dogfood-codex-security
```

That installs **`test`**, **`dogfood-codex-pav`**, and **`dogfood-codex-security`** into **`dockpipe/workflows/`**. If a name already exists, **`init`** skips it—remove that directory first if you need a fresh copy.

See **docs/cli-reference.md** (**dockpipe init**).
