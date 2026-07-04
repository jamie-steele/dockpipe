package infrastructure

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

type HostCommitResult struct {
	Result      string
	SkipReason  string
	Branch      string
	BundleOut   string
	BundleScope string
}

// CommitOnHost runs git add/commit in workdir (like commit-worktree on host).
// If bundleOut is set, bundleAll selects git's --all; otherwise only refs/heads/<current> (or HEAD if detached).
func CommitOnHost(workdir, message, bundleOut string, bundleAll bool) error {
	_, err := CommitOnHostWithResult(workdir, message, bundleOut, bundleAll)
	return err
}

// CommitOnHostWithResult runs the host-side git commit flow and returns the structured outcome.
func CommitOnHostWithResult(workdir, message, bundleOut string, bundleAll bool) (HostCommitResult, error) {
	result := HostCommitResult{}
	wd, err := gitDir(workdir)
	if err != nil {
		result.Result = "skipped"
		result.SkipReason = "not_git_repo"
		return result, nil
	}
	check := exec.Command("git", "-C", wd, "rev-parse", "--is-inside-work-tree")
	if out, err := check.CombinedOutput(); err != nil || !strings.Contains(string(out), "true") {
		result.Result = "skipped"
		result.SkipReason = "not_git_repo"
		return result, nil
	}
	st := exec.Command("git", "-C", wd, "status", "--porcelain")
	porcelain, _ := st.Output()
	if len(strings.TrimSpace(string(porcelain))) == 0 {
		result.Result = "noop"
		result.SkipReason = "no_changes"
		return result, nil
	}
	br := exec.Command("git", "-C", wd, "branch", "--show-current")
	cur, _ := br.Output()
	result.Branch = strings.TrimSpace(string(cur))
	add := exec.Command("git", "-C", wd, "add", "-A")
	if out, err := add.CombinedOutput(); err != nil {
		return HostCommitResult{}, fmt.Errorf("git add: %w\n%s", err, out)
	}
	msg := message
	if msg == "" {
		msg = "dockpipe: automated commit"
	}
	cmt := exec.Command("git", "-C", wd, "commit", "-m", msg)
	cmt.Stdout = os.Stdout
	cmt.Stderr = os.Stderr
	if err := cmt.Run(); err != nil {
		return HostCommitResult{}, err
	}
	result.Result = "committed"
	if bundleOut != "" {
		branch := strings.TrimSpace(string(cur))
		gitArgs := []string{"-C", wd, "bundle", "create", bundleOut}
		if bundleAll {
			gitArgs = append(gitArgs, "--all")
			result.BundleScope = "all"
		} else if branch != "" {
			gitArgs = append(gitArgs, "refs/heads/"+branch)
			result.BundleScope = "branch"
		} else {
			gitArgs = append(gitArgs, "HEAD")
			result.BundleScope = "head"
		}
		b := exec.Command("git", gitArgs...)
		if out, err := b.CombinedOutput(); err != nil {
			return HostCommitResult{}, fmt.Errorf("git bundle: %w\n%s", err, out)
		}
		result.BundleOut = bundleOut
	}
	return result, nil
}
