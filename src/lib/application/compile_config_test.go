package application

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dockpipe/src/lib/domain"
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
	out := effectiveWorkflowCompileRoots(cfg, repo)
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
	extra := filepath.Join(repo, "vendor", "extra-wf")
	if err := os.MkdirAll(extra, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(repo, "workflows"), 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := &domain.DockpipeProjectConfig{
		Compile: domain.DockpipeCompileConfig{
			Workflows: &[]string{"workflows", "vendor/extra-wf"},
		},
	}
	out := effectiveWorkflowCompileRoots(cfg, repo)
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

func TestEffectiveWorkflowCompileRootsLogsMissingConfiguredPaths(t *testing.T) {
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, "workflows"), 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := &domain.DockpipeProjectConfig{
		Compile: domain.DockpipeCompileConfig{
			Workflows: &[]string{"workflows", "missing-workflows"},
		},
	}
	stderr, err := captureResultStderr(t, func() error {
		out := effectiveWorkflowCompileRoots(cfg, repo)
		if len(out) != 1 {
			t.Fatalf("want 1 existing root, got %d: %v", len(out), out)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"unit=config.compile_path",
		"status=done",
		"path=" + filepath.Join(repo, "missing-workflows"),
		"result=skip",
		"root_kind=workflows",
		"skip_reason=missing_path",
	} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("expected stderr to contain %q, got:\n%s", want, stderr)
		}
	}
}
