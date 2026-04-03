# Maintainer packages — author workflows into store-shaped trees

**Ignored in git** (local / third-party / backup-to-private-repo): experiment here when **`compile.workflows`** lists **`.staging/packages`**. First-party packs live under tracked **`packages/`** (`pipeon`, `dorkpipe`, `dockpipe-mcp`, `secrets`, `cloud`, …). Third-party **editor profiles** (**vscode**, **cursor-dev**, **code-server**) live here as **`ide/`** only.

When included in **`dockpipe.config.json`**, you **edit YAML + assets here**, then **`dockpipe build`** materializes **`.dockpipe/internal/packages/`** — **`dockpipe-workflow-*.tar.gz`**, **`dockpipe-resolver-*.tar.gz`**, plus **`core`**.

**Example layout:** grouped packs such as **`agent/`** (umbrella only — see **`agent/package.yml`**), or other vendor-specific trees. **`resolvers/`** under each pack hold **tool profiles** (**`profile/`**, **`config.yml`**, **`assets/`**). **First-party dogfood-only** workflows that are not shipped as grouped packages stay in **repo-root `workflows/`** (see **`workflows/README.md`**).

**Workflow YAML** still references resolvers, strategies, and runtimes per **`docs/workflow-yaml.md`**.

## How it maps

| Authoring | After `dockpipe build` |
|-----------|-------------------------|
| Any folder with **`config.yml`** anywhere under this packages tree | **`dockpipe-workflow-<leaf>-*.tar.gz`** (leaf = directory name containing `config.yml`) |
| Resolver dirs with **`profile/`** under pack **`resolvers/<name>/`** | **`dockpipe-resolver-<name>-*.tar.gz`** |
| Umbrella **`package.yml`** with **`kind: package`** (e.g. **`agent/package.yml`**) | Metadata — documents the group |

**`--workflow <name>`** resolves by **leaf** directory name the same way as **`workflows/`** or **`src/core/workflows/`**.

## Layout

- **`.staging/packages/<group>/`** — third-party / vendor umbrellas (**`agent/`**, **`ide/`** — vscode, cursor-dev, code-server)
- **`packages/<group>/`** — first-party maintainer packs (**`pipeon/`**, **`dorkpipe/`**, **`dockpipe-mcp/`**, **`secrets/`**, **`cloud/`**, …)

Repo-root **`workflows/`** is **lean CI / dogfood**; listed compile roots are for **grouped packages** compiled into the store and embedded binary (when configured).

See **`docs/package-model.md`** for the global **authoring vs compiled store** model.

## Tests (self-contained per package)

| Package | Run |
|---------|-----|
| **`packages/pipeon/`** | `bash packages/pipeon/tests/run.sh` |
| **`packages/dorkpipe/`** | `go test -C packages/dorkpipe/lib ./…` and `bash packages/dorkpipe/tests/run.sh` — see **`packages/dorkpipe/tests/README.md`** |
| **`packages/dockpipe-mcp/`** | `bash packages/dockpipe-mcp/tests/run.sh` — see **`packages/dockpipe-mcp/tests/README.md`** |

Repo **`tests/run_tests.sh`** invokes these after core **`tests/unit-tests/`** scripts.
