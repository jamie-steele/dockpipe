param(
    [string]$Version = "",
    [string]$QtVersion = "6.8.3",
    [string]$QtArch = "win64_msvc2022_64",
    [string]$QtRoot = "",
    [string]$QtInstallRoot = "C:\Qt",
    [string]$WixRoot = "C:\tmp\wix314",
    [string]$OutDir = "C:\Source\dockpipe\bin\windows-msi\msi-dist",
    [switch]$InstallQt,
    [switch]$SkipSmokeTest
)

$ErrorActionPreference = "Stop"

function Resolve-RepoPath {
    param([string]$PathValue)
    return [IO.Path]::GetFullPath((Join-Path $PWD $PathValue))
}

function Require-Tool {
    param([string]$Exe, [string]$Hint, [string[]]$Fallbacks = @())
    $cmd = Get-Command $Exe -ErrorAction SilentlyContinue
    if ($cmd) {
        return $cmd.Source
    }
    foreach ($fallback in $Fallbacks) {
        if ($fallback -and (Test-Path -LiteralPath $fallback)) {
            return $fallback
        }
    }
    throw "Required tool not found: $Exe. $Hint"
}

function Ensure-WixRoot {
    param([string]$PathValue)
    if (Test-Path -LiteralPath (Join-Path $PathValue "candle.exe")) {
        return [IO.Path]::GetFullPath($PathValue)
    }
    if (-not (Test-Path -LiteralPath "C:\tmp\wix314-binaries.zip")) {
        Invoke-WebRequest -Uri "https://github.com/wixtoolset/wix3/releases/download/wix3141rtm/wix314-binaries.zip" -OutFile "C:\tmp\wix314-binaries.zip"
    }
    if (Test-Path -LiteralPath $PathValue) {
        Remove-Item -LiteralPath $PathValue -Recurse -Force
    }
    New-Item -ItemType Directory -Force -Path $PathValue | Out-Null
    Expand-Archive -Path "C:\tmp\wix314-binaries.zip" -DestinationPath $PathValue -Force
    return [IO.Path]::GetFullPath($PathValue)
}

function Set-MsvcEnvironment {
    $msvcBases = @(
        "C:\Program Files (x86)\Microsoft Visual Studio\2022\BuildTools\VC\Tools\MSVC",
        "C:\Program Files\Microsoft Visual Studio\2022\BuildTools\VC\Tools\MSVC",
        "C:\Program Files\Microsoft Visual Studio\2022\Professional\VC\Tools\MSVC",
        "C:\Program Files\Microsoft Visual Studio\18\Professional\VC\Tools\MSVC"
    ) | Where-Object { Test-Path -LiteralPath $_ }
    if (-not $msvcBases -or $msvcBases.Count -eq 0) {
        throw "MSVC tools not found under the standard Visual Studio locations."
    }
    $msvcRoot = $null
    foreach ($base in $msvcBases) {
        $candidate = Get-ChildItem $base -ErrorAction SilentlyContinue | Sort-Object Name -Descending | Select-Object -First 1 -ExpandProperty FullName
        if ($candidate -and (Test-Path -LiteralPath (Join-Path $candidate "include"))) {
            $msvcRoot = $candidate
            break
        }
    }
    if (-not $msvcRoot) {
        throw "MSVC tools were found, but none of the installations include the C++ headers. Install the Visual C++ desktop/build tools workload."
    }
    $msvcBin = Join-Path $msvcRoot "bin\Hostx64\x64"
    $msvcInclude = Join-Path $msvcRoot "include"
    $msvcLib = Join-Path $msvcRoot "lib\x64"
    $msvcOnecoreLib = Join-Path $msvcRoot "lib\onecore\x64"
    $atlmfcLib = Join-Path $msvcRoot "atlmfc\lib\x64"

    $sdkBase = "C:\Program Files (x86)\Windows Kits\10"
    $sdkVersion = Get-ChildItem (Join-Path $sdkBase "Include") | Sort-Object Name -Descending | Select-Object -First 1 -ExpandProperty Name
    if (-not $sdkVersion) {
        throw "Windows SDK include directories not found under $sdkBase"
    }
    $sdkIncludeBase = Join-Path $sdkBase "Include\$sdkVersion"
    $sdkLibBase = Join-Path $sdkBase "Lib\$sdkVersion"
    $sdkBinVersioned = Join-Path $sdkBase "bin\$sdkVersion\x64"
    $sdkBinFlat = Join-Path $sdkBase "bin\x64"
    $sdkBin = if (Test-Path -LiteralPath $sdkBinVersioned) { $sdkBinVersioned } else { $sdkBinFlat }

    $includeParts = @(
        $msvcInclude,
        (Join-Path $sdkIncludeBase "ucrt"),
        (Join-Path $sdkIncludeBase "shared"),
        (Join-Path $sdkIncludeBase "um"),
        (Join-Path $sdkIncludeBase "winrt"),
        (Join-Path $sdkIncludeBase "cppwinrt")
    ) | Where-Object { Test-Path -LiteralPath $_ }

    $libParts = @(
        $msvcLib,
        $msvcOnecoreLib,
        $atlmfcLib,
        (Join-Path $sdkLibBase "ucrt\x64"),
        (Join-Path $sdkLibBase "um\x64")
    ) | Where-Object { Test-Path -LiteralPath $_ }

    $env:Path = ($msvcBin, $sdkBin, $env:Path -split ';' | Where-Object { $_ }) -join ';'
    $env:INCLUDE = ($includeParts -join ';')
    $env:LIB = ($libParts -join ';')
    $env:LIBPATH = ($libParts -join ';')
}

if (-not $Version) {
    $Version = (Get-Content (Resolve-RepoPath "VERSION") -Raw).Trim()
}

$goExe = Require-Tool -Exe "go" -Hint "Install Go or ensure it is on PATH." -Fallbacks @("C:\Program Files\Go\bin\go.exe")
$cmakeExe = Require-Tool -Exe "cmake" -Hint "Install CMake or ensure it is on PATH." -Fallbacks @("C:\Program Files\CMake\bin\cmake.exe")
$pythonExe = Require-Tool -Exe "python" -Hint "Install Python or ensure it is on PATH."
$wixRootResolved = Ensure-WixRoot -PathValue $WixRoot
Set-MsvcEnvironment

if (-not $QtRoot) {
    $QtRoot = Join-Path $QtInstallRoot "$QtVersion\msvc2022_64"
}

if (-not (Test-Path -LiteralPath (Join-Path $QtRoot "lib\cmake\Qt6\Qt6Config.cmake"))) {
    if (-not $InstallQt) {
        throw "Qt not found at $QtRoot. Re-run with -InstallQt to download it via aqtinstall."
    }
    & $pythonExe -m pip install --user aqtinstall
    if ($LASTEXITCODE -ne 0) { throw "pip install aqtinstall failed" }
    & $pythonExe -m aqt install-qt windows desktop $QtVersion $QtArch -O $QtInstallRoot
    if ($LASTEXITCODE -ne 0) { throw "aqt install-qt failed" }
}

$outDirResolved = [IO.Path]::GetFullPath($OutDir)
$windowsMsiRoot = Split-Path -Parent $outDirResolved
$dockpipeExe = Join-Path $windowsMsiRoot "dockpipe.exe"
$dockpipeSyso = Resolve-RepoPath "src/cmd/dockpipe_windows_amd64.syso"
$dockpipeIcon = Resolve-RepoPath "src/app/tooling/dockpipe-launcher/resources/images/dockpipe-launcher.ico"
$coreBuildDir = Join-Path $windowsMsiRoot "core-build"
$coreStageDir = Join-Path $windowsMsiRoot "core-stage"
$launcherBuildDir = Join-Path $windowsMsiRoot "launcher-build"
$launcherStageDir = Join-Path $windowsMsiRoot "launcher-stage"
$launcherExe = ""
$windeployqt = Join-Path $QtRoot "bin\windeployqt.exe"

if (-not (Test-Path -LiteralPath $windeployqt)) {
    throw "windeployqt.exe not found at $windeployqt"
}

if (Test-Path -LiteralPath $launcherBuildDir) {
    Remove-Item -LiteralPath $launcherBuildDir -Recurse -Force
}
if (Test-Path -LiteralPath $coreBuildDir) {
    Remove-Item -LiteralPath $coreBuildDir -Recurse -Force
}
if (Test-Path -LiteralPath $coreStageDir) {
    Remove-Item -LiteralPath $coreStageDir -Recurse -Force
}
if (Test-Path -LiteralPath $launcherStageDir) {
    Remove-Item -LiteralPath $launcherStageDir -Recurse -Force
}
if (Test-Path -LiteralPath $outDirResolved) {
    Remove-Item -LiteralPath $outDirResolved -Recurse -Force
}

New-Item -ItemType Directory -Force -Path $windowsMsiRoot | Out-Null
New-Item -ItemType Directory -Force -Path $outDirResolved | Out-Null
New-Item -ItemType Directory -Force -Path $coreBuildDir | Out-Null
New-Item -ItemType Directory -Force -Path $coreStageDir | Out-Null
New-Item -ItemType Directory -Force -Path $launcherStageDir | Out-Null

try {
    & (Resolve-RepoPath "release/packaging/windows/new-go-icon-resource.ps1") -IconPath $dockpipeIcon -OutFile $dockpipeSyso
    if ($LASTEXITCODE -ne 0) { throw "dockpipe icon resource generation failed" }

    & $goExe build -trimpath -ldflags "-s -w -X main.Version=$Version" -o $dockpipeExe .\src\cmd
    if ($LASTEXITCODE -ne 0) { throw "go build failed" }

    & $dockpipeExe package compile core --workdir $coreBuildDir --from (Resolve-RepoPath "src/core") --force
    if ($LASTEXITCODE -ne 0) { throw "dockpipe package compile core failed" }
    & $dockpipeExe package build store --workdir $coreBuildDir --out $coreBuildDir --only core --version $Version
    if ($LASTEXITCODE -ne 0) { throw "dockpipe package build store --only core failed" }
    $coreTarball = Get-ChildItem -LiteralPath $coreBuildDir -Filter "dockpipe-core-*.tar.gz" -File | Select-Object -First 1
    if (-not $coreTarball) { throw "dockpipe core tarball was not produced under $coreBuildDir" }
    Copy-Item -LiteralPath $coreTarball.FullName -Destination (Join-Path $coreStageDir $coreTarball.Name) -Force

    & $cmakeExe -S src/app/tooling/dockpipe-launcher -B $launcherBuildDir -G "NMake Makefiles" "-DCMAKE_PREFIX_PATH=$QtRoot" "-DCMAKE_BUILD_TYPE=Release" "-DCMAKE_POLICY_DEFAULT_CMP0091=NEW" "-DCMAKE_MSVC_RUNTIME_LIBRARY=MultiThreadedDLL"
    if ($LASTEXITCODE -ne 0) { throw "cmake configure failed" }

    & $cmakeExe --build $launcherBuildDir
    if ($LASTEXITCODE -ne 0) { throw "cmake build failed" }

    $launcherCandidates = @(
        (Join-Path $launcherBuildDir "dockpipe-launcher.exe"),
        (Join-Path $launcherBuildDir "Release\dockpipe-launcher.exe")
    ) | Where-Object { Test-Path -LiteralPath $_ }
    $launcherExe = $launcherCandidates | Select-Object -First 1

    if (-not $launcherExe) {
        throw "Built launcher not found under $launcherBuildDir"
    }

    & $windeployqt --release --compiler-runtime --dir $launcherStageDir $launcherExe
    if ($LASTEXITCODE -ne 0) { throw "windeployqt failed" }

    Copy-Item -LiteralPath $launcherExe -Destination (Join-Path $launcherStageDir "dockpipe-launcher.exe") -Force

    if (-not (Test-Path -LiteralPath (Join-Path $launcherStageDir "dockpipe-launcher.exe"))) {
        throw "Launcher stage directory missing dockpipe-launcher.exe"
    }

    & (Resolve-RepoPath "release/packaging/msi/build.ps1") `
        -Version $Version `
        -SourceExe $dockpipeExe `
        -CoreStageDir $coreStageDir `
        -LauncherStageDir $launcherStageDir `
        -OutDir $outDirResolved `
        -WixRoot $wixRootResolved
    if ($LASTEXITCODE -ne 0) { throw "MSI build failed" }

    $msiPath = Join-Path $outDirResolved "dockpipe_${Version}_windows_amd64.msi"
    if (-not (Test-Path -LiteralPath $msiPath)) {
        throw "Expected MSI not found at $msiPath"
    }

    if (-not $SkipSmokeTest) {
        & (Resolve-RepoPath "release/packaging/msi/smoke-test.ps1") -MsiPath $msiPath -ExpectLauncher
        if ($LASTEXITCODE -ne 0) { throw "MSI smoke test failed" }
    }

    Write-Host "Built full launcher MSI: $msiPath"
}
finally {
    if (Test-Path -LiteralPath $dockpipeSyso) {
        Remove-Item -LiteralPath $dockpipeSyso -Force
    }
}
