# Contributing

Primitive first: run in container, optionally act after. Keep the core minimal.

**Security:** do not file public issues for undisclosed vulnerabilities — see **[SECURITY.md](SECURITY.md)**.

**Issues** for bugs/ideas (check [future-updates.md](docs/future-updates.md)). **PRs** for code/docs; see [AGENTS.md](AGENTS.md). Prefer **feature branch** or **fork** → **PR → `staging`** (integration). **Do not push directly** to **`staging`** / **`master`** if those branches are protected. The maintainer ships by **PR `staging` → `master`** with a **`VERSION`** bump + **`releasenotes/X.Y.Z.md`** — that merge runs **Release**. **CI** (**`govulncheck`**, **`gosec`**, tests, **`make`**, **`.deb`**, **`tests/run_tests.sh`**, **`tests/integration-tests/run.sh`**) runs on PRs to **`staging`** and **`master`**; the **VERSION / release-notes gate** runs only on PRs **into `master`**. **CodeQL** runs in the same **`.github/workflows/ci.yml`** as the **`codeql`** job. Details: [docs/releases/branching.md](docs/releases/branching.md). **Tests:** `bash tests/run_tests.sh` (unit); integration: `bash tests/integration-tests/run.sh`.

**Go:** layout is `lib/dockpipe/{domain,application,infrastructure}` — see [lib/dockpipe/README.md](lib/dockpipe/README.md). Run `go test ./...` and `gofmt` before PRs. Application tests set **`DOCKPIPE_SKIP_DOCKER_PREFLIGHT=1`** so the suite does not require a running Docker daemon (use **`dockpipe doctor`** manually to verify Docker).

**Workflow YAML (user contract):** when changing step/async/merge behavior, update **[docs/workflow-yaml.md](docs/workflow-yaml.md)** and keep [lib/dockpipe/README.md](lib/dockpipe/README.md) in sync for contributor-oriented detail.

**Isolation profiles** (resolver files): shared definitions under **`templates/core/resolvers/<name>`** only — not beside individual workflows. See **[docs/isolation-layer.md](docs/isolation-layer.md)**. New Dockerfile-backed name → `images/<name>/Dockerfile` + a branch in `lib/dockpipe/infrastructure/template.go` (`TemplateBuild`). **Scripts:** add run/act scripts in `scripts/`; workflow configs use `run:` and `act:` (or **`strategy:`** for lifecycle wrappers).

**Template:** add **`templates/<name>/`** with **`config.yml`** (and optional per-workflow **`strategies/`**). Bundled workflows live under **`templates/`**. The **`worktree`** / **`commit`** strategies are **`templates/core/strategies/`** only — not a separate workflow tree. Shared runtimes, resolvers, and strategies ship under **`templates/core/`**. See **`templates/core/README.md`**.

**Action:** add `scripts/<name>.sh`; use `DOCKPIPE_EXIT_CODE`, `DOCKPIPE_CONTAINER_WORKDIR`. Add to `action init --from` list in `bin/dockpipe` if copyable.

**Releases:** merge **`staging` → `master`** (after **`VERSION`** + **`releasenotes/…`**) triggers **`.github/workflows/release.yml`**. See [docs/releases/releasing.md](docs/releases/releasing.md). Optional **dev.to**: [docs/releases/devto.md](docs/releases/devto.md).

---

## Platform testing (we need you)

The maintainer **cannot exercise every OS, CPU, Docker setup, and shell** before a release. CI runs **`go test ./...`** and builds artifacts on **Linux amd64**; everything else depends on **real machines**. If you use dockpipe on an under-tested combo, your reports and small PRs are valuable.

**Manual QA checklists:** **[docs/qa/manual-qa.md](docs/qa/manual-qa.md)** — [core (Linux `.deb`)](docs/qa/manual-qa-core.md), [macOS](docs/qa/manual-qa-macos.md), [Windows + WSL2](docs/qa/manual-qa-windows.md).

**High-impact areas to try before or after a release:**

| Area | Why it matters |
|------|----------------|
| **Windows + WSL2** | **[docs/qa/manual-qa-windows.md](docs/qa/manual-qa-windows.md)** (+ [core](docs/qa/manual-qa-core.md) for `.deb` inside WSL); [docs/install.md](docs/install.md); optional [docs/wsl-windows.md](docs/wsl-windows.md) (git bundle between WSL/Windows — niche). |
| **Linux arm64** | `*_arm64.deb` and `linux_arm64` tarball — **[manual-qa-core.md](docs/qa/manual-qa-core.md)** §1–2. |
| **macOS** | **[docs/qa/manual-qa-macos.md](docs/qa/manual-qa-macos.md)** — Darwin tarballs, Docker Desktop, `PATH`. |
| **Fresh install** | Release **`.deb` / tarball / exe** per **[manual-qa.md](docs/qa/manual-qa.md)** (not only `go run` / `make` from a dev tree). |

**When you open an issue**, please include:

- **Platform:** OS + version, CPU arch (`uname -m` on Unix), Docker version if relevant.
- **How you installed:** e.g. `.deb` arch, tarball, `go install`, from source.
- **Exact command** and **what you expected vs what happened** (paste stderr if small).

**When you open a PR** that fixes a platform-specific bug, add or extend a **test** if you can (Go unit tests run everywhere; shell integration tests may be gated — see `tests/`). Doc-only PRs that clarify install or edge cases for your platform are welcome too.
