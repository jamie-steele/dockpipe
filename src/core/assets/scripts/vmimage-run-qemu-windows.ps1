param(
  [Parameter(Mandatory = $true)]
  [ValidateSet('start', 'wait', 'stop', 'start-wait')]
  [string]$Action,

  [string]$QemuBin,
  [string]$PidFile,
  [string]$ArgsFile
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

function Read-QemuArgs {
  param([string]$Path)
  if ([string]::IsNullOrWhiteSpace($Path) -or -not (Test-Path -LiteralPath $Path)) {
    return @()
  }
  return @(Get-Content -LiteralPath $Path)
}

function Get-QemuProcess {
  param([string]$Path)
  if ([string]::IsNullOrWhiteSpace($Path) -or -not (Test-Path -LiteralPath $Path)) {
    return $null
  }
  $pidText = (Get-Content -LiteralPath $Path -Raw).Trim()
  if ([string]::IsNullOrWhiteSpace($pidText)) {
    return $null
  }
  try {
    return Get-Process -Id ([int]$pidText) -ErrorAction Stop
  } catch {
    return $null
  }
}

switch ($Action) {
  'start' {
    if ([string]::IsNullOrWhiteSpace($QemuBin)) { throw 'QemuBin is required for start' }
    if ([string]::IsNullOrWhiteSpace($PidFile)) { throw 'PidFile is required for start' }
    $args = Read-QemuArgs -Path $ArgsFile
    $process = Start-Process -FilePath $QemuBin -ArgumentList $args -PassThru
    Set-Content -LiteralPath $PidFile -Value $process.Id -NoNewline
    break
  }
  'start-wait' {
    if ([string]::IsNullOrWhiteSpace($QemuBin)) { throw 'QemuBin is required for start-wait' }
    if ([string]::IsNullOrWhiteSpace($PidFile)) { throw 'PidFile is required for start-wait' }
    $args = Read-QemuArgs -Path $ArgsFile
    $process = Start-Process -FilePath $QemuBin -ArgumentList $args -PassThru
    Set-Content -LiteralPath $PidFile -Value $process.Id -NoNewline
    Wait-Process -Id $process.Id
    break
  }
  'wait' {
    $process = Get-QemuProcess -Path $PidFile
    if ($null -ne $process) {
      Wait-Process -Id $process.Id
    }
    break
  }
  'stop' {
    $process = Get-QemuProcess -Path $PidFile
    if ($null -ne $process) {
      Stop-Process -Id $process.Id -Force
    }
    break
  }
}
