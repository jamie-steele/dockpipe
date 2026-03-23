# `src/scripts/dorkpipe/` (maintainer layer)

**Bundled, user-facing scripts** live under **`templates/core/bundles/dorkpipe/`** (canonical). Workflows still use **`scripts/dorkpipe/…`** in YAML; **`paths.go`** resolves that to **`templates/core/…`** or, in this repo, **`src/scripts/dorkpipe/`** when a matching file exists (top-level **`scripts/`** wins if present).

**This directory** holds repo-only helpers: self-analysis, CI normalization, dev-stack, merge-paste-prompt, and similar — until promoted into **`templates/`**.

See **[docs/core-tools.md](../../../docs/core-tools.md)** (DorkPipe scripts section).
