# Release packaging

| Script / file | Output |
|---------------|--------|
| **[build-deb.sh](build-deb.sh)** | Debian **`.deb`** (amd64, arm64) → `release/packaging/build/` |
| **[build-nfpm.sh](build-nfpm.sh)** | **Alpine `.apk`**, **RPM `.rpm`**, **Arch Linux `.pkg.tar.zst`** (amd64, arm64) → `dist/` in CI |
| **[nfpm.yaml.in](nfpm.yaml.in)** | Template for [nfpm](https://github.com/goreleaser/nfpm) (substituted by `build-nfpm.sh`) |
| **[linux/install.sh](linux/install.sh)** | Optional one-liner installer (detects distro) |
| **[windows/install.ps1](windows/install.ps1)** | Windows zip/MSI + optional WSL setup |
| **[wsl/README.md](wsl/README.md)** | WSL bridge notes |

Local test (requires Go):

```bash
./release/packaging/build-nfpm.sh "$(tr -d '\n' < VERSION)" /tmp/dockpipe-nfpm-test
ls /tmp/dockpipe-nfpm-test/
```
