# `src/scripts/dorkpipe/` (maintainer layer)

**Bundled, user-facing scripts** live under **`templates/core/bundles/dorkpipe/`** (canonical). Workflows still use **`scripts/dorkpipe/…`** in YAML; **`paths.go`** resolves that to **`templates/core/…`** or, in this repo, **`src/scripts/dorkpipe/`** when a matching file exists (top-level **`scripts/`** wins if present).

**This directory** holds repo-only helpers: self-analysis, CI normalization, dev-stack, merge-paste-prompt, **`r2-publish.sh`** (host script for **`dockpipe.cloudflare.r2publish`** — R2 upload via `aws s3` or **`CLOUDFLARE_API_TOKEN`** + Terraform + Wrangler), **`secretstore-exec.sh`** (bundled **dotenv** secretstore for **`src/core/workflows/secretstore`**), and similar. **1Password** scripts (**`secretstore-op-exec.sh`**, **`secretstore-op-inject-outputs.sh`**) live under **`.staging/workflows/dockpipe/assets/scripts/`** — until promoted into **`templates/`**.

See **[docs/core-tools.md](../../../docs/core-tools.md)** (DorkPipe scripts section).
