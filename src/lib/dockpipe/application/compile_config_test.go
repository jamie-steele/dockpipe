package application

import (
	"os"
	"path/filepath"
	"testing"

	"dockpipe/src/lib/dockpipe/domain"
)

func TestEffectiveWorkflowCompileRootsUsesConfigOnly(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	wf := filepath.Join(repo, "workflows")
	if err := os.MkdirAll(wf, 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := &domain.DockpipeProjectConfig{
		Compile: domain.DockpipeCompileConfig{
			Workflows: &[]string{"workflows"},
		},
	}
	out := effectiveWorkflowCompileRoots(cfg, repo, false)
	if len(out) != 1 {
		t.Fatalf("want 1 root (explicit workflows only), got %d: %v", len(out), out)
	}
	if filepath.Clean(out[0]) != filepath.Clean(wf) {
		t.Fatalf("got %v want %s", out, wf)
	}
}

func TestEffectiveWorkflowCompileRootsNoDuplicateWhenListedTwice(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	staging := filepath.Join(repo, ".staging", "packages")
	if err := os.MkdirAll(staging, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(repo, "workflows"), 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := &domain.DockpipeProjectConfig{
		Compile: domain.DockpipeCompileConfig{
			Workflows: &[]string{"workflows", ".staging/packages"},
		},
	}
	out := effectiveWorkflowCompileRoots(cfg, repo, false)
	seen := map[string]int{}
	for _, p := range out {
		seen[filepath.Clean(p)]++
	}
	for p, n := range seen {
		if n > 1 {
			t.Fatalf("duplicate root %q (count %d): %v", p, n, out)
		}
	}
}
