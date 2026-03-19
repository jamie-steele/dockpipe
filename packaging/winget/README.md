# winget (Windows Package Manager)

dockpipe is **not** in the default `winget` source until a manifest is **accepted** into [microsoft/winget-pkgs](https://github.com/microsoft/winget-pkgs). This folder documents how to publish and how users can install **before** that.

## For users (today)

### Option A — PowerShell install script (MSI + checksum)

From an **elevated or normal** PowerShell (MSI is per-user; no admin required):

```powershell
irm https://raw.githubusercontent.com/jamie-steele/dockpipe/main/packaging/windows/install.ps1 | iex
```

Pinned version: download [install.ps1](../windows/install.ps1) and run:

```powershell
.\install.ps1 -Version 0.6.0
```

The script prefers **`dockpipe_*_windows_amd64.msi`** from the GitHub release and verifies **`SHA256SUMS.txt`** when present; it falls back to the **zip** if the release has no MSI yet.

### Option B — MSI / zip from Releases

Download **`dockpipe_<version>_windows_amd64.msi`** from [Releases](https://github.com/jamie-steele/dockpipe/releases) and double-click, or:

```powershell
msiexec /i .\dockpipe_0.6.0_windows_amd64.msi /qn
```

Then open a **new** terminal and run `dockpipe windows setup`.

## For maintainers — submit to winget-pkgs

After a GitHub release is published (MSI + `SHA256SUMS.txt`):

1. Install [wingetcreate](https://github.com/microsoft/winget-create) or use the **winget** bot flow you prefer.
2. Create or update manifests for **`JamieSteele.Dockpipe`** (adjust identifier to match your chosen `Publisher.Package` naming).
3. Point **`InstallerUrl`** at the release MSI, set **`InstallerSha256`** from `SHA256SUMS.txt`, **`PackageVersion`** = semver without `v`.

Example `wingetcreate` flow (adjust URLs/versions):

```text
wingetcreate update JamieSteele.Dockpipe --version 0.6.0 --urls "<MSI_URL>" --submit
```

Use **`InstallerType: msi`** (or `wix` if the validator prefers it for WiX-built MSIs). If the validator asks for **ProductCode**, read it from the installed MSI (e.g. `Get-WmiObject Win32_Product` is slow; prefer **Orca** / **WiX** log / `msiexec /l*v` after a test install).

PRs to **winget-pkgs** are reviewed by Microsoft/community volunteers; expect latency.

## In-repo templates (optional)

You can keep a **copy** of the last accepted manifest tree under `manifests/` for reference when opening the next PR. Do **not** commit secrets; URLs are public.
