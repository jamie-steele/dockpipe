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

function Get-DockpipeSdk {
  param(
    [string]$Root
  )

  $resolvedRoot = Get-DockpipeRepoRoot -Root $Root
  $scriptDir = $env:DOCKPIPE_SCRIPT_DIR
  $assetsDir = $env:DOCKPIPE_ASSETS_DIR
  $packageRoot = $env:DOCKPIPE_PACKAGE_ROOT
  [pscustomobject]@{
    Workdir      = $resolvedRoot
    DockpipeBin  = Resolve-DockpipeBin -Root $resolvedRoot
    WorkflowName = $env:DOCKPIPE_WORKFLOW_NAME
    ScriptDir    = $(if ($scriptDir) { $scriptDir } else { $null })
    PackageRoot  = $(if ($packageRoot) { $packageRoot } else { $null })
    AssetsDir    = $(if ($assetsDir) { $assetsDir } else { $null })
  }
}

$script:dockpipe = Get-DockpipeSdk -Root $env:DOCKPIPE_WORKDIR
