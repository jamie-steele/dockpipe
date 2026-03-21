package infrastructure

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadIsolationProfileMergesRuntimeThenResolver(t *testing.T) {
	repo := t.TempDir()
	rtDir := filepath.Join(repo, "templates", "core", "runtimes")
	rsDir := filepath.Join(repo, "templates", "core", "resolvers")
	_ = os.MkdirAll(rtDir, 0o755)
	_ = os.MkdirAll(rsDir, 0o755)
	if err := os.WriteFile(filepath.Join(rtDir, "r1"), []byte("DOCKPIPE_RUNTIME_WORKFLOW=wf1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(rsDir, "s1"), []byte("DOCKPIPE_RESOLVER_CMD=cli\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	m, err := LoadIsolationProfile(repo, "r1", "s1")
	if err != nil {
		t.Fatal(err)
	}
	if m["DOCKPIPE_RUNTIME_WORKFLOW"] != "wf1" || m["DOCKPIPE_RESOLVER_CMD"] != "cli" {
		t.Fatalf("merged map: %#v", m)
	}
}

func TestLoadIsolationProfileSameNamePairs(t *testing.T) {
	repo := t.TempDir()
	rtDir := filepath.Join(repo, "templates", "core", "runtimes")
	rsDir := filepath.Join(repo, "templates", "core", "resolvers")
	_ = os.MkdirAll(rtDir, 0o755)
	_ = os.MkdirAll(rsDir, 0o755)
	if err := os.WriteFile(filepath.Join(rtDir, "both"), []byte("DOCKPIPE_RUNTIME_TYPE=execution\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(rsDir, "both"), []byte("DOCKPIPE_RESOLVER_CMD=x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	m, err := LoadIsolationProfile(repo, "both", "")
	if err != nil {
		t.Fatal(err)
	}
	if m["DOCKPIPE_RUNTIME_TYPE"] != "execution" || m["DOCKPIPE_RESOLVER_CMD"] != "x" {
		t.Fatalf("merged: %#v", m)
	}
}
