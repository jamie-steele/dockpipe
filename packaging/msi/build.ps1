# Build dockpipe MSI (WiX 3.x). Run on Windows.
# Usage: .\build.ps1 -Version 0.6.0 -SourceExe C:\path\dockpipe.exe -OutDir C:\out
param(
    [Parameter(Mandatory = $true)][string]$Version,
    [Parameter(Mandatory = $true)][string]$SourceExe,
    [Parameter(Mandatory = $true)][string]$OutDir,
    # Optional: WiX root (extract folder: root candle.exe from wix314-binaries.zip, or parent of bin\candle.exe). Prefer passing this in CI — GITHUB_ENV can mangle Windows paths.
    [Parameter(Mandatory = $false)][string]$WixRoot = ""
)

$ErrorActionPreference = "Stop"
if (-not (Test-Path -LiteralPath $SourceExe)) {
    throw "SourceExe not found: $SourceExe"
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
$wxs = Join-Path $PSScriptRoot "dockpipe.wxs"
$objDir = Join-Path $OutDir "wixobj"
New-Item -ItemType Directory -Force -Path $objDir | Out-Null
$wixobj = Join-Path $objDir "dockpipe.wixobj"
$msiName = "dockpipe_${Version}_windows_amd64.msi"
$msiPath = Join-Path $OutDir $msiName

$srcAbs = (Resolve-Path -LiteralPath $SourceExe).Path

& $candle -nologo -arch x64 `
    "-dProductVersion=$fourPart" `
    "-dDockpipeSource=$srcAbs" `
    -out "$objDir\\" `
    $wxs
if ($LASTEXITCODE -ne 0) { throw "candle failed" }

& $light -nologo -arch x64 `
    -sw1076 `
    -out $msiPath `
    $wixobj
if ($LASTEXITCODE -ne 0) { throw "light failed" }

Write-Host "Built: $msiPath"
