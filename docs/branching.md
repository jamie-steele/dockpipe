# Branching, CI, and releases

## Model: **merge to `main` = ship that version**

1. Work on **`dev`** / **`feature/…`**.
2. Open a **PR → `main`** (or **`master`** — CI listens for both).
3. **CI must pass** before merge: `go test ./…` **and** a **release-notes gate** (below).
4. **Merge** → **Release** workflow runs: builds all artifacts (Linux/macOS/Windows zip + **MSI** + `.deb`s) and creates a **GitHub Release** tagged **`vX.Y.Z`**, where **`X.Y.Z` is read from the repo-root [`VERSION`](../VERSION) file** on that commit.

There is **no separate “push a tag to release”** step for normal flow — the tag is created as part of the GitHub Release. (You can still use **Release → Run workflow** manually for dry-runs.)

---

## `VERSION` + `releasenotes/X.Y.Z.md`

- **[`VERSION`](../VERSION)** — single line, semver **`X.Y.Z`** (no `v` prefix). This is the version **you are about to ship** when the PR lands.
- **`releasenotes/${VERSION}.md`** — required body for the GitHub release (same as today).

**Every PR into `main` / `master` must:**

1. **Bump** **`VERSION`** to a **new** semver vs **`main`** (same version twice will **fail** CI — avoids duplicate GitHub Releases).
2. **Modify** **`releasenotes/<new-version>.md`** in the same PR (usually add `releasenotes/0.6.1.md` when bumping to `0.6.1`).

If either is missing, **CI fails** and the PR cannot be merged (assuming branch protection uses CI).

**Docs-only or chore PRs** use a **patch** bump + a short release note (e.g. “Docs: …”). There is no “merge without shipping” path on `main` unless you change this workflow.

---

## What runs when

| Event | Workflow | What it does |
|--------|-----------|----------------|
| **PR** → `main` / `master` | **`ci.yml`** | **`govulncheck`**, **`gosec`**, `go test ./…`, **`make`**, **`.deb`**, **`tests/run_tests.sh`**, **`tests/integration-tests/run.sh`**, **release notes + VERSION bump** |
| **PR** / **push** `main` / `master` | **`codeql.yml`** | **CodeQL** (Go, `security-extended`) — results under **Security → Code scanning** |
| **Push** `main` / `master` (merge) | **`ci.yml`** | Same as PR row except **no** release-notes step |
| **Push** `main` / `master` (merge) | **`release.yml`** | Full build + **GitHub Release** `v$(cat VERSION)`; optional **dev.to** article update ([devto.md](devto.md)) |
| **workflow_dispatch** on Release | **`release.yml`** | Same pipeline; **dry_run** optional; **version** optional (defaults to **`VERSION`**) |

---

## Dry-run a release (no GitHub Release)

**Actions → Release → Run workflow** → optional **version**, set **dry_run** to **true** → download the **workflow artifact** (MSI, debs, checksums, etc.).

---

## Local checks before opening a PR

```bash
go test ./…
# Ensure VERSION matches the file you edited:
test -f "releasenotes/$(tr -d ' \t\r\n' < VERSION).md"
```

**Makefile** `deb` / `deb-all` use **`VERSION`** automatically when the file exists.
