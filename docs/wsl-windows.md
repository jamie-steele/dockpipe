# WSL2 → Windows: git bundle fetch

Get branches from a repo in WSL into your Windows clone **without pushing to a remote**. dockpipe can write a git bundle when it commits on the host; you then fetch that bundle from PowerShell.

**Install `dockpipe.exe` first** (MSI, script, or zip) — see **[install.md](install.md)** (Windows section). Quick option: `irm https://raw.githubusercontent.com/jamie-steele/dockpipe/master/packaging/windows/install.ps1 | iex`

**Default Windows UX:** use **`dockpipe.exe` on Windows** (Docker Desktop + Git for Windows) — same mental model as on Linux: Windows is Windows. This doc is for **git bundle / mixed-clone** workflows across WSL and Windows, or if you use the **optional** WSL bridge; it is **not** part of the default install story.

**If you use the WSL bridge**, recommended one-time setup (Windows terminal):

```powershell
dockpipe windows setup
dockpipe windows doctor
```

This configures the target distro, bootstraps WSL host-awareness env (`DOCKPIPE_WINDOWS_HOST=1`), and can run a distro install command when needed (`--install-command`).

**Manual test checklist:** **[manual-qa-windows.md](qa/manual-qa-windows.md)** (native + optional bridge; install Linux build in WSL via **[manual-qa-core.md](qa/manual-qa-core.md)** when testing the bridge).

### Dockpipe vs manual bundle (what’s actually “one branch”)

- **Workflow (llm-worktree + commit-worktree):** work happens in **one new worktree branch** (e.g. `claude/…` or `codex/…`); the **commit** is only on that branch’s tree. From the CLI this feels like a single branch.
- **The bundle file:** after commit-on-host, dockpipe runs **`git bundle create <path> refs/heads/<current-branch>`** (or **`HEAD`** if detached) — only the branch that was committed, so the file is usually **smaller** than `--all` when other branches have unique commits. Set **`DOCKPIPE_BUNDLE_ALL=1`** to restore the old **`--all`** behavior ([`commit.go`](../lib/dockpipe/infrastructure/commit.go), [`runner.sh`](../lib/runner.sh)).
- **Windows fetch:** the pain you hit (`refusing to fetch into branch … checked out`) is **Git on Windows**, not the bridge. A bare **`git fetch <bundle>`** (or `+refs/heads/*:refs/heads/*`) tries to move **local** `refs/heads/…` and **fails for the branch you have checked out**. Fetch resolver branches into **`refs/remotes/wsl/…`** first (and only then update other local branches), or restrict refspecs to **`claude/*`** / **`codex/*`** if that’s all you need.

### Running dockpipe from Windows

**Default:** use **`dockpipe.exe`** from PowerShell or CMD; it runs **on Windows** (same as Linux/macOS for git/docker), no WSL shell required.

**WSL bridge:** set **`DOCKPIPE_USE_WSL_BRIDGE=1`** so normal commands are **forwarded into WSL**: your **current Windows folder** becomes the working directory in Linux (via `wslpath`), then `dockpipe` runs there. Only **`dockpipe windows …`** stays on the host. **Windows paths in typical file flags** (e.g. `--workdir`, `--mount`, drive letters and UNC) are rewritten for the Linux argv; the command you pass after **`--`** is not altered.

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
# Fetches every branch head that EXISTS IN THE BUNDLE into refs/remotes/wsl/* (safe: does not update your checked-out local branch).
# Dockpipe’s default bundle has only the work branch; a manual "git bundle create --all" has every head.
# -MirrorLocalBranches: copy wsl/* to local branches (skips the branch you have checked out).
# -OnlyResolverBranches: when mirroring, only names under claude/* and codex/* (for fat bundles).
param(
    [Parameter(Mandatory = $true)] [string] $WindowsRepo,
    [Parameter(Mandatory = $true)] [string] $BundlePath,
    [string] $WSLRepoPath,
    [switch] $MirrorLocalBranches,
    [switch] $OnlyResolverBranches
)
$ErrorActionPreference = "Stop"
if ($WSLRepoPath) {
    $root = [System.IO.Path]::GetPathRoot($BundlePath)
    $drive = $root.TrimEnd(':\').ToLower()
    $rel = $BundlePath.Substring($root.Length) -replace '\\', '/'
    $BundlePathWsl = "/mnt/$drive/$rel"
    wsl -e sh -c "cd ""$WSLRepoPath"" && git bundle create ""$BundlePathWsl"" --all"
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
}
Set-Location $WindowsRepo
git fetch $BundlePath "+refs/heads/*:refs/remotes/wsl/*"
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

if ($MirrorLocalBranches) {
    $current = (git branch --show-current 2>$null).Trim()
    $refs = git for-each-ref "refs/remotes/wsl" --format="%(refname)"
    foreach ($ref in $refs) {
        if ($ref -notmatch '^refs/remotes/wsl/(.+)$') { continue }
        $name = $Matches[1]
        if ($OnlyResolverBranches -and ($name -notmatch '^(claude|codex)/')) { continue }
        if ($name -eq $current) { continue }
        git show-ref -q --verify "refs/heads/$name" 2>$null | Out-Null
        if ($LASTEXITCODE -eq 0) { git branch -f $name $ref } else { git branch $name $ref }
    }
}
exit 0
```

Run:

```powershell
.\FetchFromBundle.ps1 -WindowsRepo "C:\Source\YOUR_REPO" -BundlePath "C:\Users\YOUR_USER\YOUR_REPO.bundle"
```

To create the bundle from an existing WSL repo (no dockpipe): pass `-WSLRepoPath "/home/YOUR_USER/source/YOUR_REPO"`.

To also align **local** branch tips with the bundle (except the branch you have checked out): add **`-MirrorLocalBranches`**. If the bundle was built with **`--all`** and you only want **claude/** and **codex/** locals: **`-MirrorLocalBranches -OnlyResolverBranches`**.

**Even smaller bundles (optional):** Git can pack only commits not reachable from a basis (e.g. `origin/main..HEAD`), but dockpipe does not pick a basis for you; the default **single-branch** bundle already drops **other branch tips** and their unique commits from the pack.
