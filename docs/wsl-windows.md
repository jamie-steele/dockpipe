# WSL2 → Windows: git bundle fetch

Get branches from a repo in WSL into your Windows clone **without pushing to a remote**. dockpipe can write a git bundle when it commits on the host; you then fetch that bundle from PowerShell.

**Install `dockpipe.exe` first** (MSI, script, or zip) — see **[install.md](install.md)** (Windows section). Quick option: `irm https://raw.githubusercontent.com/jamie-steele/dockpipe/main/packaging/windows/install.ps1 | iex`

**Recommended one-time setup (Windows terminal):**

```powershell
dockpipe windows setup
dockpipe windows doctor
```

This configures the target distro, bootstraps WSL host-awareness env (`DOCKPIPE_WINDOWS_HOST=1`), and can run a distro install command when needed (`--install-command`).

**Manual test checklist:** **[manual-qa-windows.md](manual-qa-windows.md)** (install Linux build in WSL via **[manual-qa-core.md](manual-qa-core.md)**).

### Running dockpipe from Windows without opening WSL

After setup, use **`dockpipe.exe`** from PowerShell or CMD. Normal commands are **forwarded into WSL**: your **current Windows folder** becomes the working directory in Linux (via `wslpath`), then `dockpipe` runs there. Only **`dockpipe windows …`** stays on the host. **Windows paths in typical file flags** (e.g. `--workdir`, `--mount`, drive letters and UNC) are rewritten for the Linux argv; the command you pass after **`--`** is not altered.

---

## End-to-end (one script)

Run dockpipe in WSL (worktree, commit, bundle) then fetch the bundle into your Windows repo. Save the script below as `Run-DockpipeWorktreeThenFetch.ps1` and run:

```powershell
.\Run-DockpipeWorktreeThenFetch.ps1 -RepoUrl "https://github.com/org/repo.git" -Branch "my-branch" -Prompt "Fix the bug" -WindowsRepo "C:\Source\YOUR_REPO" -BundlePath "C:\Users\YOUR_USER\YOUR_REPO.bundle"
```

Optional: `-Resolver codex`, or `-DockpipePath "/home/YOUR_USER/source/dockpipe"` if dockpipe is not on your WSL PATH.

**Run-DockpipeWorktreeThenFetch.ps1:**

```powershell
# Run dockpipe in WSL (worktree + commit + bundle-out), then fetch the bundle into your Windows repo.
param(
    [Parameter(Mandatory = $true)] [string] $RepoUrl,
    [Parameter(Mandatory = $true)] [string] $Branch,
    [Parameter(Mandatory = $true)] [string] $Prompt,
    [Parameter(Mandatory = $true)] [string] $WindowsRepo,
    [Parameter(Mandatory = $true)] [string] $BundlePath,
    [ValidateSet("claude", "codex")] [string] $Resolver = "claude",
    [string] $DockpipePath
)
$root = [System.IO.Path]::GetPathRoot($BundlePath)
$drive = $root.TrimEnd(':\').ToLower()
$rel = $BundlePath.Substring($root.Length) -replace '\\', '/'
$BundlePathWsl = "/mnt/$drive/$rel"
$dockpipeCmd = if ($DockpipePath) { "cd ""$DockpipePath"" && ./bin/dockpipe" } else { "dockpipe" }
$resolverArg = switch ($Resolver) { "claude" { "claude -p" }; "codex" { "codex -p" }; default { "claude -p" } }
$PromptEscaped = "'" + ($Prompt -replace "'", "'\\''") + "'"
$wslCmd = "$dockpipeCmd --resolver $Resolver --repo ""$RepoUrl"" --branch ""$Branch"" --act scripts/commit-worktree.sh --bundle-out ""$BundlePathWsl"" -- $resolverArg $PromptEscaped"
wsl -e sh -c $wslCmd
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
& "$scriptDir\FetchFromBundle.ps1" -WindowsRepo $WindowsRepo -BundlePath $BundlePath
exit $LASTEXITCODE
```

---

## Step by step

### 1. Generate the bundle (in WSL)

Use a path under `/mnt/c/...` so Windows can see the file.

```bash
dockpipe --workflow llm-worktree --repo https://github.com/org/repo.git --branch my-branch \
  --bundle-out /mnt/c/Users/YOUR_USER/YOUR_REPO.bundle \
  -- claude -p "Your prompt"
```

### 2. Fetch from the bundle (PowerShell)

**FetchFromBundle.ps1:**

```powershell
# Fetch from a git bundle into your Windows repo. Optionally create the bundle from a WSL repo first.
param(
    [Parameter(Mandatory = $true)] [string] $WindowsRepo,
    [Parameter(Mandatory = $true)] [string] $BundlePath,
    [string] $WSLRepoPath
)
if ($WSLRepoPath) {
    $root = [System.IO.Path]::GetPathRoot($BundlePath)
    $drive = $root.TrimEnd(':\').ToLower()
    $rel = $BundlePath.Substring($root.Length) -replace '\\', '/'
    $BundlePathWsl = "/mnt/$drive/$rel"
    wsl -e sh -c "cd ""$WSLRepoPath"" && git bundle create ""$BundlePathWsl"" --all"
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
}
Set-Location $WindowsRepo
git fetch $BundlePath
exit $LASTEXITCODE
```

Run:

```powershell
.\FetchFromBundle.ps1 -WindowsRepo "C:\Source\YOUR_REPO" -BundlePath "C:\Users\YOUR_USER\YOUR_REPO.bundle"
```

To create the bundle from an existing WSL repo (no dockpipe): pass `-WSLRepoPath "/home/YOUR_USER/source/YOUR_REPO"`.
