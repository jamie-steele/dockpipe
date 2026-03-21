# Automating dev.to release posts

**Related:** Long-form article draft in-repo: **[blog-dockpipe-primitive.md](blog-dockpipe-primitive.md)** (source for dev.to / announcements; the workflow below posts **`releasenotes/X.Y.Z.md`**, not this file).

After each **GitHub Release** (merge **`staging` → `master`** in normal flow — see [branching.md](branching.md) / [releasing.md](releasing.md)), the **Release** workflow can **update an existing** [dev.to](https://dev.to) article via the [Forem API](https://developers.forem.com/).

We **PUT** `https://dev.to/api/articles/{id}` so the same URL stays your “living” release post; you create the article once on dev.to, copy its numeric **article id**, then wire GitHub.

---

## One-time on dev.to

1. Create a post (draft or published) — e.g. “dockpipe releases”.
2. Get the numeric **`DEVTO_ARTICLE_ID`** (used in `PUT https://dev.to/api/articles/{id}`):
   - **Editor URL (when shown):** `https://dev.to/username/edit/12345678` → **`12345678`** is the id.
   - **Page HTML:** Open the **public** article page, open **DevTools → Elements**, and find the `<article>` element. It includes **`data-article-id="<id>"`** (and often **`data-article-slug`**, **`data-path`**, etc.). That **`data-article-id`** value is **`DEVTO_ARTICLE_ID`**. You can also **View Page Source** and search for `data-article-id` or the same number in embedded JSON / `meta` tags.
   - **Slug-only editor** (no `/edit/digits`): list your public articles and read **`id`** from the JSON:
     ```bash
     curl -sS "https://dev.to/api/articles?username=YOUR_USERNAME" | jq '.[] | select(.slug | test("run-isolate"; "i")) | {id, title, slug}'
     ```
     Or open the response in a browser:  
     `https://dev.to/api/articles?username=YOUR_USERNAME`  
     Find the object whose **`slug`** or **`path`** matches your post; copy **`id`** (integer).
   - **Single article (public):** `GET https://dev.to/api/articles/{id}` works only if you already know **`id`** (useful to verify).
   There is **no** `GET /api/articles/{username}/{slug}` endpoint on dev.to.
3. Under **Settings → Account → DEV API keys**, create an API key (used only on GitHub as a **secret**).

---

## GitHub configuration

The **Release** workflow assigns the **`publish`** and **`devto`** jobs to GitHub **Environment** **`release`**. Store **`DEVTO_*`** and **`DEVTO_API_KEY`** there (recommended) or at repository scope — same names.

**Settings → Environments → `release` → Environment secrets / Environment variables**

### Secret (required when the dev.to job runs)

| Name | Value |
|------|--------|
| **`DEVTO_API_KEY`** | API key from dev.to account settings |

### Variables

| Name | Required | Description |
|------|----------|-------------|
| **`DEVTO_PUBLISH`** | Yes, to run the job | Set to **`true`** to update dev.to after each non–dry-run release. Anything else → job skipped. |
| **`DEVTO_ARTICLE_ID`** | Yes | Numeric id (`data-article-id` on `<article>`, or editor URL). |
| **`DEVTO_TAGS`** | No | Comma-separated tag **names** (spaces after commas are fine). Use **lowercase** (e.g. `cli,docker,automation,ai`). Default: `dockpipe,cli,golang`. |
| **`DEVTO_TITLE`** | No | Article **title** on dev.to. Default: `dockpipe vX.Y.Z released` (uses release tag). |

Until **`DEVTO_PUBLISH`** is **`true`** and **`DEVTO_ARTICLE_ID`** is set, the **`devto`** job does not run.

**Protection rules:** If **`release`** has **required reviewers** or a **wait timer**, the **`publish`** job (GitHub Release) waits on that gate every time **`master`** runs this workflow. Drop those rules on **`release`** if you want releases without a manual approval step.

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
- **“No such endpoint” / wrong GET URL:** Use **`GET https://dev.to/api/articles?username=…`** to list articles, or **`GET https://dev.to/api/articles/{id}`** once you have the numeric id — not a username/slug path.
