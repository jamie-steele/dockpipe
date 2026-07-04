# `assets/` (repository root)

**Not** the same tree as **`templates/core/assets/`** (workflow templates).

This directory holds **repo-root, non-Go** files that ship with the CLI:

- **`entrypoint.sh`** — generic container entrypoint copied into images and **`//go:embed`**’d from **`embed.go`** (path **`assets/entrypoint.sh`**). Package/resolver-specific CLI wrappers belong under `packages/.../assets/`, not here.

Build Docker images from the **repository root** so **`COPY assets/entrypoint.sh`** resolves.
