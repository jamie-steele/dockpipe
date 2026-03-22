#Requires -Version 5.1
# Launched by the Pipeon Windows shortcut (Desktop / Start Menu). Starts workflow vscode (code-server + Pipeon image).
# Requires: Docker Desktop, Git Bash (bash.exe), dockpipe.exe on PATH or repo bin\dockpipe.exe.
param()

$ErrorActionPreference = "Stop"

$RepoRoot = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
$env:DOCKPIPE_REPO_ROOT = if ($env:DOCKPIPE_REPO_ROOT) { $env:DOCKPIPE_REPO_ROOT } else { $RepoRoot }

$exe = $null
try {
    $cmd = Get-Command dockpipe -ErrorAction Stop
    $exe = $cmd.Source
} catch {
    $candidate = Join-Path $RepoRoot "bin\dockpipe.exe"
    if (Test-Path -LiteralPath $candidate) {
        $exe = $candidate
    }
}

if (-not $exe) {
    Write-Error @"
dockpipe.exe not found.
  Install: irm https://raw.githubusercontent.com/jamie-steele/dockpipe/master/packaging/windows/install.ps1 | iex
  Or from a clone: make build-windows   (produces bin\dockpipe.exe)
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
