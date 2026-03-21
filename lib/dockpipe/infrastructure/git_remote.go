package infrastructure

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

func gitDir(dir string) (string, error) {
	s := strings.TrimSpace(dir)
	if s == "" {
		return "", fmt.Errorf("git dir is empty")
	}
	s = HostPathForGit(s)
	return filepath.Clean(s), nil
}

// GitTopLevel runs `git rev-parse --show-toplevel` with working directory dir (trimmed path).
func GitTopLevel(dir string) (string, error) {
	d, err := gitDir(dir)
	if err != nil {
		return "", err
	}
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	cmd.Dir = d
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git rev-parse --show-toplevel: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// RepoURLsEquivalent compares two remote URLs loosely (trim, optional .git suffix, case-insensitive).
// Does not normalize SSH vs HTTPS; same form as origin is required for a match.
func RepoURLsEquivalent(a, b string) bool {
	a, b = strings.TrimSpace(a), strings.TrimSpace(b)
	if a == "" || b == "" {
		return false
	}
	if a == b {
		return true
	}
	a = strings.TrimSuffix(a, ".git")
	b = strings.TrimSuffix(b, ".git")
	return strings.EqualFold(a, b)
}

// GitRevParse runs `git -C dir rev-parse rev` (e.g. rev "HEAD").
func GitRevParse(dir, rev string) (string, error) {
	if strings.TrimSpace(dir) == "" || strings.TrimSpace(rev) == "" {
		return "", fmt.Errorf("git dir or rev is empty")
	}
	d, err := gitDir(dir)
	if err != nil {
		return "", err
	}
	cmd := exec.Command("git", "-C", d, "rev-parse", rev)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git rev-parse %s: %w", rev, err)
	}
	return strings.TrimSpace(string(out)), nil
}

// GitBranchShowCurrent runs `git branch --show-current` with working directory dir (empty when detached HEAD).
func GitBranchShowCurrent(dir string) (string, error) {
	d, err := gitDir(dir)
	if err != nil {
		return "", err
	}
	cmd := exec.Command("git", "branch", "--show-current")
	cmd.Dir = d
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git branch --show-current: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// GitRemoteGetURL runs `git remote get-url remote` with working directory dir and returns trimmed stdout.
func GitRemoteGetURL(dir, remote string) (string, error) {
	d, err := gitDir(dir)
	if err != nil {
		return "", err
	}
	if remote == "" {
		remote = "origin"
	}
	cmd := exec.Command("git", "remote", "get-url", remote)
	cmd.Dir = d
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git remote get-url %s: %w", remote, err)
	}
	u := strings.TrimSpace(string(out))
	if u == "" {
		return "", fmt.Errorf("git remote %s has empty URL", remote)
	}
	return u, nil
}
