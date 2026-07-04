param(
  [string]$UserName = "dockpipe",
  [string]$PasswordPlain = "",
  [string]$AuthorizedKey = "",
  [bool]$CreateUser = $true,
  [bool]$GrantAdministrators = $false,
  [bool]$EnablePasswordAuth = $true,
  [bool]$EnablePublicKeyAuth = $true,
  [string]$DefaultShell = "C:\Windows\System32\WindowsPowerShell\v1.0\powershell.exe",
  [bool]$InstallDockPipeAgent = $true,
  [int]$AgentPort = 47831
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

function Write-Step {
  param([string]$Message)
  Write-Host "[dockpipe windows-vm] $Message"
}

function Assert-Administrator {
  $identity = [Security.Principal.WindowsIdentity]::GetCurrent()
  $principal = [Security.Principal.WindowsPrincipal]::new($identity)
  if (-not $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) {
    throw "Run this script from an elevated PowerShell session."
  }
}

function Ensure-OpenSSHServer {
  $capability = Get-WindowsCapability -Online | Where-Object Name -like "OpenSSH.Server*"
  if ($null -eq $capability) {
    throw "OpenSSH.Server capability was not found on this Windows image."
  }
  if ($capability.State -ne "Installed") {
    Write-Step "Installing OpenSSH Server capability"
    Add-WindowsCapability -Online -Name $capability.Name | Out-Null
  } else {
    Write-Step "OpenSSH Server capability already installed"
  }
}

function Ensure-SshConfigFile {
  $sshRoot = Join-Path $env:ProgramData "ssh"
  $configPath = Join-Path $sshRoot "sshd_config"
  if (Test-Path -LiteralPath $configPath) {
    return $configPath
  }

  New-Item -ItemType Directory -Force -Path $sshRoot | Out-Null

  $defaultCandidates = @(
    (Join-Path $env:WINDIR "System32\OpenSSH\sshd_config_default"),
    (Join-Path $env:WINDIR "SysNative\OpenSSH\sshd_config_default"),
    (Join-Path $env:WINDIR "System32\OpenSSH\sshd_config")
  )

  foreach ($candidate in $defaultCandidates) {
    if (Test-Path -LiteralPath $candidate) {
      Write-Step "Bootstrapping sshd_config from $candidate"
      Copy-Item -LiteralPath $candidate -Destination $configPath -Force
      return $configPath
    }
  }

  Write-Step "Bootstrapping minimal sshd_config"
  @(
    "Port 22"
    "PasswordAuthentication yes"
    "PubkeyAuthentication yes"
    "Subsystem sftp sftp-server.exe"
  ) | Set-Content -LiteralPath $configPath
  return $configPath
}

function Ensure-LocalUser {
  param(
    [string]$Name,
    [string]$PlainPassword,
    [bool]$AddToAdministrators
  )

  $user = Get-LocalUser -Name $Name -ErrorAction SilentlyContinue
  if ($null -eq $user) {
    if ([string]::IsNullOrWhiteSpace($PlainPassword)) {
      throw "User '$Name' does not exist. Supply -PasswordPlain to create it."
    }
    Write-Step "Creating local user '$Name'"
    $secure = ConvertTo-SecureString -String $PlainPassword -AsPlainText -Force
    $user = New-LocalUser -Name $Name -Password $secure -PasswordNeverExpires -AccountNeverExpires
  } elseif (-not [string]::IsNullOrWhiteSpace($PlainPassword)) {
    Write-Step "Resetting password for local user '$Name'"
    $secure = ConvertTo-SecureString -String $PlainPassword -AsPlainText -Force
    $user | Set-LocalUser -Password $secure
  } else {
    Write-Step "Local user '$Name' already exists"
  }

  if ($AddToAdministrators) {
    $member = Get-LocalGroupMember -Group "Administrators" -ErrorAction SilentlyContinue | Where-Object Name -match "\\$([regex]::Escape($Name))$"
    if ($null -eq $member) {
      Write-Step "Adding '$Name' to Administrators"
      Add-LocalGroupMember -Group "Administrators" -Member $Name
    }
  }
}

function Ensure-AuthorizedKey {
  param(
    [string]$Name,
    [string]$Key
  )

  if ([string]::IsNullOrWhiteSpace($Key)) {
    return
  }

  $profileRoot = Join-Path $env:SystemDrive "Users\$Name"
  if (-not (Test-Path -LiteralPath $profileRoot)) {
    Write-Step "Creating profile folder for '$Name'"
    New-Item -ItemType Directory -Force -Path $profileRoot | Out-Null
  }

  $sshDir = Join-Path $profileRoot ".ssh"
  $authKeys = Join-Path $sshDir "authorized_keys"
  New-Item -ItemType Directory -Force -Path $sshDir | Out-Null

  $keys = @()
  if (Test-Path -LiteralPath $authKeys) {
    $keys = @(Get-Content -LiteralPath $authKeys)
  }
  if ($keys -notcontains $Key) {
    Write-Step "Adding authorized key for '$Name'"
    Add-Content -LiteralPath $authKeys -Value $Key
  }

  $userAcl = "${env:COMPUTERNAME}\${Name}"
  & icacls $sshDir /inheritance:r | Out-Null
  & icacls $sshDir /grant:r "${userAcl}:(OI)(CI)F" "SYSTEM:(OI)(CI)F" | Out-Null
  & icacls $authKeys /inheritance:r | Out-Null
  & icacls $authKeys /grant:r "${userAcl}:F" "SYSTEM:F" | Out-Null
}

function Set-SshdOption {
  param(
    [string]$ConfigPath,
    [string]$Name,
    [string]$Value
  )

  $content = Get-Content -LiteralPath $ConfigPath
  $pattern = "^\s*#?\s*$([regex]::Escape($Name))\s+.*$"
  $replacement = "$Name $Value"
  $updated = $false
  for ($i = 0; $i -lt $content.Count; $i++) {
    if ($content[$i] -match $pattern) {
      $content[$i] = $replacement
      $updated = $true
      break
    }
  }
  if (-not $updated) {
    $content += $replacement
  }
  Set-Content -LiteralPath $ConfigPath -Value $content
}

function Ensure-SshFirewallRule {
  $rule = Get-NetFirewallRule -Name "OpenSSH-Server-In-TCP" -ErrorAction SilentlyContinue
  if ($null -eq $rule) {
    Write-Step "Creating inbound firewall rule for TCP 22"
    New-NetFirewallRule -Name "OpenSSH-Server-In-TCP" -DisplayName "OpenSSH Server (TCP 22)" -Enabled True -Direction Inbound -Protocol TCP -Action Allow -LocalPort 22 | Out-Null
  } else {
    Write-Step "OpenSSH firewall rule already present"
  }
}

function Resolve-DockPipeAgentSourcePath {
  $searchRoots = @()
  if ($PSCommandPath) {
    $searchRoots += (Split-Path -Parent $PSCommandPath)
  }
  $searchRoots += (Get-Location).Path
  $searchRoots = $searchRoots | Where-Object { -not [string]::IsNullOrWhiteSpace($_) } | Select-Object -Unique

  foreach ($root in $searchRoots) {
    $candidate = Join-Path $root "dockpipe-guest-agent.exe"
    if (Test-Path -LiteralPath $candidate) {
      return (Resolve-Path -LiteralPath $candidate).Path
    }
    $candidate = Join-Path $root "dockpipe-guest-agent.ps1"
    if (Test-Path -LiteralPath $candidate) {
      return (Resolve-Path -LiteralPath $candidate).Path
    }
  }

  throw "dockpipe-guest-agent.exe or dockpipe-guest-agent.ps1 was not found beside the provisioning script or in the current directory."
}

function Ensure-DockPipeAgent {
  param(
    [int]$Port
  )

  $agentRoot = Join-Path $env:ProgramData "DockPipe\GuestAgent"
  $agentExe = Join-Path $agentRoot "dockpipe-guest-agent.exe"
  $agentScript = Join-Path $agentRoot "dockpipe-guest-agent.ps1"
  $configPath = Join-Path $agentRoot "config.json"
  $sourceAgent = Resolve-DockPipeAgentSourcePath
  New-Item -ItemType Directory -Force -Path $agentRoot | Out-Null
  $agentExtension = [IO.Path]::GetExtension($sourceAgent)
  if ($agentExtension -ieq ".exe") {
    Copy-Item -LiteralPath $sourceAgent -Destination $agentExe -Force
    if (Test-Path -LiteralPath $agentScript) {
      Remove-Item -LiteralPath $agentScript -Force -ErrorAction SilentlyContinue
    }
  } else {
    Copy-Item -LiteralPath $sourceAgent -Destination $agentScript -Force
  }

  @{
    port = $Port
    bind_address = "127.0.0.1"
    state_path = (Join-Path $agentRoot "state.json")
    service_account = "LocalSystem"
  } | ConvertTo-Json -Depth 4 | Set-Content -LiteralPath $configPath

  $taskName = "DockPipeGuestAgent"
  $pwshPath = Join-Path $env:WINDIR "System32\WindowsPowerShell\v1.0\powershell.exe"
  $existingTask = Get-ScheduledTask -TaskName $taskName -ErrorAction SilentlyContinue
  if ($existingTask) {
    Write-Step "Removing existing DockPipe guest agent scheduled task"
    Unregister-ScheduledTask -TaskName $taskName -Confirm:$false
  }

  Get-CimInstance Win32_Process -Filter "Name = 'dockpipe-guest-agent.exe'" -ErrorAction SilentlyContinue |
    ForEach-Object {
      Write-Step "Stopping existing DockPipe guest agent process $($_.ProcessId)"
      Stop-Process -Id $_.ProcessId -Force -ErrorAction SilentlyContinue
    }

  Get-CimInstance Win32_Process -Filter "Name = 'powershell.exe'" -ErrorAction SilentlyContinue |
    Where-Object { $_.CommandLine -like "*dockpipe-guest-agent.ps1*" } |
    ForEach-Object {
      Write-Step "Stopping existing DockPipe guest agent process $($_.ProcessId)"
      Stop-Process -Id $_.ProcessId -Force -ErrorAction SilentlyContinue
    }

  if (Test-Path -LiteralPath $agentExe) {
    $action = New-ScheduledTaskAction -Execute $agentExe -Argument ('-service -ConfigPath "{0}"' -f $configPath)
    Write-Step "Installing DockPipe guest agent startup task as LocalSystem (native executable)"
  } else {
    $action = New-ScheduledTaskAction -Execute $pwshPath -Argument ('-NoProfile -ExecutionPolicy Bypass -WindowStyle Hidden -File "{0}" -Service -ConfigPath "{1}"' -f $agentScript, $configPath)
    Write-Step "Installing DockPipe guest agent startup task as LocalSystem (PowerShell fallback)"
  }
  $trigger = New-ScheduledTaskTrigger -AtStartup
  $principal = New-ScheduledTaskPrincipal -UserId "SYSTEM" -LogonType ServiceAccount -RunLevel Highest
  $settings = New-ScheduledTaskSettingsSet -AllowStartIfOnBatteries -DontStopIfGoingOnBatteries -MultipleInstances IgnoreNew -StartWhenAvailable

  Register-ScheduledTask -TaskName $taskName -Action $action -Trigger $trigger -Principal $principal -Settings $settings | Out-Null

  Write-Step "Starting DockPipe guest agent startup task"
  Start-ScheduledTask -TaskName $taskName
  Start-Sleep -Seconds 2
  $task = Get-ScheduledTask -TaskName $taskName
  Write-Step "DockPipe guest agent task state: $($task.State) on port $Port"
}

Assert-Administrator
Ensure-OpenSSHServer

if ($CreateUser) {
  Ensure-LocalUser -Name $UserName -PlainPassword $PasswordPlain -AddToAdministrators $GrantAdministrators
}

$sshdConfig = Ensure-SshConfigFile
Set-SshdOption -ConfigPath $sshdConfig -Name "PasswordAuthentication" -Value ($(if ($EnablePasswordAuth) { "yes" } else { "no" }))
Set-SshdOption -ConfigPath $sshdConfig -Name "PubkeyAuthentication" -Value ($(if ($EnablePublicKeyAuth) { "yes" } else { "no" }))

if (-not [string]::IsNullOrWhiteSpace($DefaultShell)) {
  Write-Step "Setting DefaultShell to $DefaultShell"
  New-Item -Path "HKLM:\SOFTWARE\OpenSSH" -Force | Out-Null
  New-ItemProperty -Path "HKLM:\SOFTWARE\OpenSSH" -Name "DefaultShell" -Value $DefaultShell -PropertyType String -Force | Out-Null
}

Ensure-SshFirewallRule

if ($EnablePublicKeyAuth -and -not [string]::IsNullOrWhiteSpace($AuthorizedKey)) {
  Ensure-AuthorizedKey -Name $UserName -Key $AuthorizedKey
}

if ($InstallDockPipeAgent) {
  Ensure-DockPipeAgent -Port $AgentPort
}

Write-Step "Configuring sshd service startup"
Set-Service -Name sshd -StartupType Automatic
Start-Service -Name sshd

Write-Step "Configuring ssh-agent service startup"
Set-Service -Name ssh-agent -StartupType Manual

$status = Get-Service -Name sshd
Write-Step "sshd status: $($status.Status)"
Write-Step "Provisioning complete. Test with: ssh $UserName@localhost"
