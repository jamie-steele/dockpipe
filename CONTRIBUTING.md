# Contributing

Primitive first: run in container, optionally act after. Keep the core minimal.

**Issues** for bugs/ideas (check [future-updates.md](docs/future-updates.md)). **PRs** for code/docs; see [AGENTS.md](AGENTS.md). Prefer **`dev` / feature branches** → PR → **`main`**. **CI** (**`govulncheck`**, **`gosec`**, `go test`, `make`, `.deb`, `tests/run_tests.sh`, `tests/integration-tests/run.sh`, plus **required** `releasenotes/X.Y.Z.md` / **VERSION** bump on PRs) and **CodeQL** (`.github/workflows/codeql.yml`) run on PRs; **merging ships a GitHub Release** — see [docs/branching.md](docs/branching.md). **Tests:** `bash tests/run_tests.sh` (unit); integration: `bash tests/integration-tests/run.sh`.

**Go:** layout is `lib/dockpipe/{domain,application,infrastructure}` — see [lib/dockpipe/README.md](lib/dockpipe/README.md). Run `go test ./...` and `gofmt` before PRs.

**Workflow YAML (user contract):** when changing step/async/merge behavior, update **[docs/workflow-yaml.md](docs/workflow-yaml.md)** and keep [lib/dockpipe/README.md](lib/dockpipe/README.md) in sync for contributor-oriented detail.

**Resolver:** add a file under `templates/<template>/resolvers/<name>`. New named template → `images/<name>/Dockerfile` + a branch in `lib/dockpipe/infrastructure/template.go` (`TemplateBuild`). **Scripts:** add run/act scripts in `scripts/`; workflow configs use `run:` and `act:`.

**Template:** add `templates/<name>/` with config.yml (run, isolate, act pointing to scripts/), resolvers/, isolate/. No run script in the template; config points to the repo scripts folder. See `templates/llm-worktree/`.

**Action:** add `scripts/<name>.sh`; use `DOCKPIPE_EXIT_CODE`, `DOCKPIPE_CONTAINER_WORKDIR`. Add to `action init --from` list in `bin/dockpipe` if copyable.

**Releases:** merge to **`main`** triggers **`.github/workflows/release.yml`** (version from **`VERSION`**, notes from **`releasenotes/X.Y.Z.md`**). See [docs/releasing.md](docs/releasing.md). Blog post remains manual.

---

## Platform testing (we need you)

The maintainer **cannot exercise every OS, CPU, Docker setup, and shell** before a release. CI runs **`go test ./...`** and builds artifacts on **Linux amd64**; everything else depends on **real machines**. If you use dockpipe on an under-tested combo, your reports and small PRs are valuable.

**Manual QA checklists:** **[docs/manual-qa.md](docs/manual-qa.md)** — [core (Linux `.deb`)](docs/manual-qa-core.md), [macOS](docs/manual-qa-macos.md), [Windows + WSL2](docs/manual-qa-windows.md).

**High-impact areas to try before or after a release:**

| Area | Why it matters |
|------|----------------|
| **Windows + WSL2** | **[docs/manual-qa-windows.md](docs/manual-qa-windows.md)** (+ [core](docs/manual-qa-core.md) for `.deb` inside WSL); [docs/wsl-windows.md](docs/wsl-windows.md), [docs/install.md](docs/install.md). |
| **Linux arm64** | `*_arm64.deb` and `linux_arm64` tarball — **[manual-qa-core.md](docs/manual-qa-core.md)** §1–2. |
| **macOS** | **[docs/manual-qa-macos.md](docs/manual-qa-macos.md)** — Darwin tarballs, Docker Desktop, `PATH`. |
| **Fresh install** | Release **`.deb` / tarball / exe** per **[manual-qa.md](docs/manual-qa.md)** (not only `go run` / `make` from a dev tree). |

**When you open an issue**, please include:

- **Platform:** OS + version, CPU arch (`uname -m` on Unix), Docker version if relevant.
- **How you installed:** e.g. `.deb` arch, tarball, `go install`, from source.
- **Exact command** and **what you expected vs what happened** (paste stderr if small).

**When you open a PR** that fixes a platform-specific bug, add or extend a **test** if you can (Go unit tests run everywhere; shell integration tests may be gated — see `tests/`). Doc-only PRs that clarify install or edge cases for your platform are welcome too.
