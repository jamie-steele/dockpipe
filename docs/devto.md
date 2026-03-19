# Automating dev.to release posts

After each **GitHub Release** (merge to `main` / `master` — see [releasing.md](releasing.md)), the **Release** workflow can **update an existing** [dev.to](https://dev.to) article via the [Forem API](https://developers.forem.com/).

We **PUT** `https://dev.to/api/articles/{id}` so the same URL stays your “living” release post; you create the article once on dev.to, copy its numeric **article id**, then wire GitHub.

---

## One-time on dev.to

1. Create a post (draft or published) — e.g. “dockpipe releases”.
2. Open it in the editor; the URL looks like `https://dev.to/username/edit/12345678` — **`12345678`** is **`DEVTO_ARTICLE_ID`**.
3. Under **Settings → Account → DEV API keys**, create an API key (used only on GitHub as a **secret**).

---

## GitHub configuration

### Secret (required when enabled)

| Name | Value |
|------|--------|
| **`DEVTO_API_KEY`** | API key from dev.to account settings |

**Settings → Secrets and variables → Actions → New repository secret**

### Variables (repository)

| Name | Required | Description |
|------|----------|-------------|
| **`DEVTO_PUBLISH`** | Yes, to run the job | Set to **`true`** to update dev.to after each non–dry-run release. Anything else → job skipped. |
| **`DEVTO_ARTICLE_ID`** | Yes | Numeric article id from the dev.to editor URL. |
| **`DEVTO_TAGS`** | No | Comma-separated tags, e.g. `dockpipe,cli,golang,docker`. Default: `dockpipe,cli,golang`. |
| **`DEVTO_TITLE`** | No | Article **title** on dev.to. Default: `dockpipe vX.Y.Z released` (uses release tag). |

**Settings → Secrets and variables → Actions → Variables**

Until **`DEVTO_PUBLISH`** is **`true`** and **`DEVTO_ARTICLE_ID`** is set, the **`devto`** job does not run, so you can add secrets/variables whenever you are ready.

---

## What gets published

- **Title:** `DEVTO_TITLE` or default `dockpipe v{VERSION} released`.
- **Body:** Short header with links to the **GitHub release**, then the full contents of **`releasenotes/X.Y.Z.md`** for that version.
- **`canonical_url`:** Set to the GitHub release URL (good for cross-posting / SEO).
- **`published`:** `true` (post goes live on update — same as other edits on dev.to).

Workflow logic lives in **`.github/workflows/release.yml`** (job **`devto`**), after **`publish`**.

---

## Dry runs and forks

- **`dry_run: true`** (manual dispatch) → no GitHub Release → **`devto`** does not run.
- Forks: only run if you set variables + secret on **that** fork (they are not copied from upstream).

---

## Troubleshooting

- **`401` / `403`:** Regenerate **DEVTO_API_KEY**; ensure no extra spaces when pasting the secret.
- **`404` on PUT:** Wrong **`DEVTO_ARTICLE_ID`** or key not allowed to edit that article (must be your post).
- **Job skipped:** Check **`DEVTO_PUBLISH`** is exactly **`true`** (lowercase) and **`DEVTO_ARTICLE_ID`** is non-empty.
