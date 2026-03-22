#Requires -Version 5.1
<#
.SYNOPSIS
  Install Pipeon shortcuts (Desktop + Start Menu) with the P icon on Windows.

.DESCRIPTION
  Creates .lnk files that run scripts/pipeon-code-server-launch.ps1 via powershell.exe.
  Requires Docker Desktop, Git Bash, and dockpipe.exe (PATH or repo bin\dockpipe.exe).
  Run from the dockpipe repo root:
    powershell -NoProfile -ExecutionPolicy Bypass -File scripts/install-pipeon-desktop-shortcut.ps1

.EXAMPLE
  .\scripts\install-pipeon-desktop-shortcut.ps1 -DesktopOnly
#>
param(
    [switch]$DesktopOnly,
    [switch]$StartMenuOnly
)

$ErrorActionPreference = "Stop"

$RepoRoot = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
$Launch = Join-Path $RepoRoot "scripts\pipeon-code-server-launch.ps1"
$Icon = Join-Path $RepoRoot "templates\core\assets\images\code-server\favicon.ico"

if (-not (Test-Path -LiteralPath $Launch)) {
    throw "Missing $Launch — run from dockpipe repo root."
}
if (-not (Test-Path -LiteralPath $Icon)) {
    Write-Warning "Missing icon $Icon — run: make pipeon-icons (from Git Bash or WSL) or build from Linux/macOS checkout."
    $Icon = $Launch
}

$both = -not $DesktopOnly -and -not $StartMenuOnly
$doDesktop = $both -or $DesktopOnly
$doStart = $both -or $StartMenuOnly

$ps = Join-Path $env:SystemRoot "System32\WindowsPowerShell\v1.0\powershell.exe"

function New-PipeonShortcut {
    param([string]$Path)
    $W = New-Object -ComObject WScript.Shell
    $S = $W.CreateShortcut($Path)
    $S.TargetPath = $ps
    $S.Arguments = "-NoProfile -ExecutionPolicy Bypass -File `"$Launch`""
    $S.WorkingDirectory = $env:USERPROFILE
    $S.Description = "Pipeon - browser editor (code-server) with Pipeon"
    $S.IconLocation = "$Icon,0"
    $S.Save()
}

if ($doDesktop) {
    $desk = [Environment]::GetFolderPath("Desktop")
    $lnk = Join-Path $desk "Pipeon.lnk"
    New-PipeonShortcut -Path $lnk
    Write-Host "Installed: $lnk"
}

if ($doStart) {
    $programs = Join-Path $env:APPDATA "Microsoft\Windows\Start Menu\Programs"
    if (-not (Test-Path -LiteralPath $programs)) {
        New-Item -ItemType Directory -Force -Path $programs | Out-Null
    }
    $lnk = Join-Path $programs "Pipeon.lnk"
    New-PipeonShortcut -Path $lnk
    Write-Host "Installed: $lnk"
}

Write-Host @"

Next: build the image once (Git Bash from repo root):
  make build-code-server-image

Or in PowerShell (Docker must be on PATH):
  docker build -t dockpipe-code-server:latest -f templates/core/assets/images/code-server/Dockerfile .

Workspace defaults to USERPROFILE. Override: set PIPEON_WORKDIR before launching, or edit the shortcut.
"@
