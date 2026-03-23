# Branching, CI, and releases

## Model: **`staging` → `master` = ship**

1. **Contributors** (or you) work on **feature branches** or **forks** → open **PR → `staging`**.
2. **Merge to `staging`** when the change is accepted. **No release** yet — you can push follow-ups, edit **`releasenotes/`**, bump **`VERSION`**, or tweak the contributor’s work on **`staging`** (via PR or maintainer commits, per your rules).
3. When you’re ready to **cut a release**, open **PR `staging` → `master`** (or merge with the same protections). That PR must **bump `VERSION`** and **update `releasenotes/X.Y.Z.md`** (CI enforces this only for PRs **targeting `master`**).
4. **Merge to `master`** → **Release** workflow runs: artifacts + **GitHub Release** **`vX.Y.Z`**.

**`master`** is **the released line**. **`staging`** holds “next release” integration until you ship.

There is **no separate “push a tag to release”** step for normal flow — the tag is created as part of the GitHub Release. (You can still use **Release → Run workflow** manually for dry-runs.)

### Vetting: PRs only (no direct pushes to `staging` / `master`)

**Use pull requests** instead of pushing straight to protected branches. Typical setup:

- **Outside contributors:** **fork** → **PR → `staging`**.
- **Maintainers:** **feature branch** → **PR → `staging`** (same as contributors), or whatever your org allows on **`staging`**.
- **Release:** **PR `staging` → `master`** when **`VERSION`** + release notes are final — merge triggers **Release**.

Turn off **“allow administrators to bypass”** if you want **your own** changes to use the same PR path.

### Default branch = latest release (no drift)

**`master`** should match **what you last shipped** (and **`v$(cat VERSION)`** on that branch). **`staging`** may be **ahead** until the next ship PR.

**First-time GitHub Actions:** Workflows must exist on **`master`** once. Prefer shipping them with a normal **`staging` → `master`** release PR (or a small patch release).

### Recommended flow

1. **On a feature branch:** **Actions → CI → Run workflow** → pick your branch (full tests; **no** VERSION gate).
2. **PR → `staging`:** CI runs **without** the release-notes / VERSION bump requirement.
3. **On `staging`:** You adjust code, **release notes**, and **`VERSION`** when you’re ready to ship.
4. **PR `staging` → `master`:** CI runs **with** the VERSION + release-notes gate → merge **ships**.
5. **Release dry run:** **Actions → Release → Run workflow** → branch + **dry_run: true**.

### Dependabot

**`.github/dependabot.yml`** runs **weekly** version updates for **`gomod`** (repo root) and **`github-actions`** (workflows; grouped into one PR). PRs target **`staging`** so dependency bumps do not push straight to **`master`** (which would run **Release**). Ensure branch **`staging`** exists, or remove **`target-branch`** until it does.

---

## `VERSION` + `releasenotes/X.Y.Z.md`

- **[`VERSION`](../../VERSION)** — single line, semver **`X.Y.Z`** (no `v` prefix). This is the version **you are about to ship** when the **`staging` → `master`** PR lands.
- **`releasenotes/${VERSION}.md`** — required body for the GitHub release.

**Every PR into `master` must** (enforced by CI):

1. **Bump** **`VERSION`** to a **new** semver vs the **base** branch (`master`).
2. **Modify** **`releasenotes/<new-version>.md`** in that same PR.

**PRs into `staging`** do **not** run that gate — integrate freely, then finalize notes + version on **`staging`** before the ship PR.

**Docs-only or chore ship PRs** use a **patch** bump + a short release note (e.g. “Docs: …”).

---

## One PR → one CI workflow run (three jobs)

**`.github/workflows/ci.yml`** is a single workflow named **CI**. Each trigger creates **one** run in the Actions list. Inside it, three **jobs** run in parallel:

- **`test`** (Ubuntu) — **`govulncheck`**, **`gosec`**, **`go test`**, **`make`**, **`.deb`**, shell + integration tests, and (on PRs to **`master`**) the VERSION / release-notes gate.
- **`test-windows`** (Windows) — **`go test ./...`** and **`test_clone_worktree_include.sh`** (bash + git; host pre-script–like coverage). Does **not** run Docker integration tests or full **`tests/run_tests.sh`** (those stay on Linux).
- **`codeql`** — **CodeQL** (Go, **`security-extended`** via **`.github/codeql/codeql-config.yml`**), uploads to **Security → Code scanning** when allowed.

On the **weekly schedule**, only **`codeql`** runs (**`test`** and **`test-windows`** are skipped).

---

## What runs when

| Event | Workflow | What it does |
|--------|-----------|----------------|
| **PR** → **`staging`** | **`ci.yml`** | Jobs **`test`** + **`test-windows`** + **`codeql`** — **no** VERSION / release-notes gate on **`test`** |
| **PR** → **`master`** | **`ci.yml`** | Same + **release notes + VERSION bump** on **`test`** |
| **Push** **`staging`** / **`master`** | **`ci.yml`** | **`test`** + **`test-windows`** + **`codeql`** (no VERSION gate on push) |
| **workflow_dispatch** | **`ci.yml`** | **`test`** + **`test-windows`** + **`codeql`** (no VERSION gate) |
| **Schedule** (weekly) | **`ci.yml`** | **`codeql`** only |
| **Push** **`master`** (merge) | **`release.yml`** | Full build + **GitHub Release** `v$(cat VERSION)`; optional **dev.to** ([devto.md](devto.md)) |
| **workflow_dispatch** on Release | **`release.yml`** | Same pipeline; **dry_run** optional |

> **Release** still runs only on **`push` to `master`**, not on pushes to **`staging`**.

---

## Dry-run a release (no GitHub Release)

**Actions → Release → Run workflow** → optional **version**, set **dry_run** to **true** → download the **workflow artifact** (MSI, debs, checksums, etc.).

---

## Local checks before a **ship** PR (`staging` → `master`)

```bash
go test ./…
# Ship PR must match VERSION file:
test -f "releasenotes/$(tr -d ' \t\r\n' < VERSION).md"
```

**Makefile** `deb` / `deb-all` use **`VERSION`** automatically when the file exists.
