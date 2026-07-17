param(
    [Parameter(Mandatory = $true)][string]$IconPath,
    [Parameter(Mandatory = $true)][string]$OutFile
)

$ErrorActionPreference = "Stop"

if (-not (Test-Path -LiteralPath $IconPath)) {
    throw "IconPath not found: $IconPath"
}

$goversioninfoExe = (Get-Command goversioninfo.exe -ErrorAction SilentlyContinue)?.Source
if (-not $goversioninfoExe) {
    $fallback = Join-Path $env:USERPROFILE "go\bin\goversioninfo.exe"
    if (Test-Path -LiteralPath $fallback) {
        $goversioninfoExe = $fallback
    }
}
if (-not $goversioninfoExe) {
    $goExe = (Get-Command go.exe -ErrorAction SilentlyContinue)?.Source
    if (-not $goExe -and (Test-Path -LiteralPath "C:\Program Files\Go\bin\go.exe")) {
        $goExe = "C:\Program Files\Go\bin\go.exe"
    }
    if (-not $goExe) {
        throw "goversioninfo.exe not found and go.exe is not available to install it."
    }
    & $goExe install github.com/josephspurrier/goversioninfo/cmd/goversioninfo@latest
    if ($LASTEXITCODE -ne 0) { throw "go install goversioninfo failed" }
    $fallback = Join-Path $env:USERPROFILE "go\bin\goversioninfo.exe"
    if (-not (Test-Path -LiteralPath $fallback)) {
        throw "goversioninfo.exe was installed but not found at $fallback"
    }
    $goversioninfoExe = $fallback
}

$outDir = Split-Path -Parent $OutFile
New-Item -ItemType Directory -Force -Path $outDir | Out-Null

& $goversioninfoExe `
    -64 `
    -skip-versioninfo `
    "-icon=$([IO.Path]::GetFullPath($IconPath))" `
    "-application-icon=$([IO.Path]::GetFullPath($IconPath))" `
    "-o=$OutFile"
if ($LASTEXITCODE -ne 0) { throw "goversioninfo failed" }
