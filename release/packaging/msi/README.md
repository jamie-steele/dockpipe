# Windows MSI (WiX 3)

Builds **`dockpipe_<version>_windows_amd64.msi`**: per-user install to `%LOCALAPPDATA%\dockpipe`, appends that directory to the **user** `PATH` (no admin for silent install), and can also carry a staged **DockPipe Launcher** payload as an **optional MSI feature** with a per-user Start Menu shortcut.

## Release pipeline (optional)

**Push to `master`:** the GitHub **Release** workflow builds the MSI when the committed marker file **`release/packaging/msi/SHIP_MSI`** is present. This repository now keeps that marker checked in so normal releases publish **`dockpipe_<version>_windows_amd64.msi`** alongside the zip.

**Manual `workflow_dispatch`:** use input **`build_msi`** (default **true**).

## Requirements (local)

- Windows
- [WiX Toolset v3.14](https://github.com/wixtoolset/wix3/releases) — unzip and set `WIX` to the extract folder: **`wix314-binaries.zip`** has **`candle.exe`** at the **root** of the extract; an MSI install often has **`bin\candle.exe`**. **`build.ps1`** accepts either layout.
- Go 1.22+

## Build

```powershell
go build -trimpath -ldflags "-s -w" -o dockpipe.exe ./src/cmd
.\release\packaging\msi\build.ps1 -Version 0.6.0 -SourceExe .\dockpipe.exe -OutDir .\bin\msi-dist
# Or pass WiX root explicitly (avoids relying on $env:WIX): -WixRoot C:\path\to\wix314
```

To include the Qt launcher in the same MSI, stage the launcher payload first and pass `-LauncherStageDir`:

```powershell
cmake -S src/app/tooling/dockpipe-launcher -B src/app/tooling/dockpipe-launcher/build
cmake --build src/app/tooling/dockpipe-launcher/build --config Release
C:\Qt\6.8.3\msvc2022_64\bin\windeployqt.exe --release --compiler-runtime --dir .\bin\launcher-stage .\src\app\tooling\dockpipe-launcher\build\Release\dockpipe-launcher.exe
.\release\packaging\msi\build.ps1 -Version 0.6.0 -SourceExe .\dockpipe.exe -LauncherStageDir .\bin\launcher-stage -OutDir .\bin\msi-dist
```

Output: `bin\msi-dist\dockpipe_0.6.0_windows_amd64.msi`

When the launcher payload is present, the MSI exposes **DockPipe Launcher** as a separate feature in the standard Windows installer UI. Users can deselect it during install and later use **Apps & features → dockpipe → Modify** to add or remove it. Silent installs can choose features with standard MSI properties such as `ADDLOCAL=MainFeature` for CLI-only installs.

## Full local build

Use **`build-local.ps1`** to generate the real launcher-inclusive MSI locally. It can download Qt through **aqtinstall**, build the launcher, run **`windeployqt`**, package the full launcher payload, and smoke-test the installer:

```powershell
.\release\packaging\msi\build-local.ps1 -InstallQt
```

Outputs:

- `bin\windows-msi\msi-dist\dockpipe_<version>_windows_amd64.msi`
- `bin\windows-msi\launcher-build\` (local launcher build output)
- `bin\windows-msi\launcher-stage\` (Qt-deployed launcher payload used to build the MSI)
- `bin\windows-msi\dockpipe.exe` (local CLI binary used to build the MSI)

## CI

The **`build-msi`** job in **`.github/workflows/release.yml`** runs on **`windows-latest`** only (WiX does not run on Linux). It downloads **`wix314-binaries.zip`**, expands it under **`RUNNER_TEMP`**, and passes that folder as **`-WixRoot`**. **`build.ps1`** also supports an installed WiX layout with **`bin\candle.exe`**. **`GITHUB_ENV`** is avoided for WiX paths.

That job now also runs **`smoke-test.ps1`** to verify silent per-user install, direct execution of the installed **`dockpipe.exe`**, and silent uninstall before publishing the artifact. If you include the launcher payload, the smoke test also verifies **`dockpipe-launcher.exe`**, **`Qt6Core.dll`**, **`platforms\qwindows.dll`**, and the Start Menu shortcut.
