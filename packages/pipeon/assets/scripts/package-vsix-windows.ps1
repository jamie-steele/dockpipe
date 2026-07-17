#Requires -Version 5.1
param(
    [Parameter(Mandatory = $true)]
    [string]$ExtensionDir,

    [Parameter(Mandatory = $true)]
    [string]$OutputFile,

    [string]$VsceEntrypoint = "",
    [string]$NpmCache = "",
    [int]$TimeoutSeconds = 180
)

$ErrorActionPreference = "Stop"

Write-Host "[pipeon-build] package-vsix-windows: starting vsce package in $ExtensionDir"
$startedAt = Get-Date

if (-not $VsceEntrypoint) {
    $VsceEntrypoint = Join-Path $ExtensionDir "node_modules\@vscode\vsce\vsce"
}

Push-Location $ExtensionDir
try {
    if ($NpmCache) {
        $env:NPM_CONFIG_CACHE = $NpmCache
    }
    $args = @($VsceEntrypoint, "package", "--no-dependencies", "-o", $OutputFile)
    $proc = Start-Process -FilePath "node" -ArgumentList $args -NoNewWindow -PassThru
    if (-not $proc.WaitForExit([Math]::Max(1, $TimeoutSeconds) * 1000)) {
        Write-Warning "[pipeon-build] package-vsix-windows: vsce package timed out after $TimeoutSeconds seconds; stopping node process $($proc.Id)"
        Stop-Process -Id $proc.Id -Force -ErrorAction SilentlyContinue
        Start-Sleep -Milliseconds 250
        $written = Get-Item -LiteralPath $OutputFile -ErrorAction SilentlyContinue
        if ($written -and $written.Length -gt 0 -and $written.LastWriteTime -ge $startedAt.AddSeconds(-2)) {
            Write-Host "[pipeon-build] package-vsix-windows: vsce wrote $OutputFile before timing out; accepting existing VSIX"
            return
        }
        throw "vsce package timed out after $TimeoutSeconds seconds and did not produce $OutputFile"
    }
    if ($proc.ExitCode -ne 0) {
        throw "vsce package failed with exit code $($proc.ExitCode)"
    }
    Write-Host "[pipeon-build] package-vsix-windows: vsce package returned successfully"
}
finally {
    Pop-Location
}
