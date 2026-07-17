param(
  [Parameter(Mandatory = $true)]
  [ValidateSet('start', 'wait', 'stop', 'start-wait')]
  [string]$Action,

  [string]$QemuBin,
  [string]$PidFile,
  [string]$ArgsFile,
  [string]$StdOutFile,
  [string]$StdErrFile
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

function Start-QemuProcess {
  param(
    [Parameter(Mandatory = $true)]
    [string]$FilePath,
    [string[]]$ArgumentList = @(),
    [string]$StdOutPath,
    [string]$StdErrPath,
    [switch]$CaptureOutput
  )

  $psi = [System.Diagnostics.ProcessStartInfo]::new()
  $psi.FileName = $FilePath
  $psi.UseShellExecute = $false

  foreach ($arg in @($ArgumentList)) {
    [void]$psi.ArgumentList.Add($arg)
  }

  if ($CaptureOutput -and -not [string]::IsNullOrWhiteSpace($StdOutPath)) {
    $psi.RedirectStandardOutput = $true
  }
  if ($CaptureOutput -and -not [string]::IsNullOrWhiteSpace($StdErrPath)) {
    $psi.RedirectStandardError = $true
  }

  $process = [System.Diagnostics.Process]::new()
  $process.StartInfo = $psi
  if (-not $process.Start()) {
    throw "Failed to start QEMU process."
  }

  return $process
}

function Wait-QemuProcess {
  param(
    [Parameter(Mandatory = $true)]
    [System.Diagnostics.Process]$Process
  )

  $cancelHandler = [ConsoleCancelEventHandler]{
    param($sender, $eventArgs)
    $eventArgs.Cancel = $true
    try {
      if (-not $Process.HasExited) {
        Stop-Process -Id $Process.Id -Force -ErrorAction SilentlyContinue
      }
    } catch {
    }
  }

  [Console]::add_CancelKeyPress($cancelHandler)
  try {
    $Process.WaitForExit()
  } finally {
    [Console]::remove_CancelKeyPress($cancelHandler)
  }
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

function Stop-QemuProcessGracefully {
  param(
    [Parameter(Mandatory = $true)]
    [System.Diagnostics.Process]$Process,
    [int]$WaitMilliseconds = 15000
  )

  try {
    if ($Process.HasExited) {
      return
    }
  } catch {
    return
  }

  try {
    if ($Process.MainWindowHandle -ne 0) {
      $null = $Process.CloseMainWindow()
      if ($Process.WaitForExit($WaitMilliseconds)) {
        return
      }
    }
  } catch {
  }

  Stop-Process -Id $Process.Id -Force
}

switch ($Action) {
  'start' {
    if ([string]::IsNullOrWhiteSpace($QemuBin)) { throw 'QemuBin is required for start' }
    if ([string]::IsNullOrWhiteSpace($PidFile)) { throw 'PidFile is required for start' }
    $args = Read-QemuArgs -Path $ArgsFile
    $process = Start-QemuProcess -FilePath $QemuBin -ArgumentList $args
    Set-Content -LiteralPath $PidFile -Value $process.Id -NoNewline
    break
  }
  'start-wait' {
    if ([string]::IsNullOrWhiteSpace($QemuBin)) { throw 'QemuBin is required for start-wait' }
    if ([string]::IsNullOrWhiteSpace($PidFile)) { throw 'PidFile is required for start-wait' }
    $args = Read-QemuArgs -Path $ArgsFile
    $process = Start-QemuProcess -FilePath $QemuBin -ArgumentList $args -StdOutPath $StdOutFile -StdErrPath $StdErrFile -CaptureOutput
    Set-Content -LiteralPath $PidFile -Value $process.Id -NoNewline
    Wait-QemuProcess -Process $process
    if (-not [string]::IsNullOrWhiteSpace($StdOutFile)) {
      [System.IO.File]::WriteAllText($StdOutFile, $process.StandardOutput.ReadToEnd())
    }
    if (-not [string]::IsNullOrWhiteSpace($StdErrFile)) {
      [System.IO.File]::WriteAllText($StdErrFile, $process.StandardError.ReadToEnd())
    }
    break
  }
  'wait' {
    $process = Get-QemuProcess -Path $PidFile
    if ($null -ne $process) {
      Wait-QemuProcess -Process $process
    }
    break
  }
  'stop' {
    $process = Get-QemuProcess -Path $PidFile
    if ($null -ne $process) {
      Stop-QemuProcessGracefully -Process $process
    }
    break
  }
}
