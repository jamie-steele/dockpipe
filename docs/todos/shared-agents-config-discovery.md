# TODO-005 Shared Agents Config Discovery

## Still Open

- Let `agents.yml` resolve from the nearest parent workflow folder with sibling-file override so
  workflow families can share role definitions without copying the same agent catalog into every
  child workflow directory.
- Keep the lookup bounded to the workflow/package authoring root so config inheritance stays
  predictable and does not silently drift into repo-wide fallback behavior.
- Update orchestration docs and authored-surface guidance so the shared-parent lookup order is
  explicit for both AI workers and human authors.
