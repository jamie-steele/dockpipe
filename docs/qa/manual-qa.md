# Manual QA (release & platform checks)

CI runs **`go test ./...`**; it does **not** replace installs on real machines. Use these checklists **before a release** or after changing packaging, Windows native/bridge behavior, or Docker-facing behavior.

| Doc | Use when |
|-----|----------|
| **[manual-qa-core.md](manual-qa-core.md)** | **Linux** (native or **WSL**): `.deb` install/upgrade, Linux tarballs, core CLI smoke. |
| **[manual-qa-macos.md](manual-qa-macos.md)** | **macOS**: Darwin tarball (Intel vs Apple Silicon), Docker Desktop, PATH. |
| **[manual-qa-windows.md](manual-qa-windows.md)** | **Windows**: native `dockpipe.exe`, optional WSL bridge (`DOCKPIPE_USE_WSL_BRIDGE=1`), `windows setup` / `doctor`. |

**Contributors:** [CONTRIBUTING.md — Platform testing](../CONTRIBUTING.md#platform-testing-we-need-you).
