# WSL2 → Windows: git bundle fetch

Get branches from a repo in WSL into your Windows clone **without pushing to a remote**. dockpipe can write a git bundle when it commits on the host; you then fetch that bundle from PowerShell.

**One-time:** Set your default WSL distro to your Linux (e.g. Ubuntu): `wsl -l -v` then `wsl --set-default Ubuntu` if needed. Ensure dockpipe and Docker run inside WSL; API keys in WSL environment.

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
