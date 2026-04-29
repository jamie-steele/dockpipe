# GitHub Container Registry — `dockpipe.github.ghcrpush`

Host workflow for pushing a local OCI/container image to **GitHub Container Registry**.

This workflow does **not** build the image. It expects a local source image to already exist, then:

- logs into **`ghcr.io`** (unless **`GHCR_LOGIN_SKIP=1`**)
- retags the local image for the target GHCR repository
- pushes one or more tags

Implementation lives beside this workflow as the logical script id:

- **`scripts/dockpipe.github.ghcrpush/ghcr-push.sh`**

## Typical use

```bash
export GHCR_IMAGE_SOURCE=dockpipe-launcher:local
export GHCR_IMAGE_TARGET=ghcr.io/dockpipe/dockpipe-launcher
export GHCR_TAGS=0.6.0,latest
export GHCR_USERNAME="$GITHUB_ACTOR"
export GHCR_TOKEN="$GITHUB_TOKEN"

dockpipe --workflow dockpipe.github.ghcrpush
```

## Single fully tagged push

If **`GHCR_TAGS`** is empty and **`GHCR_IMAGE_TARGET`** already includes a tag, the workflow pushes exactly that ref:

```bash
export GHCR_IMAGE_SOURCE=dockpipe-launcher:local
export GHCR_IMAGE_TARGET=ghcr.io/dockpipe/dockpipe-launcher:0.6.0
export GHCR_USERNAME="$GITHUB_ACTOR"
export GHCR_TOKEN="$GITHUB_TOKEN"

dockpipe --workflow dockpipe.github.ghcrpush
```

If **`GHCR_TAGS`** is empty and **`GHCR_IMAGE_TARGET`** does **not** include a tag, the workflow defaults to **`latest`**.

## Dry run

```bash
GHCR_PUSH_DRY_RUN=1 dockpipe --workflow dockpipe.github.ghcrpush
```

## Variables

| Variable | Default | Meaning |
|----------|---------|---------|
| `GHCR_CONTAINER_CLI` | `docker` | Container CLI used for `login`, `tag`, and `push`. `podman` also works when it supports the same flags. |
| `GHCR_IMAGE_SOURCE` | *(required)* | Existing local image reference to push from. |
| `GHCR_IMAGE_TARGET` | *(required)* | Target repository or fully tagged image ref under `ghcr.io`. |
| `GHCR_TAGS` | *(empty)* | Optional comma/space/newline separated tags. |
| `GHCR_USERNAME` | `GITHUB_ACTOR` | Registry username; required unless `GHCR_LOGIN_SKIP=1`. |
| `GHCR_TOKEN` | `GITHUB_TOKEN` | Registry token; required unless `GHCR_LOGIN_SKIP=1`. |
| `GHCR_LOGIN_SKIP` | `0` | Set to `1` to reuse existing container CLI auth. |
| `GHCR_PUSH_DRY_RUN` | `0` | Set to `1` to print the plan only. |
| `GHCR_REGISTRY` | `ghcr.io` | Registry host. |

## Notes

- GitHub Actions usually provides **`GITHUB_ACTOR`** and **`GITHUB_TOKEN`** already.
- For local/manual use, a classic PAT or fine-grained token with package write permission is usually the easiest path.
- This workflow is intentionally narrow: build the image in a separate workflow or pipeline step, then publish it here.
