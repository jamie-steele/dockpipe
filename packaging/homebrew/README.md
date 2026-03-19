# Homebrew packaging

This directory contains the Homebrew formula used by a tap repo.

## Files

- `dockpipe.rb` — Formula for `brew install dockpipe`.

## Maintainer flow (new release)

1. Compute source tarball SHA for the release tag:

   ```bash
   VERSION=0.6.0
   curl -L "https://github.com/jamie-steele/dockpipe/archive/refs/tags/v${VERSION}.tar.gz" -o "/tmp/dockpipe-${VERSION}.tar.gz"
   shasum -a 256 "/tmp/dockpipe-${VERSION}.tar.gz"
   ```

2. Update formula:
   - `url` -> `.../v<version>.tar.gz`
   - `sha256` -> computed hash

3. Commit formula update in tap repo (recommended: `homebrew-dockpipe`):
   - path: `Formula/dockpipe.rb`

4. Verify install:

   ```bash
   brew tap jamie-steele/dockpipe
   brew install dockpipe
   dockpipe --help
   ```

## Notes

- Formula builds from source using Go and installs full dockpipe runtime assets (`lib`, `scripts`, `images`, `templates`, `docs`) into `libexec`.
- Wrapper sets `DOCKPIPE_REPO_ROOT` so runtime path resolution works in Homebrew layouts.
