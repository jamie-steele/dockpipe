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
func TestResolveWorkflowScriptResolvesScriptsPrefixToCoreWhenUserMissing(t *testing.T) {
	repo := t.TempDir()
	core := filepath.Join(repo, "templates", "core", "assets", "scripts", "shared.sh")
	if err := os.MkdirAll(filepath.Dir(core), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(core, []byte("#!/bin/sh\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := ResolveWorkflowScript("scripts/shared.sh", "/wf", repo)
	if got != filepath.ToSlash(core) {
		t.Fatalf("got %q want %q", got, filepath.ToSlash(core))
	}
}

func TestResolveWorkflowScriptResolvesScriptsPrefixToBundlesDirWhenPresent(t *testing.T) {
	repo := t.TempDir()
	b := filepath.Join(repo, "templates", "core", "bundles", "dorkpipe", "assets", "scripts", "aggregate-reasoning-context.sh")
	if err := os.MkdirAll(filepath.Dir(b), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(b, []byte("#!/bin/sh\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := ResolveWorkflowScript("scripts/dorkpipe/aggregate-reasoning-context.sh", "/wf", repo)
	if got != filepath.ToSlash(b) {
		t.Fatalf("got %q want %q", got, filepath.ToSlash(b))
	}
}

func TestResolveWorkflowScriptResolvesScriptsPrefixToResolverDirWhenPresent(t *testing.T) {
	repo := t.TempDir()
	rs := filepath.Join(repo, "templates", "core", "resolvers", "cursor-dev", "assets", "scripts", "cursor-dev-session.sh")
	if err := os.MkdirAll(filepath.Dir(rs), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(rs, []byte("#!/bin/sh\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := ResolveWorkflowScript("scripts/cursor-dev/cursor-dev-session.sh", "/wf", repo)
	if got != filepath.ToSlash(rs) {
		t.Fatalf("got %q want %q", got, filepath.ToSlash(rs))
	}
}

func TestResolveWorkflowScriptPrefersUserScriptsOverCore(t *testing.T) {
	repo := t.TempDir()
	user := filepath.Join(repo, "scripts", "shared.sh")
	core := filepath.Join(repo, "templates", "core", "assets", "scripts", "shared.sh")
	for _, p := range []string{user, core} {
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("#!/bin/sh\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	got := ResolveWorkflowScript("scripts/shared.sh", "/wf", repo)
	if got != filepath.ToSlash(user) {
		t.Fatalf("got %q want user path %q", got, filepath.ToSlash(user))
	}
}

func TestResolveWorkflowScriptPrefersRepoScriptsOverSrcScripts(t *testing.T) {
	repo := t.TempDir()
	top := filepath.Join(repo, "scripts", "tool.sh")
	src := filepath.Join(repo, "src", "scripts", "tool.sh")
	for _, p := range []string{top, src} {
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("#!/bin/sh\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	got := ResolveWorkflowScript("scripts/tool.sh", "/wf", repo)
	if got != filepath.ToSlash(top) {
		t.Fatalf("got %q want top-level scripts/ %q", got, filepath.ToSlash(top))
	}
}

func TestResolveWorkflowScriptUsesSrcScriptsWhenTopLevelScriptsMissing(t *testing.T) {
	repo := t.TempDir()
	src := filepath.Join(repo, "src", "scripts", "maint.sh")
	if err := os.MkdirAll(filepath.Dir(src), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(src, []byte("#!/bin/sh\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := ResolveWorkflowScript("scripts/maint.sh", "/wf", repo)
	if got != filepath.ToSlash(src) {
		t.Fatalf("got %q want %q", got, filepath.ToSlash(src))
	}
}

// TestResolveResolverFilePathFailsWhenProfileOnlyInRuntimes verifies resolver profiles are not read
// from templates/core/runtimes/ (taxonomy boundary).
func TestResolveResolverFilePathFailsWhenProfileOnlyInRuntimes(t *testing.T) {
	repo := t.TempDir()
	rt := filepath.Join(repo, "templates", "core", "runtimes", "ghost")
	if err := os.MkdirAll(filepath.Dir(rt), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(rt, []byte("DOCKPIPE_RUNTIME_KIND=dockerfile\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := ResolveResolverFilePath(repo, "ghost")
	if err == nil {
		t.Fatal("expected error: resolver name must not resolve from runtimes/")
	}
}

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
