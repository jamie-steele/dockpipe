function Get-DockpipeRepoRoot {
  param(
    [string]$Root
  )

  if (-not $Root) {
    if ($env:DOCKPIPE_WORKDIR) {
      $Root = $env:DOCKPIPE_WORKDIR
    } else {
      $Root = (Get-Location).Path
    }
  }

  return (Resolve-Path -LiteralPath $Root).Path
}

function Resolve-DockpipeBin {
  param(
    [string]$Root
  )

  if ($env:DOCKPIPE_BIN) {
    return $env:DOCKPIPE_BIN
  }

  $resolvedRoot = Get-DockpipeRepoRoot -Root $Root
  $candidate = Join-Path $resolvedRoot "src/bin/dockpipe"
  if (Test-Path -LiteralPath $candidate) {
    return $candidate
  }

  $cmd = Get-Command dockpipe -ErrorAction SilentlyContinue
  if ($cmd) {
    return $cmd.Source
  }

  return $null
}

function Resolve-DorkpipeBin {
  param(
    [string]$Root
  )

  if ($env:DORKPIPE_BIN) {
    return $env:DORKPIPE_BIN
  }

  $resolvedRoot = Get-DockpipeRepoRoot -Root $Root
  $candidate = Join-Path $resolvedRoot "packages/dorkpipe/bin/dorkpipe"
  if (Test-Path -LiteralPath $candidate) {
    return $candidate
  }

  $cmd = Get-Command dorkpipe -ErrorAction SilentlyContinue
  if ($cmd) {
    return $cmd.Source
  }

  return $null
}

function Get-DockpipeSdk {
  param(
    [string]$Root
  )

  $resolvedRoot = Get-DockpipeRepoRoot -Root $Root
  [pscustomobject]@{
    Workdir     = $resolvedRoot
    DockpipeBin = Resolve-DockpipeBin -Root $resolvedRoot
    DorkpipeBin = Resolve-DorkpipeBin -Root $resolvedRoot
    WorkflowName = $env:DOCKPIPE_WORKFLOW_NAME
  }
}

$script:dockpipe = Get-DockpipeSdk -Root $env:DOCKPIPE_WORKDIR
