package infrastructure

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveResolverFilePath(t *testing.T) {
	repo := t.TempDir()
	wf := filepath.Join(repo, "templates", "mywf")
	_ = os.MkdirAll(filepath.Join(wf, "resolvers"), 0o755)
	local := filepath.Join(wf, "resolvers", "local")
	if err := os.WriteFile(local, []byte("x=1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	coreDir := filepath.Join(repo, "templates", "core", "resolvers")
	_ = os.MkdirAll(coreDir, 0o755)
	core := filepath.Join(coreDir, "shared")
	if err := os.WriteFile(core, []byte("y=2\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	p, err := ResolveResolverFilePath(repo, wf, "local")
	if err != nil {
		t.Fatal(err)
	}
	if p != local {
		t.Fatalf("want local resolver %s got %s", local, p)
	}
	p2, err := ResolveResolverFilePath(repo, wf, "shared")
	if err != nil {
		t.Fatal(err)
	}
	if p2 != core {
		t.Fatalf("want core resolver %s got %s", core, p2)
	}
	_, err = ResolveResolverFilePath(repo, wf, "missing")
	if err == nil {
		t.Fatal("expected error for missing resolver")
	}
}
