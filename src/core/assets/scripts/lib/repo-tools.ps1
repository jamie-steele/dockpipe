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
  $candidates = @("src/bin/dockpipe", "src/bin/dockpipe.exe")
  if ($IsWindows) {
    $candidates = @("src/bin/dockpipe.exe", "src/bin/dockpipe")
  }
  foreach ($relative in $candidates) {
    $candidate = Join-Path $resolvedRoot $relative
    if (Test-Path -LiteralPath $candidate) {
      return $candidate
    }
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

function Invoke-DockpipeScope {
  param(
    [string]$Scope,
    [string]$Package,
    [Parameter(ValueFromRemainingArguments = $true)]
    [string[]]$Path = @(),
    [string]$Root
  )

  $sdk = Get-DockpipeSdk -Root $Root
  if (-not $sdk.DockpipeBin) {
    throw "dockpipe binary not found; set DOCKPIPE_BIN or add dockpipe to PATH"
  }
  $argv = @("scope", "--workdir", $sdk.Workdir)
  if ($Package) {
    $argv += @("--package", $Package)
  }
  if ($Scope) {
    $argv += $Scope
  }
  foreach ($part in $Path) {
    $argv += $part
  }
  $out = & $sdk.DockpipeBin @argv
  if ($LASTEXITCODE -ne 0) {
    throw "dockpipe scope failed"
  }
  if (-not $Scope -and $Path.Count -eq 0) {
    return ($out | ConvertFrom-Json)
  }
  return $out
}

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
  switch ($env:DOCKPIPE_SDK_PROMPT_MODE) {
    "json" { return "json" }
    "terminal" { return "terminal" }
  }
  if (-not [Console]::IsInputRedirected -and -not [Console]::IsErrorRedirected) {
    return "terminal"
  }
  return "noninteractive"
}

function ConvertTo-DockpipeResourceResponse {
  param(
    [string[]]$Values
  )

  $normalized = @()
  foreach ($value in $Values) {
    if ($null -eq $value) {
      continue
    }
    $trimmed = $value.Trim()
    if ($trimmed.Length -ge 2) {
      if (($trimmed.StartsWith('"') -and $trimmed.EndsWith('"')) -or ($trimmed.StartsWith("'") -and $trimmed.EndsWith("'"))) {
        $trimmed = $trimmed.Substring(1, $trimmed.Length - 2)
      }
    }
    if (-not [string]::IsNullOrWhiteSpace($trimmed)) {
      $normalized += $trimmed
    }
  }
  return $normalized
}

function Test-DockpipeResourceExists {
  param(
    [string]$Value,
    [string]$ResourceKind,
    [string]$BaseDir
  )

  $resolved = $Value
  if (-not [System.IO.Path]::IsPathRooted($resolved)) {
    $base = $(if ($BaseDir) { $BaseDir } else { (Get-Location).Path })
    $resolved = Join-Path $base $resolved
  }
  if ($ResourceKind -eq "directory") {
    return Test-Path -LiteralPath $resolved -PathType Container
  }
  return Test-Path -LiteralPath $resolved -PathType Any
}

function Invoke-DockpipePrompt {
  param(
    [Parameter(Mandatory = $true)]
    [ValidateSet("confirm", "choice", "input", "file", "resource")]
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
    [string]$AutoApproveValue,
    [string]$PathMode = "open-file",
    [string]$FileFilter,
    [switch]$MustExist,
    [ValidateSet("select", "new")]
    [string]$ResourceMode = "select",
    [ValidateSet("single", "multi")]
    [string]$ResourceSelection = "single",
    [string]$ResourceKind = "file",
    [string[]]$Filters = @()
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
    if (($Kind -eq "file" -or $Kind -eq "resource") -and $DefaultValue) {
      return $DefaultValue
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
        path_mode = $PathMode
        file_filter = $FileFilter
        must_exist = [bool]$MustExist
        base_dir = $(if ($env:DOCKPIPE_WORKDIR) { $env:DOCKPIPE_WORKDIR } else { (Get-Location).Path })
        resource_mode = $ResourceMode
        resource_selection = $ResourceSelection
        resource_kind = $ResourceKind
        filters = @($Filters)
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
        "file" {
          while ($true) {
            if ($FileFilter) {
              [Console]::Error.WriteLine(("Filter: {0}" -f $FileFilter))
            }
            if ($DefaultValue) {
              [Console]::Error.Write(("{0} [{1}]: " -f $Message, $DefaultValue))
            } else {
              [Console]::Error.Write(("{0}: " -f $Message))
            }
            $raw = [Console]::In.ReadLine()
            if ([string]::IsNullOrEmpty($raw)) {
              $raw = $DefaultValue
            }
            if (-not $raw) {
              return $raw
            }
            if ($MustExist) {
              $resolved = $raw
              if (-not [System.IO.Path]::IsPathRooted($resolved)) {
                $base = $(if ($env:DOCKPIPE_WORKDIR) { $env:DOCKPIPE_WORKDIR } else { (Get-Location).Path })
                $resolved = Join-Path $base $resolved
              }
              $exists = $(if ($PathMode -eq "open-dir") { Test-Path -LiteralPath $resolved -PathType Container } else { Test-Path -LiteralPath $resolved -PathType Any })
              if (-not $exists) {
                $kindLabel = $(if ($PathMode -eq "open-dir") { "Directory" } else { "File" })
                [Console]::Error.WriteLine(("{0} not found: {1}" -f $kindLabel, $raw))
                continue
              }
            }
            return $raw
          }
        }
        "resource" {
          $resourceFilters = @($Filters | Where-Object { -not [string]::IsNullOrWhiteSpace($_) })
          $filterDisplay = if ($resourceFilters.Count -gt 0) { $resourceFilters -join ";;" } else { $FileFilter }
          while ($true) {
            if ($filterDisplay) {
              [Console]::Error.WriteLine(("Filter: {0}" -f $filterDisplay))
            }
            if ($ResourceSelection -eq "multi") {
              $promptText = "$Message (separate multiple paths with ;)"
            } else {
              $promptText = $Message
            }
            if ($DefaultValue) {
              [Console]::Error.Write(("{0} [{1}]: " -f $promptText, $DefaultValue))
            } else {
              [Console]::Error.Write(("{0}: " -f $promptText))
            }
            $raw = [Console]::In.ReadLine()
            if ([string]::IsNullOrEmpty($raw)) {
              $raw = $DefaultValue
            }
            if (-not $raw) {
              return $raw
            }
            if ($ResourceSelection -eq "multi") {
              $entries = ConvertTo-DockpipeResourceResponse -Values ($raw -split ';')
              if ($entries.Count -eq 0) {
                return "[]"
              }
              if ($MustExist) {
                $valid = $true
                foreach ($entry in $entries) {
                  if (-not (Test-DockpipeResourceExists -Value $entry -ResourceKind $ResourceKind -BaseDir $env:DOCKPIPE_WORKDIR)) {
                    $label = $(if ($ResourceKind -eq "directory") { "Directory" } else { "File" })
                    [Console]::Error.WriteLine(("{0} not found: {1}" -f $label, $entry))
                    $valid = $false
                    break
                  }
                }
                if (-not $valid) {
                  continue
                }
              }
              return ($entries | ConvertTo-Json -Compress)
            }

            $entries = ConvertTo-DockpipeResourceResponse -Values @($raw)
            if ($entries.Count -eq 0) {
              return ""
            }
            $selected = $entries[0]
            if ($MustExist -and -not (Test-DockpipeResourceExists -Value $selected -ResourceKind $ResourceKind -BaseDir $env:DOCKPIPE_WORKDIR)) {
              $label = $(if ($ResourceKind -eq "directory") { "Directory" } else { "File" })
              [Console]::Error.WriteLine(("{0} not found: {1}" -f $label, $selected))
              continue
            }
            return $selected
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
