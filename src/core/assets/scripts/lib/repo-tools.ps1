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

function Test-DockpipeTruthy {
  param(
    [string]$Value
  )

  if (-not $Value) {
    return $false
  }

  switch ($Value.ToLowerInvariant()) {
    "1" { return $true }
    "true" { return $true }
    "yes" { return $true }
    "y" { return $true }
    "on" { return $true }
    default { return $false }
  }
}

function Get-DockpipePromptMode {
  if ($env:DOCKPIPE_SDK_PROMPT_MODE -eq "json") {
    return "json"
  }
  if (-not [Console]::IsInputRedirected -and -not [Console]::IsErrorRedirected) {
    return "terminal"
  }
  return "noninteractive"
}

function Invoke-DockpipePrompt {
  param(
    [Parameter(Mandatory = $true)]
    [ValidateSet("confirm", "choice", "input")]
    [string]$Kind,
    [string]$PromptId,
    [string]$Title,
    [string]$Message,
    [string]$DefaultValue,
    [string[]]$Options = @(),
    [switch]$Sensitive,
    [string]$Intent,
    [string]$AutomationGroup,
    [switch]$AllowAutoApprove,
    [string]$AutoApproveValue
  )

  if (-not $PromptId) {
    $PromptId = "prompt.$([guid]::NewGuid().ToString('N'))"
  }
  if (-not $Message) {
    $Message = $Title
  }

  if ($AllowAutoApprove -and (Test-DockpipeTruthy $env:DOCKPIPE_APPROVE_PROMPTS)) {
    if ($AutoApproveValue) {
      return $AutoApproveValue
    }
    if ($DefaultValue) {
      return $DefaultValue
    }
    if ($Kind -eq "confirm") {
      return "yes"
    }
    if ($Kind -eq "choice" -and $Options.Count -gt 0) {
      return $Options[0]
    }
  }

  $mode = Get-DockpipePromptMode
  switch ($mode) {
    "json" {
      $payload = [ordered]@{
        type      = $Kind
        id        = $PromptId
        title     = $Title
        message   = $Message
        default   = $DefaultValue
        sensitive = [bool]$Sensitive
        intent    = $Intent
        automation_group = $AutomationGroup
        allow_auto_approve = [bool]$AllowAutoApprove
        auto_approve_value = $AutoApproveValue
        options   = @($Options)
      } | ConvertTo-Json -Compress
      [Console]::Error.WriteLine("::dockpipe-prompt::$payload")
      return [Console]::In.ReadLine()
    }
    "terminal" {
      if ($Title) {
        [Console]::Error.WriteLine($Title)
      }
      switch ($Kind) {
        "confirm" {
          $defaultRaw = ""
          if ($null -ne $DefaultValue) {
            $defaultRaw = $DefaultValue
          }
          $defaultYes = @("1","true","yes","y","on") -contains $defaultRaw.ToLowerInvariant()
          while ($true) {
            $suffix = $(if ($defaultYes) { " [Y/n] " } else { " [y/N] " })
            [Console]::Error.Write("$Message$suffix")
            $raw = [Console]::In.ReadLine()
            if ([string]::IsNullOrWhiteSpace($raw)) {
              return $(if ($defaultYes) { "yes" } else { "no" })
            }
            switch ($raw.Trim().ToLowerInvariant()) {
              "y" { return "yes" }
              "yes" { return "yes" }
              "n" { return "no" }
              "no" { return "no" }
            }
            [Console]::Error.WriteLine("Please answer yes or no.")
          }
        }
        "input" {
          if ($Sensitive) {
            return Read-Host -Prompt $Message -MaskInput
          }
          if ($DefaultValue) {
            $raw = Read-Host -Prompt "$Message [$DefaultValue]"
            if ([string]::IsNullOrEmpty($raw)) {
              return $DefaultValue
            }
            return $raw
          }
          return Read-Host -Prompt $Message
        }
        "choice" {
          if (-not $Options -or $Options.Count -eq 0) {
            throw "DockPipe prompt choice requires at least one option."
          }
          [Console]::Error.WriteLine($Message)
          $defaultIndex = 1
          for ($i = 0; $i -lt $Options.Count; $i++) {
            if ($DefaultValue -and $Options[$i] -eq $DefaultValue) {
              $defaultIndex = $i + 1
            }
            [Console]::Error.WriteLine(("  {0}. {1}" -f ($i + 1), $Options[$i]))
          }
          while ($true) {
            [Console]::Error.Write(("Choose an option [{0}]: " -f $defaultIndex))
            $raw = [Console]::In.ReadLine()
            if ([string]::IsNullOrWhiteSpace($raw)) {
              return $Options[$defaultIndex - 1]
            }
            $selected = 0
            if ([int]::TryParse($raw.Trim(), [ref]$selected) -and $selected -ge 1 -and $selected -le $Options.Count) {
              return $Options[$selected - 1]
            }
            [Console]::Error.WriteLine(("Enter a number between 1 and {0}." -f $Options.Count))
          }
        }
      }
    }
    default {
      if ($DefaultValue) {
        return $DefaultValue
      }
      throw "DockPipe prompt requires a terminal or DOCKPIPE_SDK_PROMPT_MODE=json."
    }
  }
}
