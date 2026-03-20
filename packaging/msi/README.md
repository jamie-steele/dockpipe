# Windows MSI (WiX 3)

Builds **`dockpipe_<version>_windows_amd64.msi`**: per-user install to `%LOCALAPPDATA%\dockpipe`, appends that directory to the **user** `PATH` (no admin for silent install).

## Requirements (local)

- Windows
- [WiX Toolset v3.14](https://github.com/wixtoolset/wix3/releases) — unzip and set `WIX` to the extract folder: **`wix314-binaries.zip`** has **`candle.exe`** at the **root** of the extract; an MSI install often has **`bin\candle.exe`**. **`build.ps1`** accepts either layout.
- Go 1.22+

## Build

```powershell
go build -trimpath -ldflags "-s -w" -o dockpipe.exe ./cmd/dockpipe
.\packaging\msi\build.ps1 -Version 0.6.0 -SourceExe .\dockpipe.exe -OutDir .\msi-dist
# Or pass WiX root explicitly (avoids relying on $env:WIX): -WixRoot C:\path\to\wix314
# Local iteration only: -SkipIceValidation (adds light -sval; skips ICE, not for release)
```

Output: `msi-dist\dockpipe_0.6.0_windows_amd64.msi`

## CI

The **`build-msi`** job in **`.github/workflows/release.yml`** runs on **`windows-latest`** only (WiX does not run on Linux). It downloads **`wix314-binaries.zip`**, expands it under **`RUNNER_TEMP`**, and passes that folder as **`-WixRoot`**. **`build.ps1`** also supports an installed WiX layout with **`bin\candle.exe`**. **`GITHUB_ENV`** is avoided for WiX paths.
