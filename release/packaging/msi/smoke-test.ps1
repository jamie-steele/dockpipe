# Validates the per-user DockPipe MSI install/uninstall path on Windows.
param(
    [Parameter(Mandatory = $true)][string]$MsiPath,
    [switch]$ExpectLauncher
)

$ErrorActionPreference = "Stop"

if (-not (Test-Path -LiteralPath $MsiPath)) {
    throw "MSI not found: $MsiPath"
}

$installDir = Join-Path $env:LOCALAPPDATA "dockpipe"
$exePath = Join-Path $installDir "dockpipe.exe"
$corePackageDir = Join-Path $installDir "packages\core"
$launcherPath = Join-Path $installDir "dockpipe-launcher.exe"
$launcherPlatformPlugin = Join-Path $installDir "platforms\qwindows.dll"
$launcherCoreDll = Join-Path $installDir "Qt6Core.dll"
$logPath = Join-Path $env:TEMP "dockpipe-msi-smoke.log"
$startMenuShortcut = Join-Path $env:APPDATA "Microsoft\Windows\Start Menu\Programs\DockPipe\DockPipe Launcher.lnk"

function Get-UserPathValue {
    return [Environment]::GetEnvironmentVariable("Path", "User")
}

function Get-DirectorySnapshot {
    param([string]$PathValue)

    if (-not (Test-Path -LiteralPath $PathValue)) {
        return @()
    }

    $root = [IO.Path]::GetFullPath($PathValue).TrimEnd('\')
    $items = Get-ChildItem -LiteralPath $PathValue -Recurse -Force -ErrorAction SilentlyContinue
    $snapshot = @()
    foreach ($item in $items) {
        $full = [IO.Path]::GetFullPath($item.FullName)
        $relative = $full.Substring($root.Length).TrimStart('\')
        if (-not [string]::IsNullOrWhiteSpace($relative)) {
            $snapshot += $relative.Replace('\', '/')
        }
    }
    return @($snapshot | Sort-Object -Unique)
}

function Test-PathEntryPresent {
    param([string]$PathValue, [string]$Entry)

    $normalizedEntry = $Entry.TrimEnd('\')
    $parts = @($PathValue -split ';' | Where-Object { -not [string]::IsNullOrWhiteSpace($_) })
    foreach ($part in $parts) {
        if ($part.TrimEnd('\') -ieq $normalizedEntry) {
            return $true
        }
    }
    return $false
}

$baselineUserPath = Get-UserPathValue
$baselineCoreSnapshot = Get-DirectorySnapshot -PathValue $corePackageDir
$baselineInstallDirInPath = Test-PathEntryPresent -PathValue $baselineUserPath -Entry $installDir
$baselineExePresent = Test-Path -LiteralPath $exePath
$baselineLauncherPresent = Test-Path -LiteralPath $launcherPath
$baselineLauncherPlatformPluginPresent = Test-Path -LiteralPath $launcherPlatformPlugin
$baselineLauncherCoreDllPresent = Test-Path -LiteralPath $launcherCoreDll
$baselineStartMenuShortcutPresent = Test-Path -LiteralPath $startMenuShortcut

if ($baselineExePresent) {
    throw "Smoke test requires $exePath to be absent before install. Remove the existing install or use a clean profile."
}
if ($baselineLauncherPresent) {
    throw "Smoke test requires $launcherPath to be absent before install. Remove the existing launcher install or use a clean profile."
}
if ($baselineLauncherPlatformPluginPresent) {
    throw "Smoke test requires $launcherPlatformPlugin to be absent before install. Remove the existing launcher payload or use a clean profile."
}
if ($baselineLauncherCoreDllPresent) {
    throw "Smoke test requires $launcherCoreDll to be absent before install. Remove the existing launcher payload or use a clean profile."
}
if ($baselineStartMenuShortcutPresent) {
    throw "Smoke test requires $startMenuShortcut to be absent before install. Remove the existing launcher shortcut or use a clean profile."
}

Write-Host "Installing $MsiPath"
$install = Start-Process msiexec.exe -ArgumentList @("/i", "`"$MsiPath`"", "/qn", "/norestart", "/l*v", "`"$logPath`"") -Wait -PassThru
if ($install.ExitCode -ne 0 -and $install.ExitCode -ne 3010) {
    throw "MSI install failed with exit code $($install.ExitCode)"
}

if (-not (Test-Path -LiteralPath $exePath)) {
    throw "Installed dockpipe.exe not found at $exePath"
}
if (-not (Get-ChildItem -LiteralPath $corePackageDir -Filter "dockpipe-core-*.tar.gz" -File -ErrorAction SilentlyContinue | Select-Object -First 1)) {
    throw "Installed dockpipe core package not found under $corePackageDir"
}
if ($ExpectLauncher) {
    if (-not (Test-Path -LiteralPath $launcherPath)) {
        throw "Installed dockpipe-launcher.exe not found at $launcherPath"
    }
    if (-not (Test-Path -LiteralPath $launcherPlatformPlugin)) {
        throw "Qt platform plugin not found at $launcherPlatformPlugin"
    }
    if (-not (Test-Path -LiteralPath $launcherCoreDll)) {
        throw "Qt6Core.dll not found at $launcherCoreDll"
    }
    if (-not (Test-Path -LiteralPath $startMenuShortcut)) {
        throw "DockPipe Launcher shortcut not found at $startMenuShortcut"
    }
}

$userPath = Get-UserPathValue
if (-not (Test-PathEntryPresent -PathValue $userPath -Entry $installDir)) {
    throw "User PATH does not contain $installDir after install"
}

Write-Host "Running installed executable"
& $exePath '--help' | Out-Null
if ($LASTEXITCODE -ne 0) {
    throw "Installed dockpipe.exe --help exited $LASTEXITCODE"
}

Write-Host "Uninstalling MSI"
$uninstall = Start-Process msiexec.exe -ArgumentList @("/x", "`"$MsiPath`"", "/qn", "/norestart") -Wait -PassThru
if ($uninstall.ExitCode -ne 0 -and $uninstall.ExitCode -ne 3010) {
    throw "MSI uninstall failed with exit code $($uninstall.ExitCode)"
}

if (Test-Path -LiteralPath $exePath) {
    throw "Installed dockpipe.exe still exists after uninstall: $exePath"
}
$currentCoreSnapshot = Get-DirectorySnapshot -PathValue $corePackageDir
if (@($currentCoreSnapshot) -join "`n" -ne @($baselineCoreSnapshot) -join "`n") {
    throw "Installed dockpipe core package contents did not return to baseline after uninstall: $corePackageDir"
}
if ($baselineCoreSnapshot.Count -eq 0 -and (Test-Path -LiteralPath $corePackageDir)) {
    throw "Installed dockpipe core package directory still exists after uninstall: $corePackageDir"
}
if ($ExpectLauncher -and (Test-Path -LiteralPath $launcherPath)) {
    throw "Installed dockpipe-launcher.exe still exists after uninstall: $launcherPath"
}
if ($ExpectLauncher -and (Test-Path -LiteralPath $launcherPlatformPlugin)) {
    throw "Qt platform plugin still exists after uninstall: $launcherPlatformPlugin"
}
if ($ExpectLauncher -and (Test-Path -LiteralPath $launcherCoreDll)) {
    throw "Qt6Core.dll still exists after uninstall: $launcherCoreDll"
}
if ($ExpectLauncher -and (Test-Path -LiteralPath $startMenuShortcut)) {
    throw "DockPipe Launcher shortcut still exists after uninstall: $startMenuShortcut"
}

$userPathAfter = Get-UserPathValue
if (-not $baselineInstallDirInPath -and (Test-PathEntryPresent -PathValue $userPathAfter -Entry $installDir)) {
    throw "User PATH still contains $installDir after uninstall"
}

Write-Host "MSI smoke test passed"
