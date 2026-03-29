# Contributing

Primitive first: run in container, optionally act after. Keep the core minimal.

**Security:** do not file public issues for undisclosed vulnerabilities — see **[SECURITY.md](SECURITY.md)**.

**Issues** for bugs and ideas. **PRs** for code/docs; see [AGENTS.md](AGENTS.md). Prefer **feature branch** or **fork** → **PR → `staging`** (integration). **Do not push directly** to **`staging`** / **`master`** if those branches are protected. The maintainer ships by **PR `staging` → `master`** with a **`VERSION`** bump + **`release/releasenotes/X.Y.Z.md`** — that merge runs **Release**. **CI** (**`govulncheck`**, **`gosec`**, tests, **`make`**, **`.deb`**, **`tests/run_tests.sh`**, **`tests/integration-tests/run.sh`**) runs on PRs to **`staging`** and **`master`**; the **VERSION / release-notes gate** runs only on PRs **into `master`**. **CodeQL** runs in the same **`.github/workflows/ci.yml`** as the **`codeql`** job. Details: [release/docs/branching.md](release/docs/branching.md). **Tests:** `bash tests/run_tests.sh` (unit); integration: `bash tests/integration-tests/run.sh`.

**Go:** layout is `src/lib/{domain,application,infrastructure}` — see [src/lib/README.md](src/lib/README.md). Run `go test ./...` and `gofmt` before PRs. Application tests set **`DOCKPIPE_SKIP_DOCKER_PREFLIGHT=1`** so the suite does not require a running Docker daemon (use **`dockpipe doctor`** manually to verify Docker).

**Bundled layout in docs/code:** **`TemplateBuild`** resolves Dockerfiles via **`DockerfileDir`**: **`resolvers/<name>/assets/images/<name>`**, then **`bundles/<domain>/assets/images/<domain>`**, then **`assets/images/<name>`** (agnostic: **base-dev**, **dev**, **example**, **minimal**, …). **Optional Compose** examples: **`resolvers/…/assets/compose/`**, **`bundles/…/assets/compose/`**, plus agnostic demos under **`assets/compose/minimal/`** and **`assets/compose/multi-service/`**. **Domain** script bundles use **`bundles/<domain>/assets/scripts/`** (and **`bundles/<domain>/assets/docs/`** for shipped markdown). **Agnostic** helpers stay in **`templates/core/assets/scripts/`** — there is **no** **`templates/core/assets/docs/`** (docs are domain-only under bundles/resolvers as needed). After editing docs or path strings, run **`make check-paths`**. CI runs this check.

**Workflow YAML (user contract):** when changing step/async/merge behavior, update **[docs/workflow-yaml.md](docs/workflow-yaml.md)** and keep [src/lib/README.md](src/lib/README.md) in sync for contributor-oriented detail.

**Resolver** profiles: shared definitions under **`templates/core/resolvers/<name>`** only — not beside individual workflows. See **[docs/isolation-layer.md](docs/isolation-layer.md)**. New Dockerfile-backed name → **`templates/core/resolvers/<name>/assets/images/<name>/Dockerfile`** (or **`bundles/<domain>/assets/images/`** for bundle-owned images) + a case in **`src/lib/infrastructure/template.go`** (`TemplateBuild`). See **[docs/templates-core-assets.md](docs/templates-core-assets.md)**. **Framework scripts:** agnostic run/act/pre scripts belong in **`templates/core/assets/scripts/`**; domain packs under **`templates/core/bundles/<domain>/`**. Workflow YAML references **`scripts/…`** (resolved per **`paths.go`**). Repo-only tooling stays in top-level **`scripts/`** (see **`scripts/README.md`**).

**Template:** add **`templates/<name>/`** with **`config.yml`** (and optional per-workflow **`strategies/`**). Bundled workflows live under **`templates/`**. The **`worktree`** / **`commit`** strategies are **`templates/core/strategies/`** only — not a separate workflow tree. Shared runtimes, resolvers, strategies, and framework scripts ship under **`templates/core/`**. See **[docs/architecture-model.md](docs/architecture-model.md)** (layout table).

**Action:** add **`templates/core/assets/scripts/<name>.sh`** for agnostic bundled actions; domain-specific assets go under **`templates/core/bundles/`** when appropriate. Use `DOCKPIPE_EXIT_CODE`, `DOCKPIPE_CONTAINER_WORKDIR`. Add to `action init --from` list in **`src/cmd`** if copyable.

**Releases:** merge **`staging` → `master`** (after **`VERSION`** + **`release/releasenotes/…`**) triggers **`.github/workflows/release.yml`**. See [release/docs/releasing.md](release/docs/releasing.md). Optional **dev.to**: [release/docs/devto.md](release/docs/devto.md).

---

## Platform testing (we need you)

The maintainer **cannot exercise every OS, CPU, Docker setup, and shell** before a release. CI runs **`go test ./...`** and builds artifacts on **Linux amd64**; everything else depends on **real machines**. If you use dockpipe on an under-tested combo, your reports and small PRs are valuable.

**Manual QA:** **[docs/qa/manual-qa.md](docs/qa/manual-qa.md)** (Linux, macOS, Windows).

**High-impact areas to try before or after a release:**

| Area | Why it matters |
|------|----------------|
| **Windows + WSL2** | [docs/qa/manual-qa.md](docs/qa/manual-qa.md) · [docs/install.md](docs/install.md) · [docs/wsl-windows.md](docs/wsl-windows.md) |
| **Linux arm64** | `*_arm64.deb` / `linux_arm64` tarball — [docs/qa/manual-qa.md](docs/qa/manual-qa.md) |
| **macOS** | [docs/qa/manual-qa.md](docs/qa/manual-qa.md) |
| **Fresh install** | Release **`.deb` / tarball / exe** per **[manual-qa.md](docs/qa/manual-qa.md)** (not only `go run` / `make` from a dev tree). |

**When you open an issue**, please include:

- **Platform:** OS + version, CPU arch (`uname -m` on Unix), Docker version if relevant.
- **How you installed:** e.g. `.deb` arch, tarball, `go install`, from source.
- **Exact command** and **what you expected vs what happened** (paste stderr if small).

**When you open a PR** that fixes a platform-specific bug, add or extend a **test** if you can (Go unit tests run everywhere; shell integration tests may be gated — see `tests/`). Doc-only PRs that clarify install or edge cases for your platform are welcome too.
