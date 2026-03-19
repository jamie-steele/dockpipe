package infrastructure

import (
	"fmt"
	"os/exec"
	"strings"
)

// GitTopLevel runs `git -C dir rev-parse --show-toplevel` (trimmed path).
func GitTopLevel(dir string) (string, error) {
	if strings.TrimSpace(dir) == "" {
		return "", fmt.Errorf("git dir is empty")
	}
	cmd := exec.Command("git", "-C", dir, "rev-parse", "--show-toplevel")
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
	cmd := exec.Command("git", "-C", dir, "rev-parse", rev)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git rev-parse %s: %w", rev, err)
	}
	return strings.TrimSpace(string(out)), nil
}

// GitBranchShowCurrent runs `git -C dir branch --show-current` (empty when detached HEAD).
func GitBranchShowCurrent(dir string) (string, error) {
	if strings.TrimSpace(dir) == "" {
		return "", fmt.Errorf("git dir is empty")
	}
	cmd := exec.Command("git", "-C", dir, "branch", "--show-current")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git branch --show-current: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// GitRemoteGetURL runs `git -C dir remote get-url remote` and returns trimmed stdout.
func GitRemoteGetURL(dir, remote string) (string, error) {
	if strings.TrimSpace(dir) == "" {
		return "", fmt.Errorf("git dir is empty")
	}
	if remote == "" {
		remote = "origin"
	}
	cmd := exec.Command("git", "-C", dir, "remote", "get-url", remote)
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
