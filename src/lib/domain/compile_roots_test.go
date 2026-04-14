package domain

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEffectiveResolverCompileRootsMergesWorkflowsAndCore(t *testing.T) {
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, "workflows", "x"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(repo, "src", "core", "resolvers", "r1"), 0o755); err != nil {
		t.Fatal(err)
	}
	wf := []string{"workflows"}
	cfg := &DockpipeProjectConfig{Compile: DockpipeCompileConfig{Workflows: &wf}}
	got := EffectiveResolverCompileRoots(cfg, repo)
	if len(got) != 2 {
		t.Fatalf("want 2 roots (workflows + src/core/resolvers), got %v", got)
	}
}

func TestEffectiveResolverCompileRootsLegacyResolversMerged(t *testing.T) {
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, "workflows"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(repo, "extra", "resolvers"), 0o755); err != nil {
		t.Fatal(err)
	}
	wf := []string{"workflows"}
	legacy := []string{"extra"}
	cfg := &DockpipeProjectConfig{Compile: DockpipeCompileConfig{Workflows: &wf, Resolvers: &legacy}}
	got := EffectiveResolverCompileRoots(cfg, repo)
	if len(got) != 2 {
		t.Fatalf("want workflows + legacy extra root, got %d: %v", len(got), got)
	}
}
