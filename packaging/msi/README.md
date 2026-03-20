# Windows MSI (WiX 3)

Builds **`dockpipe_<version>_windows_amd64.msi`**: per-user install to `%LOCALAPPDATA%\dockpipe`, appends that directory to the **user** `PATH` (no admin for silent install).

## Requirements (local)

- Windows
- [WiX Toolset v3.14](https://github.com/wixtoolset/wix3/releases) — unzip and set `WIX` to the folder that contains `bin\candle.exe`
- Go 1.22+

## Build

```powershell
go build -trimpath -ldflags "-s -w" -o dockpipe.exe ./cmd/dockpipe
.\packaging\msi\build.ps1 -Version 0.6.0 -SourceExe .\dockpipe.exe -OutDir .\msi-dist
# Or pass WiX root explicitly (avoids relying on $env:WIX): -WixRoot C:\path\to\wix314
```

Output: `msi-dist\dockpipe_0.6.0_windows_amd64.msi`

## CI

Release workflow **`.github/workflows/release.yml`** downloads **`wix314-binaries.zip`**, expands it under **`RUNNER_TEMP`**, and resolves the WiX root as the parent of **`bin\candle.exe`** (must live in a directory named **`bin`** under the extract — not the first arbitrary `candle.exe` in the tree). It passes **`-WixRoot`** into **`build.ps1`** so **`GITHUB_ENV`** path mangling is avoided.
