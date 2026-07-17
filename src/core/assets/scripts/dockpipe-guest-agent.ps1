param(
  [switch]$Service,
  [string]$ConfigPath = ""
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

function Get-AgentRoot {
  if (-not [string]::IsNullOrWhiteSpace($ConfigPath)) {
    return Split-Path -Parent $ConfigPath
  }
  return (Join-Path $env:ProgramData "DockPipe\GuestAgent")
}

function Get-AgentConfigPath {
  if (-not [string]::IsNullOrWhiteSpace($ConfigPath)) {
    return $ConfigPath
  }
  return (Join-Path (Get-AgentRoot) "config.json")
}

function Ensure-AgentRoot {
  $root = Get-AgentRoot
  New-Item -ItemType Directory -Force -Path $root | Out-Null
  return $root
}

function Write-AgentLog {
  param([string]$Message)
  $root = Ensure-AgentRoot
  $logPath = Join-Path $root "agent.log"
  $line = "{0} {1}" -f ([DateTimeOffset]::Now.ToString("o")), $Message
  Add-Content -LiteralPath $logPath -Value $line
}

function Get-AgentConfig {
  $path = Get-AgentConfigPath
  if (-not (Test-Path -LiteralPath $path)) {
    return [pscustomobject]@{
      port = 47831
      bind_address = "127.0.0.1"
      state_path = (Join-Path (Ensure-AgentRoot) "state.json")
      service_account = [Security.Principal.WindowsIdentity]::GetCurrent().Name
    }
  }
  return (Get-Content -Raw -LiteralPath $path | ConvertFrom-Json)
}

function Save-AgentState {
  param([hashtable]$State)
  $config = Get-AgentConfig
  $statePath = $config.state_path
  if ([string]::IsNullOrWhiteSpace($statePath)) {
    $statePath = Join-Path (Ensure-AgentRoot) "state.json"
  }
  $State.last_seen = [DateTimeOffset]::Now.ToString("o")
  $State | ConvertTo-Json -Depth 8 | Set-Content -LiteralPath $statePath
}

function Invoke-AgentCommand {
  param([string]$Command)

  $start = [DateTimeOffset]::Now
  $stdoutPath = Join-Path ([IO.Path]::GetTempPath()) ("dockpipe-agent-stdout-" + [guid]::NewGuid().ToString("N") + ".log")
  $stderrPath = Join-Path ([IO.Path]::GetTempPath()) ("dockpipe-agent-stderr-" + [guid]::NewGuid().ToString("N") + ".log")
  $encoded = [Convert]::ToBase64String([Text.Encoding]::Unicode.GetBytes($Command))
  try {
    $proc = Start-Process -FilePath "powershell.exe" `
      -ArgumentList @("-NoProfile", "-ExecutionPolicy", "Bypass", "-EncodedCommand", $encoded) `
      -RedirectStandardOutput $stdoutPath `
      -RedirectStandardError $stderrPath `
      -PassThru -Wait

    $stdout = if (Test-Path -LiteralPath $stdoutPath) { Get-Content -Raw -LiteralPath $stdoutPath } else { "" }
    $stderr = if (Test-Path -LiteralPath $stderrPath) { Get-Content -Raw -LiteralPath $stderrPath } else { "" }
    $durationMs = [int](([DateTimeOffset]::Now - $start).TotalMilliseconds)
    return [pscustomobject]@{
      exit_code = $proc.ExitCode
      stdout = $stdout
      stderr = $stderr
      duration_ms = $durationMs
    }
  }
  finally {
    Remove-Item -LiteralPath $stdoutPath, $stderrPath -Force -ErrorAction SilentlyContinue
  }
}

function Send-JsonResponse {
  param(
    [System.Net.HttpListenerContext]$Context,
    [int]$StatusCode,
    $Body
  )

  $json = $Body | ConvertTo-Json -Depth 8 -Compress
  $bytes = [Text.Encoding]::UTF8.GetBytes($json)
  $Context.Response.StatusCode = $StatusCode
  $Context.Response.ContentType = "application/json"
  $Context.Response.ContentEncoding = [Text.Encoding]::UTF8
  $Context.Response.ContentLength64 = $bytes.Length
  $Context.Response.OutputStream.Write($bytes, 0, $bytes.Length)
  $Context.Response.OutputStream.Close()
}

function Read-JsonRequestBody {
  param([System.Net.HttpListenerRequest]$Request)
  $reader = [IO.StreamReader]::new($Request.InputStream, $Request.ContentEncoding)
  try {
    $raw = $reader.ReadToEnd()
  }
  finally {
    $reader.Dispose()
  }
  if ([string]::IsNullOrWhiteSpace($raw)) {
    return [pscustomobject]@{}
  }
  return ($raw | ConvertFrom-Json)
}

function Start-AgentServer {
  $config = Get-AgentConfig
  $bindAddress = if ([string]::IsNullOrWhiteSpace($config.bind_address)) { "127.0.0.1" } else { [string]$config.bind_address }
  $port = if ($config.port) { [int]$config.port } else { 47831 }
  $listener = [System.Net.HttpListener]::new()
  $listener.Prefixes.Add(("http://{0}:{1}/" -f $bindAddress, $port))
  $listener.Start()

  $startedAt = [DateTimeOffset]::Now.ToString("o")
  Write-AgentLog ("listening on {0}:{1}" -f $bindAddress, $port)
  Save-AgentState @{
    status = "ready"
    started_at = $startedAt
    service_account = [Security.Principal.WindowsIdentity]::GetCurrent().Name
    machine_name = $env:COMPUTERNAME
  }

  while ($listener.IsListening) {
    try {
      $context = $listener.GetContext()
      $request = $context.Request
      $path = $request.Url.AbsolutePath.TrimEnd("/")
      if ([string]::IsNullOrWhiteSpace($path)) {
        $path = "/"
      }

      switch ("{0} {1}" -f $request.HttpMethod.ToUpperInvariant(), $path) {
        "GET /" {
          Send-JsonResponse -Context $context -StatusCode 200 -Body @{
            name = "dockpipe-guest-agent"
            status = "ready"
            started_at = $startedAt
            service_account = [Security.Principal.WindowsIdentity]::GetCurrent().Name
            machine_name = $env:COMPUTERNAME
          }
          continue
        }
        "GET /health" {
          Send-JsonResponse -Context $context -StatusCode 200 -Body @{
            name = "dockpipe-guest-agent"
            status = "ready"
            started_at = $startedAt
            service_account = [Security.Principal.WindowsIdentity]::GetCurrent().Name
            machine_name = $env:COMPUTERNAME
          }
          continue
        }
        "GET /state" {
          $config = Get-AgentConfig
          $statePath = $config.state_path
          if (-not [string]::IsNullOrWhiteSpace($statePath) -and (Test-Path -LiteralPath $statePath)) {
            Send-JsonResponse -Context $context -StatusCode 200 -Body (Get-Content -Raw -LiteralPath $statePath | ConvertFrom-Json)
          } else {
            Send-JsonResponse -Context $context -StatusCode 200 -Body @{
              status = "ready"
              started_at = $startedAt
              service_account = [Security.Principal.WindowsIdentity]::GetCurrent().Name
              machine_name = $env:COMPUTERNAME
            }
          }
          continue
        }
        "POST /run" {
          $body = Read-JsonRequestBody -Request $request
          $command = [string]$body.command
          if ([string]::IsNullOrWhiteSpace($command)) {
            Send-JsonResponse -Context $context -StatusCode 400 -Body @{ error = "command is required" }
            continue
          }
          Write-AgentLog ("run request received")
          $result = Invoke-AgentCommand -Command $command
          Save-AgentState @{
            status = "ready"
            started_at = $startedAt
            service_account = [Security.Principal.WindowsIdentity]::GetCurrent().Name
            machine_name = $env:COMPUTERNAME
            last_run_exit_code = $result.exit_code
            last_run_duration_ms = $result.duration_ms
          }
          Send-JsonResponse -Context $context -StatusCode 200 -Body $result
          continue
        }
        "POST /shutdown" {
          Write-AgentLog "shutdown request received"
          Send-JsonResponse -Context $context -StatusCode 202 -Body @{ status = "accepted"; action = "shutdown" }
          Start-Process -FilePath "shutdown.exe" -ArgumentList @("/s", "/t", "0", "/f") -WindowStyle Hidden | Out-Null
          continue
        }
        default {
          Send-JsonResponse -Context $context -StatusCode 404 -Body @{ error = "not found"; method = $request.HttpMethod; path = $path }
          continue
        }
      }
    }
    catch {
      Write-AgentLog ("request handling failed: " + $_.Exception.Message)
      try {
        if ($null -ne $context) {
          Send-JsonResponse -Context $context -StatusCode 500 -Body @{ error = $_.Exception.Message }
        }
      }
      catch {
      }
    }
  }
}

if ($Service) {
  Write-AgentLog "service mode starting"
  Start-AgentServer
}
else {
  Start-AgentServer
}
