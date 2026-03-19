package infrastructure

import "testing"

func TestRepoURLsEquivalent(t *testing.T) {
	t.Parallel()
	if !RepoURLsEquivalent("https://x/a.git", "https://x/a") {
		t.Fatal("expected .git-insensitive match")
	}
	if !RepoURLsEquivalent("HTTPS://X/a", "https://x/a") {
		t.Fatal("expected case-insensitive match")
	}
	if RepoURLsEquivalent("https://x/a", "https://y/a") {
		t.Fatal("expected mismatch")
	}
	if RepoURLsEquivalent("", "https://x/a") {
		t.Fatal("empty should not match")
	}
}
