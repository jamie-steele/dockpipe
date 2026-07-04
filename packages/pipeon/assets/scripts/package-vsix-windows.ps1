#Requires -Version 5.1
param(
    [Parameter(Mandatory = $true)]
    [string]$ExtensionDir,

    [Parameter(Mandatory = $true)]
    [string]$OutputFile,

    [string]$VsceEntrypoint = "",
    [string]$NpmCache = ""
)

$ErrorActionPreference = "Stop"

Write-Host "[pipeon-build] package-vsix-windows: starting vsce package in $ExtensionDir"

if (-not $VsceEntrypoint) {
    $VsceEntrypoint = Join-Path $ExtensionDir "node_modules\@vscode\vsce\vsce"
}

Push-Location $ExtensionDir
try {
    if ($NpmCache) {
        $env:NPM_CONFIG_CACHE = $NpmCache
    }
    & node $VsceEntrypoint package --no-dependencies -o $OutputFile
    if ($LASTEXITCODE -ne 0) {
        throw "vsce package failed with exit code $LASTEXITCODE"
    }
    Write-Host "[pipeon-build] package-vsix-windows: vsce package returned successfully"
}
finally {
    Pop-Location
}
