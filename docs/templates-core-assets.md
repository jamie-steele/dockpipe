# `templates/core/assets/`

Support files (scripts, images, compose samples) — **not** extra primitives. Definitions: **[architecture-model.md](architecture-model.md)**.

```
templates/core/assets/scripts/   # agnostic helpers (clone, commit, terraform lib, …)
templates/core/assets/images/    # Dockerfiles for TemplateBuild / --isolate
templates/core/assets/compose/   # optional examples (not auto-run)
templates/core/bundles/<domain>/ # domain trees (dorkpipe, …)
```

**Installed or materialized separately from `dockpipe init`** into a project’s **`templates/core/`** when you use **`dockpipe install core`** or other package/materialization flows.

| Kind | Notes |
|------|--------|
| **SAFE TO BUNDLE** | DockPipe-authored; ship in the binary. |
| **USER-SUPPLIED** | Credentials / tools the user installs. |

**Script details:** **`src/core/assets/scripts/README.md`** (**`DOCKPIPE_TF_*`**; Terraform library lives under **`packages/terraform/resolvers/terraform-core/`**). **Image search order:** resolver **`assets/images/<name>`** → bundle → **`assets/images/<name>`**.

**Maintainer-only script trees** (e.g. **`scripts/dorkpipe/`**) live in **`.staging/packages/…`** per **`dockpipe.config.json`** — not duplicated as fake repo-root stubs.
