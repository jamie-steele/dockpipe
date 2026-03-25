# `assets/` (repository root)

**Not** the same tree as **`templates/core/assets/`** (workflow templates).

This directory holds **repo-root, non-Go** files that ship with the CLI:

- **`entrypoint.sh`** — copied into images and **`//go:embed`**’d from **`embed.go`** (path **`assets/entrypoint.sh`**).
- **`parse_workflow_steps.py`** — optional helper for workflow tooling.

Build Docker images from the **repository root** so **`COPY assets/entrypoint.sh`** resolves.
