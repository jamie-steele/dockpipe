# Build dockpipe MSI (WiX 3.x). Run on Windows.
# Usage: .\build.ps1 -Version 0.6.0 -SourceExe C:\path\dockpipe.exe -OutDir C:\out
param(
    [Parameter(Mandatory = $true)][string]$Version,
    [Parameter(Mandatory = $true)][string]$SourceExe,
    [Parameter(Mandatory = $true)][string]$OutDir
)

$ErrorActionPreference = "Stop"
if (-not (Test-Path -LiteralPath $SourceExe)) {
    throw "SourceExe not found: $SourceExe"
}

# WiX requires Product/@Version as X.Y.Z.W
$fourPart = if ($Version -match '^\d+\.\d+\.\d+$') { "$Version.0" } elseif ($Version -match '^\d+\.\d+\.\d+\.\d+$') { $Version } else {
    throw "Version must be semver like 0.6.0 or 0.6.0.0, got: $Version"
}

$wixRoot = $env:WIX
if (-not $wixRoot -or -not (Test-Path "$wixRoot\bin\candle.exe")) {
    throw "WIX environment variable must point to WiX Toolset v3 root (bin\candle.exe). Install from https://github.com/wixtoolset/wix3/releases"
}

$candle = Join-Path $wixRoot "bin\candle.exe"
$light = Join-Path $wixRoot "bin\light.exe"
$wxs = Join-Path $PSScriptRoot "dockpipe.wxs"
$objDir = Join-Path $OutDir "wixobj"
New-Item -ItemType Directory -Force -Path $objDir | Out-Null
$wixobj = Join-Path $objDir "dockpipe.wixobj"
$msiName = "dockpipe_${Version}_windows_amd64.msi"
$msiPath = Join-Path $OutDir $msiName

$srcAbs = (Resolve-Path -LiteralPath $SourceExe).Path

& $candle -nologo -arch x64 -ext WixUtilExtension `
    "-dProductVersion=$fourPart" `
    "-dDockpipeSource=$srcAbs" `
    -out "$objDir\\" `
    $wxs
if ($LASTEXITCODE -ne 0) { throw "candle failed" }

& $light -nologo -arch x64 -ext WixUtilExtension `
    -sw1076 `
    -out $msiPath `
    $wixobj
if ($LASTEXITCODE -ne 0) { throw "light failed" }

Write-Host "Built: $msiPath"
