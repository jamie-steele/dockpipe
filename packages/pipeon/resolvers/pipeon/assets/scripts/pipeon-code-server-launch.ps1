#Requires -Version 5.1
# Launched by the Pipeon Windows shortcut (Desktop / Start Menu). Starts workflow vscode (code-server + Pipeon image).
# Requires: Docker Desktop, Git Bash (bash.exe), and dockpipe.exe on PATH or in DOCKPIPE_BIN.
param()

$ErrorActionPreference = "Stop"

$exe = $null
if ($env:DOCKPIPE_BIN -and (Test-Path -LiteralPath $env:DOCKPIPE_BIN)) {
    $exe = $env:DOCKPIPE_BIN
} else {
    try {
        $cmd = Get-Command dockpipe -ErrorAction Stop
        $exe = $cmd.Source
    } catch {
    }
}

if (-not $exe) {
    Write-Error @"
dockpipe.exe not found.
  Install it on PATH, or set DOCKPIPE_BIN to the executable you want Pipeon to launch.
"@
    exit 1
}

$work = if ($env:PIPEON_WORKDIR -and (Test-Path -LiteralPath $env:PIPEON_WORKDIR)) {
    $env:PIPEON_WORKDIR
} else {
    $env:USERPROFILE
}

Set-Location -LiteralPath $work
& $exe --workflow vscode --workdir .
