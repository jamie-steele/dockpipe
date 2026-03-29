package application

import (
	"strings"
	"testing"

	"dockpipe/src/lib/dockpipe/infrastructure"
)

// TestRunInvalidResolverErrorsBeforeDocker fails fast when no templates/core/resolvers/<name> exists.
func TestRunInvalidResolverErrorsBeforeDocker(t *testing.T) {
	withRunSeams(t)
	repoRoot := t.TempDir()
	repoRootAppFn = func() (string, error) { return repoRoot, nil }

	err := Run([]string{"--resolver", "no-such-resolver-xyz", "--", "echo", "x"}, nil)
	if err == nil || !strings.Contains(err.Error(), "isolation profile") {
		t.Fatalf("expected isolation profile error, got %v", err)
	}
}

// TestRunInvalidStrategyErrorsBeforeIsolate fails when templates/core/strategies/<name> is missing.
func TestRunInvalidStrategyErrorsBeforeIsolate(t *testing.T) {
	withRunSeams(t)
	repoRoot := testRepoRoot(t)
	repoRootAppFn = func() (string, error) { return repoRoot, nil }

	err := Run([]string{"--strategy", "no-such-strategy-xyz", "--", "echo", "x"}, nil)
	if err == nil || !strings.Contains(err.Error(), "strategy") {
		t.Fatalf("expected strategy resolution error, got %v", err)
	}
}

// TestCrosscutStrategyWorktreeAndCommitResolveFromCore exercises bundled strategies without Docker.
func TestCrosscutStrategyWorktreeAndCommitResolveFromCore(t *testing.T) {
	repo := testRepoRoot(t)
	for _, strat := range []string{"commit", "worktree"} {
		_, _, err := LoadStrategyAssignments(repo, "", strat)
		if err != nil {
			t.Fatalf("strategy %q: %v", strat, err)
		}
	}
}

// TestCrosscutRuntimeResolverDockerimageClaudeAgainstBundledTree merges real runtimes/dockerimage + resolvers/claude profiles.
func TestCrosscutRuntimeResolverDockerimageClaudeAgainstBundledTree(t *testing.T) {
	repo := testRepoRoot(t)
	m, err := infrastructure.LoadIsolationProfile(repo, "dockerimage", "claude")
	if err != nil {
		t.Fatal(err)
	}
	if m["DOCKPIPE_RUNTIME_SUBSTRATE"] != "dockerimage" {
		t.Fatalf("expected dockerimage substrate, got %#v", m)
	}
	// Bundled claude profile uses DOCKPIPE_RESOLVER_CMD (not necessarily TEMPLATE).
	if strings.TrimSpace(m["DOCKPIPE_RESOLVER_CMD"]) != "claude" && strings.TrimSpace(m["DOCKPIPE_RESOLVER_TEMPLATE"]) != "claude" {
		t.Fatalf("expected claude resolver signal in merged profile, got %#v", m)
	}
}
