package application

import (
	"strings"

	"dockpipe/src/lib/dockpipe/infrastructure"
)

// mergeWorktreeGitDockerEnv sets DOCKPIPE_WORKTREE_HEAD, and either DOCKPIPE_WORKTREE_BRANCH
// or DOCKPIPE_WORKTREE_DETACHED=1, by running git on the host workdir path (same tree mounted at /work).
// Clears those keys first so stale values from a prior step are not reused.
func mergeWorktreeGitDockerEnv(dockerEnv map[string]string, workdirHost string) {
	delete(dockerEnv, "DOCKPIPE_WORKTREE_HEAD")
	delete(dockerEnv, "DOCKPIPE_WORKTREE_BRANCH")
	delete(dockerEnv, "DOCKPIPE_WORKTREE_DETACHED")

	wd := strings.TrimSpace(workdirHost)
	if wd == "" {
		return
	}
	sha, err := infrastructure.GitRevParse(wd, "HEAD")
	if err != nil || sha == "" {
		return
	}
	dockerEnv["DOCKPIPE_WORKTREE_HEAD"] = sha

	br, err := infrastructure.GitBranchShowCurrent(wd)
	if err == nil && br != "" {
		dockerEnv["DOCKPIPE_WORKTREE_BRANCH"] = br
		return
	}
	dockerEnv["DOCKPIPE_WORKTREE_DETACHED"] = "1"
}
