package infrastructure

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveResolverFilePath(t *testing.T) {
	repo := t.TempDir()
	// Workflow-local resolvers/ are not used — only templates/core/resolvers/.
	coreDir := filepath.Join(repo, "templates", "core", "resolvers")
	_ = os.MkdirAll(coreDir, 0o755)
	core := filepath.Join(coreDir, "shared")
	if err := os.WriteFile(core, []byte("y=2\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	p, err := ResolveResolverFilePath(repo, "shared")
	if err != nil {
		t.Fatal(err)
	}
	if p != core {
		t.Fatalf("want core resolver %s got %s", core, p)
	}
	_, err = ResolveResolverFilePath(repo, "missing")
	if err == nil {
		t.Fatal("expected error for missing resolver")
	}
}

func TestResolveResolverFilePathPrefersProfileInDirectory(t *testing.T) {
	repo := t.TempDir()
	rsDir := filepath.Join(repo, "templates", "core", "resolvers", "tool")
	if err := os.MkdirAll(rsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	prof := filepath.Join(rsDir, "profile")
	if err := os.WriteFile(prof, []byte("DOCKPIPE_RESOLVER_CMD=x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := ResolveResolverFilePath(repo, "tool")
	if err != nil {
		t.Fatal(err)
	}
	if p != prof {
		t.Fatalf("want profile %s got %s", prof, p)
	}
}

// TestResolveResolverFilePathIgnoresWorkflowLocal verifies profiles beside templates/<wf>/ are not used.
func TestResolveResolverFilePathIgnoresWorkflowLocal(t *testing.T) {
	repo := t.TempDir()
	wf := filepath.Join(repo, "templates", "acme")
	_ = os.MkdirAll(filepath.Join(wf, "resolvers"), 0o755)
	if err := os.WriteFile(filepath.Join(wf, "resolvers", "onlyhere"), []byte("DOCKPIPE_RESOLVER_TEMPLATE=x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := ResolveResolverFilePath(repo, "onlyhere")
	if err == nil {
		t.Fatal("expected error: runtime profiles are not read from workflow template folders")
	}
}
