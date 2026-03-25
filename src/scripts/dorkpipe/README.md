# `src/scripts/dorkpipe/` (maintainer layer)

**Bundled, user-facing scripts** live under **`templates/core/bundles/dorkpipe/`** (canonical). Workflows still use **`scripts/dorkpipe/…`** in YAML; **`paths.go`** resolves that to **`templates/core/…`** or, in this repo, **`src/scripts/dorkpipe/`** when a matching file exists (top-level **`scripts/`** wins if present).

**This directory** holds repo-only helpers: self-analysis, CI normalization, dev-stack, merge-paste-prompt, **`r2-publish.sh`** (R2 upload via `aws s3` or **`CLOUDFLARE_API_TOKEN`** + Terraform + Wrangler), **`secretstore-exec.sh`** (pluggable secret-store wrapper for **`templates/secretstore`**), **`secretstore-op-inject-outputs.sh`** ( **`op inject`** → outputs file for multi-step merge; see **`.staging/workflows/secretstore-r2-publish-test`**), and similar — until promoted into **`templates/`**.

See **[docs/core-tools.md](../../../docs/core-tools.md)** (DorkPipe scripts section).
