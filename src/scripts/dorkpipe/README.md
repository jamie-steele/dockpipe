# `scripts/dorkpipe/` (maintainer layer)

**Bundled, user-facing scripts** live under **`templates/core/bundles/dorkpipe/`** (canonical). This directory repeats those paths as **symlinks** where the monorepo needs stable `scripts/dorkpipe/…` references (workflows, tests, `ResolveWorkflowScript` prefers repo `scripts/` first).

**Repo-only** files (no symlink) include self-analysis, CI normalization, dev-stack, merge-paste-prompt, and similar — until promoted into **`templates/`**.

See **[docs/core-tools.md](../../docs/core-tools.md)** (DorkPipe scripts section).
