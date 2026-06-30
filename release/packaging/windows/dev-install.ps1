#Requires -Version 5.1
<#
.SYNOPSIS
  Build the current DockPipe source tree and install it into the local per-user DockPipe location.

.DESCRIPTION
  Dev-side install helper for Windows. This script:
  - builds dockpipe.exe from the current repo
  - refreshes src\bin\dockpipe.exe in the repo when possible
  - copies it to %LOCALAPPDATA%\dockpipe\dockpipe.exe
  - best-effort rebuilds/redeploys dockpipe-launcher.exe into %LOCALAPPDATA%\dockpipe when Qt/CMake are available
  - restarts a running dockpipe-launcher process after refresh
  - compiles the current core slice from src/core into dockpipe-core-<version>.tar.gz
  - copies that fresh tarball into %LOCALAPPDATA%\dockpipe\packages\core

  It intentionally bypasses MSI/zip packaging so maintainers can push source changes quickly.

.EXAMPLE
  .\release\packaging\windows\dev-install.ps1

.EXAMPLE
  .\release\packaging\windows\dev-install.ps1 -Version 0.6.0-dev

.EXAMPLE
  .\release\packaging\windows\dev-install.ps1 -SkipLauncher
#>
param(
    [string]$Version = "",
    [Alias("NoLauncher")]
    [switch]$SkipLauncher,
    [string]$QtRoot = "",
    [string]$QtVersion = "6.8.3",
    [string]$QtInstallRoot = "C:\Qt"
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

function Find-OptionalTool {
    param([string]$Exe, [string[]]$Fallbacks = @())
    $cmd = Get-Command $Exe -ErrorAction SilentlyContinue
    if ($cmd) {
        return $cmd.Source
    }
    foreach ($fallback in $Fallbacks) {
        if ($fallback -and (Test-Path -LiteralPath $fallback)) {
            return $fallback
        }
    }
    return $null
}

function Resolve-QtRoot {
    param([string]$ExplicitRoot, [string]$QtVersionValue, [string]$QtInstallRootValue)
    if ($ExplicitRoot) {
        return $ExplicitRoot
    }
    return (Join-Path $QtInstallRootValue "$QtVersionValue\msvc2022_64")
}

function Stop-LauncherIfRunning {
    param([string]$InstallRoot)
    $running = @()
    $launcherExe = Join-Path $InstallRoot "dockpipe-launcher.exe"
    foreach ($proc in (Get-Process -Name "dockpipe-launcher" -ErrorAction SilentlyContinue)) {
        try {
            $path = $proc.Path
        }
        catch {
            $path = ""
        }
        if ($path -and [IO.Path]::GetFullPath($path) -eq [IO.Path]::GetFullPath($launcherExe)) {
            $running += $proc
        }
    }
    if ($running.Count -gt 0) {
        Write-Host "Stopping running DockPipe Launcher process(es)"
        $running | Stop-Process -Force
        Start-Sleep -Milliseconds 500
        return $true
    }
    return $false
}

function Try-RefreshLauncher {
    param(
        [string]$RepoRoot,
        [string]$StageRoot,
        [string]$InstallRoot,
        [string]$QtRootValue
    )

    $cmakeExe = Find-OptionalTool -Exe "cmake" -Fallbacks @("C:\Program Files\CMake\bin\cmake.exe")
    if (-not $cmakeExe) {
        Write-Warning "Skipping launcher refresh: cmake not found on PATH."
        return
    }

    $resolvedQtRoot = Resolve-QtRoot -ExplicitRoot $QtRootValue -QtVersionValue $QtVersion -QtInstallRootValue $QtInstallRoot
    $qtConfig = Join-Path $resolvedQtRoot "lib\cmake\Qt6\Qt6Config.cmake"
    $windeployqt = Join-Path $resolvedQtRoot "bin\windeployqt.exe"
    if (-not (Test-Path -LiteralPath $qtConfig) -or -not (Test-Path -LiteralPath $windeployqt)) {
        Write-Warning "Skipping launcher refresh: Qt 6 or windeployqt.exe not found at $resolvedQtRoot."
        return
    }

    $launcherBuildDir = Join-Path $StageRoot "launcher-build"
    $launcherStageDir = Join-Path $StageRoot "launcher-stage"
    if (Test-Path -LiteralPath $launcherBuildDir) {
        Remove-Item -LiteralPath $launcherBuildDir -Recurse -Force
    }
    if (Test-Path -LiteralPath $launcherStageDir) {
        Remove-Item -LiteralPath $launcherStageDir -Recurse -Force
    }
    New-Item -ItemType Directory -Force -Path $launcherStageDir | Out-Null

    Write-Host "Building dockpipe-launcher.exe"
    Push-Location $RepoRoot
    try {
        & $cmakeExe -S src/app/tooling/dockpipe-launcher -B $launcherBuildDir "-DCMAKE_PREFIX_PATH=$resolvedQtRoot"
        if ($LASTEXITCODE -ne 0) { throw "cmake configure failed for dockpipe-launcher" }

        & $cmakeExe --build $launcherBuildDir --config Release
        if ($LASTEXITCODE -ne 0) { throw "cmake build failed for dockpipe-launcher" }
    }
    finally {
        Pop-Location
    }

    $launcherCandidates = @(
        (Join-Path $launcherBuildDir "dockpipe-launcher.exe"),
        (Join-Path $launcherBuildDir "Release\dockpipe-launcher.exe")
    ) | Where-Object { Test-Path -LiteralPath $_ }
    $launcherExe = $launcherCandidates | Select-Object -First 1
    if (-not $launcherExe) {
        throw "Built launcher not found under $launcherBuildDir"
    }

    & $windeployqt --release --compiler-runtime --dir $launcherStageDir $launcherExe
    if ($LASTEXITCODE -ne 0) { throw "windeployqt failed for dockpipe-launcher" }

    Copy-Item -LiteralPath $launcherExe -Destination (Join-Path $launcherStageDir "dockpipe-launcher.exe") -Force
    if (-not (Test-Path -LiteralPath (Join-Path $launcherStageDir "dockpipe-launcher.exe"))) {
        throw "Launcher stage directory missing dockpipe-launcher.exe"
    }

    $restartLauncher = Stop-LauncherIfRunning -InstallRoot $InstallRoot
    Write-Host "Installing dockpipe-launcher payload to $(Join-Path $InstallRoot 'dockpipe-launcher.exe')"
    Get-ChildItem -LiteralPath $launcherStageDir -Force | ForEach-Object {
        Copy-Item -LiteralPath $_.FullName -Destination $InstallRoot -Recurse -Force
    }

    if ($restartLauncher) {
        Write-Host "Restarting DockPipe Launcher"
        Start-Process -FilePath (Join-Path $InstallRoot "dockpipe-launcher.exe") | Out-Null
    }
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

if (-not $SkipLauncher) {
    try {
        Try-RefreshLauncher -RepoRoot $repoRoot -StageRoot $stageRoot -InstallRoot $installRoot -QtRootValue $QtRoot
    }
    catch {
        Write-Warning "Launcher refresh failed: $($_.Exception.Message)"
    }
}
else {
    Write-Host "Skipping launcher rebuild/install (-SkipLauncher)"
}

Write-Host ""
Write-Host "Installed DockPipe dev build:"
Write-Host "  exe:  $installExe"
Write-Host "  core: $(Join-Path $installCoreDir $coreTarball.Name)"
if (-not $SkipLauncher) {
    Write-Host "  launcher: $(Join-Path $installRoot 'dockpipe-launcher.exe')"
}
Write-Host ""
Write-Host "Verify with:"
Write-Host "  dockpipe --version"
