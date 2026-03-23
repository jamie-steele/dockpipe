#Requires -Version 5.1
<#
.SYNOPSIS
  Download and install dockpipe on Windows (MSI preferred, else zip).

.DESCRIPTION
  - Fetches the latest GitHub release (or a specific -Version).
  - Verifies SHA256 using SHA256SUMS.txt from the same release when available.
  - Installs dockpipe.exe only (bundled templates/images unpack to the user cache on first run — no extra folders beside the exe).
  - MSI: per-user WiX install to %LOCALAPPDATA%\dockpipe, PATH updated. Zip fallback: %LOCALAPPDATA%\Programs\dockpipe.

  After install, optionally configures WSL for DOCKPIPE_USE_WSL_BRIDGE=1 (minimal Alpine + latest Linux dockpipe from GitHub). Use -SkipWSLSetup to skip. May prompt for Administrator (WSL) or require a reboot.

.EXAMPLE
  iwr -useb https://raw.githubusercontent.com/jamie-steele/dockpipe/master/release/packaging/windows/install.ps1 | iex

.EXAMPLE
  .\install.ps1 -Version 0.6.0
#>
param(
    [string]$Version = "",
    [string]$Repo = "jamie-steele/dockpipe",
    [switch]$SkipWSLSetup
)

$ErrorActionPreference = "Stop"

function Invoke-DockpipeWslSetup {
    param([Parameter(Mandatory = $true)][string]$DockpipeExe)
    if ($SkipWSLSetup) {
        Write-Host "Skipping WSL setup (-SkipWSLSetup). For bridge users later: dockpipe windows setup --bootstrap-wsl --distro Alpine --non-interactive --install-dockpipe"
        return
    }
    if (-not (Test-Path -LiteralPath $DockpipeExe)) {
        Write-Warning "dockpipe.exe not found at $DockpipeExe — skipping WSL setup."
        return
    }
    Write-Host "Configuring WSL for optional DOCKPIPE_USE_WSL_BRIDGE (Alpine + dockpipe from GitHub). Administrator / reboot may be required..."
    & $DockpipeExe @('windows', 'setup', '--bootstrap-wsl', '--distro', 'Alpine', '--non-interactive', '--install-dockpipe')
    if ($LASTEXITCODE -ne 0) {
        Write-Warning "WSL setup exited $LASTEXITCODE — install Docker Desktop + Git for Windows for native mode, or run manually: dockpipe windows doctor"
    }
}

function Get-Release {
    param([string]$Ver)
    $base = "https://api.github.com/repos/$Repo/releases"
    if ($Ver) {
        $tag = if ($Ver.StartsWith("v")) { $Ver } else { "v$Ver" }
        return Invoke-RestMethod -Uri "$base/tags/$tag" -Headers @{ "User-Agent" = "dockpipe-install" }
    }
    return Invoke-RestMethod -Uri "$base/latest" -Headers @{ "User-Agent" = "dockpipe-install" }
}

function Get-Sha256Map {
    param($Release)
    $sumAsset = $Release.assets | Where-Object { $_.name -eq "SHA256SUMS.txt" } | Select-Object -First 1
    if (-not $sumAsset) { return @{} }
    $tmp = Join-Path ([System.IO.Path]::GetTempPath()) "dockpipe-sha256sums.txt"
    Invoke-WebRequest -Uri $sumAsset.browser_download_url -OutFile $tmp -UseBasicParsing
    $map = @{}
    Get-Content $tmp | ForEach-Object {
        # GNU sha256sum: "hash  file" or "hash *file"
        if ($_ -match '^\s*([a-fA-F0-9]{64})\s+\*?\s*(.+)$') {
            $name = $matches[2].Trim()
            $map[$name] = $matches[1].ToLowerInvariant()
        }
    }
    Remove-Item $tmp -Force -ErrorAction SilentlyContinue
    $map
}

$rel = Get-Release -Ver $Version
$verTag = $rel.tag_name.TrimStart("v")
$sums = Get-Sha256Map -Release $rel

$msi = $rel.assets | Where-Object { $_.name -match "dockpipe_.*_windows_amd64\.msi$" } | Select-Object -First 1
$zip = $rel.assets | Where-Object { $_.name -match "dockpipe_.*_windows_amd64\.zip$" } | Select-Object -First 1

if ($msi) {
    $dl = Join-Path $env:TEMP $msi.name
    Write-Host "Downloading $($msi.name) ..."
    Invoke-WebRequest -Uri $msi.browser_download_url -OutFile $dl -UseBasicParsing
    if ($sums.ContainsKey($msi.name)) {
        $h = (Get-FileHash -Algorithm SHA256 -LiteralPath $dl).Hash.ToLowerInvariant()
        if ($h -ne $sums[$msi.name]) {
            throw "SHA256 mismatch for $($msi.name). Expected $($sums[$msi.name]), got $h"
        }
    }
    Write-Host "Installing MSI (elevates if needed) ..."
    $p = Start-Process msiexec.exe -ArgumentList @("/i", "`"$dl`"", "/qn", "/norestart") -Wait -PassThru
    if ($p.ExitCode -ne 0 -and $p.ExitCode -ne 3010) {
        throw "msiexec failed with exit code $($p.ExitCode)"
    }
    $msiExe = Join-Path $env:LOCALAPPDATA "dockpipe\dockpipe.exe"
    Invoke-DockpipeWslSetup -DockpipeExe $msiExe
    Write-Host "Installed dockpipe $verTag. Open a new terminal for PATH changes, then: dockpipe --help"
    exit 0
}

if (-not $zip) {
    throw "No windows_amd64.msi or .zip found in release $($rel.tag_name)."
}

$zipPath = Join-Path $env:TEMP $zip.name
Write-Host "Downloading $($zip.name) (no MSI in this release) ..."
Invoke-WebRequest -Uri $zip.browser_download_url -OutFile $zipPath -UseBasicParsing
if ($sums.ContainsKey($zip.name)) {
    $h = (Get-FileHash -Algorithm SHA256 -LiteralPath $zipPath).Hash.ToLowerInvariant()
    if ($h -ne $sums[$zip.name]) {
        throw "SHA256 mismatch for $($zip.name)."
    }
}

$dest = Join-Path $env:LOCALAPPDATA "Programs\dockpipe"
New-Item -ItemType Directory -Force -Path $dest | Out-Null
Expand-Archive -LiteralPath $zipPath -DestinationPath $dest -Force
$exe = Join-Path $dest "dockpipe.exe"
if (-not (Test-Path $exe)) {
    throw "Expected dockpipe.exe under $dest"
}

$userPath = [Environment]::GetEnvironmentVariable("Path", "User")
if ($userPath -notlike "*$dest*") {
    [Environment]::SetEnvironmentVariable("Path", "$userPath;$dest", "User")
    $env:Path = "$env:Path;$dest"
}
Invoke-DockpipeWslSetup -DockpipeExe $exe
Write-Host "Installed dockpipe $verTag to $dest (user PATH updated). Open a new terminal, then: dockpipe --help"
