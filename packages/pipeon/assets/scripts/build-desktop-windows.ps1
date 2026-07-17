param(
  [Parameter(Mandatory = $true)]
  [string]$RepoRoot,

  [Parameter(Mandatory = $true)]
  [string]$TargetDir
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

function Find-VcVars64 {
  $vswhereCandidates = @(
    (Join-Path ${env:ProgramFiles(x86)} 'Microsoft Visual Studio\Installer\vswhere.exe'),
    (Join-Path $env:ProgramFiles 'Microsoft Visual Studio\Installer\vswhere.exe')
  ) | Where-Object { $_ -and (Test-Path -LiteralPath $_) }

  foreach ($vswhere in $vswhereCandidates) {
    $installPath = & $vswhere -latest -products * -requires Microsoft.VisualStudio.Component.VC.Tools.x86.x64 -property installationPath 2>$null
    if ($LASTEXITCODE -eq 0 -and $installPath) {
      $vcvars = Join-Path $installPath 'VC\Auxiliary\Build\vcvars64.bat'
      if (Test-Path -LiteralPath $vcvars) {
        return $vcvars
      }
    }
  }

  $candidates = @(
    'C:\Program Files\Microsoft Visual Studio\2022\BuildTools\VC\Auxiliary\Build\vcvars64.bat',
    'C:\Program Files\Microsoft Visual Studio\2022\Professional\VC\Auxiliary\Build\vcvars64.bat',
    'C:\Program Files\Microsoft Visual Studio\2022\Community\VC\Auxiliary\Build\vcvars64.bat',
    'C:\Program Files\Microsoft Visual Studio\2022\Enterprise\VC\Auxiliary\Build\vcvars64.bat',
    'C:\Program Files\Microsoft Visual Studio\2022\Preview\VC\Auxiliary\Build\vcvars64.bat'
  )
  foreach ($candidate in $candidates) {
    if (Test-Path -LiteralPath $candidate) {
      return $candidate
    }
  }

  throw 'Could not locate vcvars64.bat. Install Visual Studio C++ build tools and rerun.'
}

function Resolve-CargoExe {
  $cmd = Get-Command cargo.exe -ErrorAction SilentlyContinue
  if ($cmd) {
    return $cmd.Source
  }
  $cargoCandidates = @(
    (Join-Path $HOME '.cargo\bin\cargo.exe'),
    (Join-Path $env:USERPROFILE '.cargo\bin\cargo.exe')
  ) | Where-Object { $_ -and (Test-Path -LiteralPath $_) }
  if ($cargoCandidates.Count -gt 0) {
    return $cargoCandidates[0]
  }
  throw 'cargo.exe not found. Install Rust with rustup and rerun.'
}

function Invoke-WithVcVars {
  param(
    [Parameter(Mandatory = $true)]
    [string]$VcVarsPath,

    [Parameter(Mandatory = $true)]
    [string]$Command
  )

  $cmdLine = 'call "{0}" >nul && {1}' -f $VcVarsPath, $Command
  & cmd.exe /d /s /c $cmdLine
  if ($LASTEXITCODE -ne 0) {
    throw "Command failed under vcvars64.bat: $Command"
  }
}

$repoRootResolved = (Resolve-Path -LiteralPath $RepoRoot).Path
$targetDirResolved = $TargetDir
if (-not (Test-Path -LiteralPath $targetDirResolved)) {
  New-Item -ItemType Directory -Path $targetDirResolved -Force | Out-Null
}
$targetDirResolved = (Resolve-Path -LiteralPath $targetDirResolved).Path

$vcvars = Find-VcVars64
$cargoExe = Resolve-CargoExe
$manifestPath = Join-Path $repoRootResolved 'packages\pipeon\apps\pipeon-desktop\src-tauri\Cargo.toml'
$desktopBinDir = Join-Path $repoRootResolved 'packages\pipeon\apps\pipeon-desktop\bin'
New-Item -ItemType Directory -Path $desktopBinDir -Force | Out-Null

$cargoCmd = 'set "CARGO_TARGET_DIR={0}" && "{1}" build --manifest-path "{2}" --release' -f `
  $targetDirResolved, $cargoExe, $manifestPath

Invoke-WithVcVars -VcVarsPath $vcvars -Command $cargoCmd

$builtExe = Join-Path $targetDirResolved 'release\pipeon-desktop.exe'
if (-not (Test-Path -LiteralPath $builtExe)) {
  throw "Expected built desktop binary at $builtExe"
}

Copy-Item -LiteralPath $builtExe -Destination (Join-Path $desktopBinDir 'pipeon-desktop.exe') -Force
