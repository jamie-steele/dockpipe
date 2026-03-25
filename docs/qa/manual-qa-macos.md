# Manual test plan — macOS

**Scope:** Validates **Darwin release tarballs** and typical **Docker on Mac** setups. The CLI is the same Go binary as Linux; differences are **install path**, **CPU arch**, and **Docker Desktop** behavior.

**Who:** Maintainers and [platform-testing contributors](../CONTRIBUTING.md#platform-testing-we-need-you).

**Also see:** [install.md](../install.md) · [manual-qa.md](manual-qa.md) (index) · [release/packaging/homebrew/README.md](../../release/packaging/homebrew/README.md) if using Homebrew

---

## Prerequisites

- **macOS** (note version, e.g. 14.x).
- **Docker** running — usually **Docker Desktop** (`docker info` works). Alternatives (Colima, Rancher Desktop) are fine but note which in reports.
- Know your arch:
  - Apple Silicon → **`darwin_arm64`** tarball (or Rosetta + amd64 — prefer native arm64).
  - Intel Mac → **`darwin_amd64`** tarball.

```bash
uname -m   # arm64 → use darwin_arm64; x86_64 → darwin_amd64
```

---

## 1. Tarball from release

- [ ] Download `dockpipe_<version>_darwin_arm64.tar.gz` **or** `darwin_amd64.tar.gz` matching your Mac.
- [ ] `tar -xzf dockpipe_<version>_darwin_<arch>.tar.gz`
- [ ] `chmod +x dockpipe`
- [ ] `./dockpipe --help`
- [ ] `./dockpipe -- echo ok` (Docker must be up).

### PATH (optional)

- [ ] Move binary to e.g. `~/bin` or `/usr/local/bin`, ensure directory is on `PATH`, run `dockpipe --help` without `./`.

---

## 2. Docker sanity

- [ ] `docker run --rm hello-world` (or `docker info` only if images already pulled).
- [ ] If `dockpipe -- echo ok` fails with socket errors, check Docker Desktop is **running** and your user can access the engine.

---

## 3. Optional: Homebrew

If you test the in-repo formula or a tap:

- [ ] `brew install …` / upgrade path per [release/packaging/homebrew/README.md](../../release/packaging/homebrew/README.md).
- [ ] `dockpipe --help` from Homebrew prefix.

---

## 4. Optional: heavier smoke

- [ ] `dockpipe --isolate base-dev -- echo ok` (image build/pull may take time).
- [ ] Workflow or `--repo` flow if you have network and credentials.

---

## 5. What to record if something fails

| Field | Example |
|--------|---------|
| macOS version | 14.2 |
| `uname -m` | arm64 |
| Docker | Docker Desktop 4.x |
| Install | darwin_arm64 tarball / Homebrew |
| Command + stderr | … |

---

## 6. Maintainer: local build (cross-compile from Linux is fine)

```bash
GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o dockpipe ./src/cmd/dockpipe
# or GOARCH=amd64 for Intel
```

Release pipeline builds both tarballs; see [releasing.md](../../release/docs/releasing.md).
