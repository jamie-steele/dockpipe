#Requires -Version 5.1
<#
.SYNOPSIS
  Build the current DockPipe source tree and install it into the local per-user DockPipe location.

.DESCRIPTION
  Dev-side install helper for Windows. This script:
  - builds dockpipe.exe from the current repo
  - refreshes src\bin\dockpipe.exe in the repo when possible
  - copies it to %LOCALAPPDATA%\dockpipe\dockpipe.exe
  - compiles the current core slice from src/core into dockpipe-core-<version>.tar.gz
  - copies that fresh tarball into %LOCALAPPDATA%\dockpipe\packages\core

  It intentionally bypasses MSI/zip packaging so maintainers can push source changes quickly.

.EXAMPLE
  .\release\packaging\windows\dev-install.ps1

.EXAMPLE
  .\release\packaging\windows\dev-install.ps1 -Version 0.6.0-dev
#>
param(
    [string]$Version = ""
)

$ErrorActionPreference = "Stop"

function Resolve-RepoRoot {
    $root = Split-Path -Parent $PSScriptRoot
    $root = Split-Path -Parent $root
    $root = Split-Path -Parent $root
    return [IO.Path]::GetFullPath($root)
}

function Require-Go {
    $cmd = Get-Command go -ErrorAction SilentlyContinue
    if (-not $cmd) {
        throw "go not found on PATH. Install Go or add it to PATH before running this script."
    }
    return $cmd.Source
}

$repoRoot = Resolve-RepoRoot
$goExe = Require-Go

if (-not $Version) {
    $Version = (Get-Content -LiteralPath (Join-Path $repoRoot "VERSION") -Raw).Trim()
}
if (-not $Version) {
    throw "VERSION is empty. Pass -Version explicitly or fix the repo VERSION file."
}

$stageRoot = Join-Path $repoRoot "bin\.dockpipe\build\dev-install"
$buildExe = Join-Path $stageRoot "dockpipe.exe"
$repoBinDir = Join-Path $repoRoot "src\bin"
$repoExe = Join-Path $repoBinDir "dockpipe.exe"
$compileWorkdir = Join-Path $stageRoot "compile-workdir"
$compiledCoreDir = Join-Path $compileWorkdir "bin\.dockpipe\internal\packages\core"

$installRoot = Join-Path $env:LOCALAPPDATA "dockpipe"
$installExe = Join-Path $installRoot "dockpipe.exe"
$installCoreDir = Join-Path $installRoot "packages\core"

if (Test-Path -LiteralPath $stageRoot) {
    Remove-Item -LiteralPath $stageRoot -Recurse -Force
}
New-Item -ItemType Directory -Force -Path $stageRoot | Out-Null
New-Item -ItemType Directory -Force -Path $compileWorkdir | Out-Null
New-Item -ItemType Directory -Force -Path $repoBinDir | Out-Null

Write-Host "Building dockpipe.exe from $repoRoot"
Push-Location $repoRoot
try {
    & $goExe build -trimpath -ldflags "-s -w -X main.Version=$Version" -o $buildExe .\src\cmd
    if ($LASTEXITCODE -ne 0) { throw "go build failed" }

    Write-Host "Compiling core package"
    & $buildExe package compile core --workdir $compileWorkdir --from (Join-Path $repoRoot "src\core") --force
    if ($LASTEXITCODE -ne 0) { throw "dockpipe package compile core failed" }
}
finally {
    Pop-Location
}

$coreTarball = Get-ChildItem -LiteralPath $compiledCoreDir -Filter "dockpipe-core-*.tar.gz" -File | Sort-Object LastWriteTime, Name | Select-Object -Last 1
if (-not $coreTarball) {
    throw "No dockpipe-core-*.tar.gz was produced under $compiledCoreDir"
}

New-Item -ItemType Directory -Force -Path $installRoot | Out-Null
New-Item -ItemType Directory -Force -Path $installCoreDir | Out-Null

if (Test-Path -LiteralPath $installExe) {
    Copy-Item -LiteralPath $installExe -Destination ($installExe + ".bak") -Force
}

Write-Host "Refreshing repo-local dockpipe.exe at $repoExe"
try {
    Copy-Item -LiteralPath $buildExe -Destination $repoExe -Force
}
catch {
    Write-Warning "Could not update $repoExe. A running process may still have the file open. Child-process paths now prefer the active dockpipe.exe, but repo-local invocations may still use the older binary until you close the lock and rebuild."
}

Write-Host "Installing dockpipe.exe to $installExe"
Copy-Item -LiteralPath $buildExe -Destination $installExe -Force

Write-Host "Refreshing global core package in $installCoreDir"
Get-ChildItem -LiteralPath $installCoreDir -Filter "dockpipe-core-*.tar.gz" -File -ErrorAction SilentlyContinue | Remove-Item -Force
Copy-Item -LiteralPath $coreTarball.FullName -Destination (Join-Path $installCoreDir $coreTarball.Name) -Force

Write-Host ""
Write-Host "Installed DockPipe dev build:"
Write-Host "  exe:  $installExe"
Write-Host "  core: $(Join-Path $installCoreDir $coreTarball.Name)"
Write-Host ""
Write-Host "Verify with:"
Write-Host "  dockpipe --version"
