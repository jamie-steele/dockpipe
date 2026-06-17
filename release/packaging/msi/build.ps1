# Build dockpipe MSI (WiX 3.x). Run on Windows.
# Usage: .\build.ps1 -Version 0.6.0 -SourceExe C:\path\dockpipe.exe -OutDir C:\out [-LauncherStageDir C:\path\launcher-stage]
param(
    [Parameter(Mandatory = $true)][string]$Version,
    [Parameter(Mandatory = $true)][string]$SourceExe,
    [Parameter(Mandatory = $true)][string]$OutDir,
    [Parameter(Mandatory = $false)][string]$CoreStageDir = "",
    [Parameter(Mandatory = $false)][string]$LauncherStageDir = "",
    [Parameter(Mandatory = $false)][string]$LauncherExe = "",
    # Optional: WiX root (extract folder: root candle.exe from wix314-binaries.zip, or parent of bin\candle.exe). Prefer passing this in CI — GITHUB_ENV can mangle Windows paths.
    [Parameter(Mandatory = $false)][string]$WixRoot = ""
)

$ErrorActionPreference = "Stop"
if (-not (Test-Path -LiteralPath $SourceExe)) {
    throw "SourceExe not found: $SourceExe"
}
if ($CoreStageDir -and -not (Test-Path -LiteralPath $CoreStageDir)) {
    throw "CoreStageDir not found: $CoreStageDir"
}
if ($LauncherStageDir -and -not (Test-Path -LiteralPath $LauncherStageDir)) {
    throw "LauncherStageDir not found: $LauncherStageDir"
}
if ($LauncherExe -and -not (Test-Path -LiteralPath $LauncherExe)) {
    throw "LauncherExe not found: $LauncherExe"
}

# WiX requires Product/@Version as X.Y.Z.W
$fourPart = if ($Version -match '^\d+\.\d+\.\d+$') { "$Version.0" } elseif ($Version -match '^\d+\.\d+\.\d+\.\d+$') { $Version } else {
    throw "Version must be semver like 0.6.0 or 0.6.0.0, got: $Version"
}

$rawWix = if ($WixRoot) { $WixRoot } else { $env:WIX }
# CI may set WIX with forward slashes; normalize for Windows APIs.
if (-not $rawWix) {
    throw "WIX environment variable must point to WiX Toolset v3 root (candle.exe). Install from https://github.com/wixtoolset/wix3/releases"
}
$wixRoot = [IO.Path]::GetFullPath($rawWix.Replace('/', '\'))
# Official wix314-binaries.zip lays tools at the extract root (candle.exe, light.exe). Installed WiX often uses bin\candle.exe.
$binCandle = Join-Path $wixRoot "bin\candle.exe"
$rootCandle = Join-Path $wixRoot "candle.exe"
if (Test-Path -LiteralPath $binCandle) {
    $candle = $binCandle
    $light = Join-Path $wixRoot "bin\light.exe"
} elseif (Test-Path -LiteralPath $rootCandle) {
    $candle = $rootCandle
    $light = Join-Path $wixRoot "light.exe"
} else {
    throw "WiX root must contain bin\candle.exe (typical install) or candle.exe (wix314-binaries.zip layout). Got: $rawWix"
}
if (-not (Test-Path -LiteralPath $light)) {
    throw "light.exe not found next to candle (expected bin\light.exe or light.exe under WiX root). Root: $wixRoot"
}
$heat = Join-Path $wixRoot "heat.exe"
if (-not (Test-Path -LiteralPath $heat)) {
    throw "heat.exe not found under WiX root: $wixRoot"
}
$wxs = Join-Path $PSScriptRoot "dockpipe.wxs"
$objDir = Join-Path $OutDir "wixobj"
New-Item -ItemType Directory -Force -Path $objDir | Out-Null
$msiName = "dockpipe_${Version}_windows_amd64.msi"
$msiPath = Join-Path $OutDir $msiName

$srcAbs = (Resolve-Path -LiteralPath $SourceExe).Path
$coreStageAbs = ""
$launcherStageAbs = ""
$tempLauncherStageDir = ""

if ($CoreStageDir) {
    $coreStageAbs = (Resolve-Path -LiteralPath $CoreStageDir).Path
}

if ($LauncherStageDir) {
    $launcherStageAbs = (Resolve-Path -LiteralPath $LauncherStageDir).Path
} elseif ($LauncherExe) {
    # Back-compat path: stage the single launcher exe so callers still produce a valid payload tree.
    $tempLauncherStageDir = Join-Path $objDir "launcher-stage"
    New-Item -ItemType Directory -Force -Path $tempLauncherStageDir | Out-Null
    Copy-Item -LiteralPath $LauncherExe -Destination (Join-Path $tempLauncherStageDir "dockpipe-launcher.exe") -Force
    $launcherStageAbs = (Resolve-Path -LiteralPath $tempLauncherStageDir).Path
}

$launcherEnabled = if ($launcherStageAbs) { "1" } else { "0" }
$coreEnabled = if ($coreStageAbs) { "1" } else { "0" }
$coreHarvestWxs = Join-Path $objDir "core-payload.wxs"
$harvestWxs = Join-Path $objDir "launcher-payload.wxs"
$candleInputs = @($wxs)

if ($coreStageAbs) {
    if (-not (Get-ChildItem -LiteralPath $coreStageAbs -Filter "dockpipe-core-*.tar.gz" -File -ErrorAction SilentlyContinue)) {
        throw "CoreStageDir must contain dockpipe-core-*.tar.gz at its root. Got: $coreStageAbs"
    }
    & $heat dir $coreStageAbs `
        -nologo `
        -gg `
        -scom `
        -sreg `
        -sfrag `
        -srd `
        -dr COREPACKAGESDIR `
        -cg CorePayloadComponents `
        -var var.DockpipeCoreStageDir `
        -out $coreHarvestWxs
    if ($LASTEXITCODE -ne 0) { throw "heat failed" }
    $candleInputs += $coreHarvestWxs
}

if ($launcherStageAbs) {
    $launcherExeFromStage = Join-Path $launcherStageAbs "dockpipe-launcher.exe"
    if (-not (Test-Path -LiteralPath $launcherExeFromStage)) {
        throw "LauncherStageDir must contain dockpipe-launcher.exe at its root. Got: $launcherStageAbs"
    }
    & $heat dir $launcherStageAbs `
        -nologo `
        -gg `
        -scom `
        -sreg `
        -sfrag `
        -srd `
        -dr INSTALLFOLDER `
        -cg LauncherPayloadComponents `
        -var var.DockpipeLauncherStageDir `
        -out $harvestWxs
    if ($LASTEXITCODE -ne 0) { throw "heat failed" }
    $candleInputs += $harvestWxs
}

& $candle -nologo -arch x64 `
    "-dProductVersion=$fourPart" `
    "-dDockpipeSource=$srcAbs" `
    "-dDockpipeCoreEnabled=$coreEnabled" `
    "-dDockpipeCoreStageDir=$coreStageAbs" `
    "-dDockpipeLauncherEnabled=$launcherEnabled" `
    "-dDockpipeLauncherStageDir=$launcherStageAbs" `
    -out "$objDir\\" `
    $candleInputs
if ($LASTEXITCODE -ne 0) { throw "candle failed" }

$wixobjPaths = Get-ChildItem -LiteralPath $objDir -Filter "*.wixobj" | Sort-Object Name | Select-Object -ExpandProperty FullName
if (-not $wixobjPaths -or $wixobjPaths.Count -eq 0) {
    throw "No .wixobj files were generated under $objDir"
}

# WiX 3: -arch is for candle only; light treats unknown args as .wixobj paths — "x64" became a bogus Source file (LGHT0103).
& $light -nologo `
    -ext WixUIExtension `
    -sw1076 `
    -sice:ICE38 `
    -sice:ICE64 `
    -out $msiPath `
    $wixobjPaths
if ($LASTEXITCODE -ne 0) { throw "light failed" }

Write-Host "Built: $msiPath"
if ($coreStageAbs) {
    Write-Host "Included core payload: $coreStageAbs"
}
if ($launcherStageAbs) {
    Write-Host "Included launcher payload: $launcherStageAbs"
}
