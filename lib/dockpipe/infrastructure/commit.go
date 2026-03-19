package infrastructure

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// CommitOnHost runs git add/commit in workdir (like commit-worktree on host).
func CommitOnHost(workdir, message, bundleOut string) error {
	check := exec.Command("git", "-C", workdir, "rev-parse", "--is-inside-work-tree")
	if out, err := check.CombinedOutput(); err != nil || !strings.Contains(string(out), "true") {
		fmt.Fprintln(os.Stderr, "[dockpipe] Not a git repo; skipping commit.")
		return nil
	}
	st := exec.Command("git", "-C", workdir, "status", "--porcelain")
	porcelain, _ := st.Output()
	if len(strings.TrimSpace(string(porcelain))) == 0 {
		fmt.Fprintln(os.Stderr, "[dockpipe] No changes to commit.")
		return nil
	}
	br := exec.Command("git", "-C", workdir, "branch", "--show-current")
	cur, _ := br.Output()
	fmt.Fprintf(os.Stderr, "[dockpipe] Committing on branch: %s\n", strings.TrimSpace(string(cur)))
	add := exec.Command("git", "-C", workdir, "add", "-A")
	if out, err := add.CombinedOutput(); err != nil {
		return fmt.Errorf("git add: %w\n%s", err, out)
	}
	msg := message
	if msg == "" {
		msg = "dockpipe: automated commit"
	}
	cmt := exec.Command("git", "-C", workdir, "commit", "-m", msg)
	cmt.Stdout = os.Stdout
	cmt.Stderr = os.Stderr
	if err := cmt.Run(); err != nil {
		return err
	}
	if bundleOut != "" {
		b := exec.Command("git", "-C", workdir, "bundle", "create", bundleOut, "--all")
		if out, err := b.CombinedOutput(); err != nil {
			fmt.Fprintf(os.Stderr, "[dockpipe] Failed to write bundle: %s\n", bundleOut)
			return fmt.Errorf("git bundle: %w\n%s", err, out)
		}
		fmt.Fprintf(os.Stderr, "[dockpipe] Bundle written: %s\n", bundleOut)
	}
	return nil
}
