package infrastructure

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dockpipe/src/lib/infrastructure/packagebuild"
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

func TestResolveResolverFilePathPrefersPackagesResolversStore(t *testing.T) {
	repo := t.TempDir()
	rsDir := filepath.Join(repo, DockpipeDirRel, "internal", "packages", "resolvers", "tool")
	if err := os.MkdirAll(rsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	prof := filepath.Join(rsDir, "profile")
	if err := os.WriteFile(prof, []byte("DOCKPIPE_RESOLVER_CMD=x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Shadow templates/core/resolvers — should be ignored when packages store wins first.
	legacy := filepath.Join(repo, "templates", "core", "resolvers", "tool", "profile")
	if err := os.MkdirAll(filepath.Dir(legacy), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(legacy, []byte("DOCKPIPE_RESOLVER_CMD=legacy\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := ResolveResolverFilePath(repo, "tool")
	if err != nil {
		t.Fatal(err)
	}
	if p != prof {
		t.Fatalf("want packages store profile %s got %s", prof, p)
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
func TestResolveCoreNamespacedScriptPath(t *testing.T) {
	repo := t.TempDir()
	core := filepath.Join(repo, "templates", "core", "assets", "scripts", "hello.sh")
	if err := os.MkdirAll(filepath.Dir(core), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(core, []byte("#\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Dotted namespace must map basename.hello.sh → assets/scripts/hello.sh (see relFromCoreNamespace).
	got, err := ResolveCoreNamespacedScriptPath(repo, "", "assets.scripts.hello.sh")
	if err != nil {
		t.Fatal(err)
	}
	want, err := filepath.Abs(core)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Clean(got) != filepath.Clean(want) {
		t.Fatalf("got %q want %q", got, want)
	}
	_, err = ResolveCoreNamespacedScriptPath(repo, "", "assets.scripts.nope.sh")
	if err == nil {
		t.Fatal("expected error for missing script")
	}
}

func TestResolveWorkflowScriptResolvesScriptsDockpipeToPackagesResolver(t *testing.T) {
	repo := t.TempDir()
	cfg := `{"schema":1,"compile":{"workflows":["packages"]}}`
	if err := os.WriteFile(filepath.Join(repo, "dockpipe.config.json"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(repo, "packages", "dorkpipe", "resolvers", "dorkpipe", "assets", "scripts", "r2-publish.sh")
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte("#!/bin/sh\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := ResolveWorkflowScript("scripts/dockpipe/r2-publish.sh", "/wf", repo, "")
	if got != filepath.ToSlash(p) {
		t.Fatalf("got %q want %q", got, filepath.ToSlash(p))
	}
}

func TestResolveWorkflowScriptResolvesCoreTerraformRun(t *testing.T) {
	repo := t.TempDir()
	cfg := `{"schema":1,"compile":{"workflows":["packages"]}}`
	if err := os.WriteFile(filepath.Join(repo, "dockpipe.config.json"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(repo, "packages", "terraform", "resolvers", "terraform-core", "assets", "scripts", "terraform-run.sh")
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte("#!/bin/sh\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := ResolveWorkflowScript("scripts/core.assets.scripts.terraform-run.sh", "/wf", repo, "")
	if got != filepath.ToSlash(p) {
		t.Fatalf("got %q want %q", got, filepath.ToSlash(p))
	}
}

func TestResolveWorkflowScriptResolvesCoreTerraformRunFromBundledTerraformCore(t *testing.T) {
	repo := t.TempDir()
	p := filepath.Join(repo, BundledLayoutDir, "workflows", "terraform-core", "assets", "scripts", "terraform-run.sh")
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte("#!/bin/sh\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := ResolveWorkflowScript("scripts/core.assets.scripts.terraform-run.sh", "/wf", repo, "")
	if got != filepath.ToSlash(p) {
		t.Fatalf("got %q want %q", got, filepath.ToSlash(p))
	}
}

func TestResolveWorkflowScriptResolvesScriptsPrefixToCoreWhenUserMissing(t *testing.T) {
	repo := t.TempDir()
	core := filepath.Join(repo, "templates", "core", "assets", "scripts", "shared.sh")
	if err := os.MkdirAll(filepath.Dir(core), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(core, []byte("#!/bin/sh\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := ResolveWorkflowScript("scripts/shared.sh", "/wf", repo, "")
	if got != filepath.ToSlash(core) {
		t.Fatalf("got %q want %q", got, filepath.ToSlash(core))
	}
}

func TestResolveWorkflowScriptResolvesScriptsPrefixToBundlesDirWhenPresent(t *testing.T) {
	repo := t.TempDir()
	cfg := `{"schema":1,"compile":{"bundles":["vendor/dockpipe-pkgs"]}}`
	if err := os.WriteFile(filepath.Join(repo, "dockpipe.config.json"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	b := filepath.Join(repo, "vendor", "dockpipe-pkgs", "dorkpipe", "assets", "scripts", "aggregate-reasoning-context.sh")
	if err := os.MkdirAll(filepath.Dir(b), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(b, []byte("#!/bin/sh\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := ResolveWorkflowScript("scripts/dorkpipe/aggregate-reasoning-context.sh", "/wf", repo, "")
	if got != filepath.ToSlash(b) {
		t.Fatalf("got %q want %q", got, filepath.ToSlash(b))
	}
}

func TestResolveWorkflowScriptResolvesScriptsPrefixToResolverDirWhenPresent(t *testing.T) {
	repo := t.TempDir()
	cfg := `{"schema":1,"compile":{"workflows":["vendor/extra-workflows"]}}`
	if err := os.WriteFile(filepath.Join(repo, "dockpipe.config.json"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	rs := filepath.Join(repo, "vendor", "extra-workflows", "dockpipe", "ide", "resolvers", "cursor-dev", "assets", "scripts", "cursor-dev-session.sh")
	if err := os.MkdirAll(filepath.Dir(rs), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(rs, []byte("#!/bin/sh\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := ResolveWorkflowScript("scripts/cursor-dev/cursor-dev-session.sh", "/wf", repo, "")
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
	got := ResolveWorkflowScript("scripts/shared.sh", "/wf", repo, "")
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
	got := ResolveWorkflowScript("scripts/tool.sh", "/wf", repo, "")
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
	got := ResolveWorkflowScript("scripts/maint.sh", "/wf", repo, "")
	if got != filepath.ToSlash(src) {
		t.Fatalf("got %q want %q", got, filepath.ToSlash(src))
	}
}

// TestResolveWorkflowScriptResolvesReviewPipelineFromWorkflowsRoot verifies logical scripts/review-pipeline/…
// resolves to workflows/review-pipeline/assets/scripts/ via compile workflow roots (no src/scripts/review symlink).
func TestResolveWorkflowScriptResolvesReviewPipelineFromWorkflowsRoot(t *testing.T) {
	repo := t.TempDir()
	script := filepath.Join(repo, "workflows", "review-pipeline", "assets", "scripts", "hello.sh")
	if err := os.MkdirAll(filepath.Dir(script), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(script, []byte("#!/bin/sh\n# review-pipeline\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := ResolveWorkflowScript("scripts/review-pipeline/hello.sh", "/wf", repo, "")
	if filepath.ToSlash(got) != filepath.ToSlash(script) {
		t.Fatalf("got %q want %q", got, script)
	}
}

// TestResolveWorkflowScriptResolvesPipeonFromNestedCompileRoot verifies logical scripts/pipeon/…
// resolves via tryNestedWorkflowScripts (compile.workflows includes a root directory named "packages") — no src/scripts/pipeon symlink.
func TestResolveWorkflowScriptResolvesPipeonFromNestedCompileRoot(t *testing.T) {
	repo := t.TempDir()
	cfg := `{"schema":1,"compile":{"workflows":["packages"]}}`
	if err := os.WriteFile(filepath.Join(repo, "dockpipe.config.json"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	script := filepath.Join(repo, "packages", "pipeon", "resolvers", "pipeon", "assets", "scripts", "hello.sh")
	if err := os.MkdirAll(filepath.Dir(script), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(script, []byte("#!/bin/sh\n# pipeon-nested\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := ResolveWorkflowScript("scripts/pipeon/hello.sh", "/wf", repo, "")
	if filepath.ToSlash(got) != filepath.ToSlash(script) {
		t.Fatalf("got %q want %q", got, script)
	}
}

// TestResolveWorkflowScriptPrefersCompiledResolverTarball verifies scripts/dorkpipe/… resolves from
// dockpipe-resolver-*.tar.gz under bin/.dockpipe/internal/packages/resolvers/ (extracted cache), not only
// from a flat extracted tree or .staging authoring paths.
func TestResolveWorkflowScriptPrefersCompiledResolverTarball(t *testing.T) {
	repo := t.TempDir()
	pkgRes := filepath.Join(repo, DockpipeDirRel, "internal", "packages", "resolvers")
	if err := os.MkdirAll(pkgRes, 0o755); err != nil {
		t.Fatal(err)
	}
	staging := filepath.Join(t.TempDir(), "resolver-staging")
	scriptRel := filepath.Join("assets", "scripts", "tarball-only.sh")
	if err := os.MkdirAll(filepath.Join(staging, filepath.Dir(scriptRel)), 0o755); err != nil {
		t.Fatal(err)
	}
	marker := []byte("#!/bin/sh\n# from-compiled-tarball\n")
	if err := os.WriteFile(filepath.Join(staging, scriptRel), marker, 0o644); err != nil {
		t.Fatal(err)
	}
	tgz := filepath.Join(pkgRes, "dockpipe-resolver-dorkpipe-9.9.9.tar.gz")
	if _, err := packagebuild.WriteDirTarGzWithPrefix(staging, tgz, "resolvers/dorkpipe"); err != nil {
		t.Fatal(err)
	}
	// No flat pkgRes/dorkpipe/assets/scripts/ — only the tarball should satisfy resolution.
	got := ResolveWorkflowScript("scripts/dorkpipe/tarball-only.sh", "/wf", repo, "")
	if !strings.Contains(filepath.ToSlash(got), "/.dockpipe/") {
		t.Fatalf("expected path under bin/.dockpipe (tarball extract cache), got %q", got)
	}
	b, err := os.ReadFile(got)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "from-compiled-tarball") {
		t.Fatalf("resolved file content unexpected: %s", b)
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

// TestResolveWorkflowScriptUsesProjectRootWhenRepoRootIsBundle verifies scripts/… resolution uses
// projectRoot (checkout) for nested resolver trees even when repoRoot is the materialized bundle layout.
func TestResolveWorkflowScriptUsesProjectRootWhenRepoRootIsBundle(t *testing.T) {
	bundle := t.TempDir()
	proj := t.TempDir()
	cfg := `{"schema":1,"compile":{"workflows":["packages"]}}`
	if err := os.WriteFile(filepath.Join(proj, "dockpipe.config.json"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	script := filepath.Join(proj, "packages", "acme", "resolvers", "r2", "dockpipe.cloudflare.r2infra", "assets", "scripts", "terraform-cloudflare-r2-run.sh")
	if err := os.MkdirAll(filepath.Dir(script), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(script, []byte("#!/bin/sh\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := ResolveWorkflowScript("scripts/dockpipe.cloudflare.r2infra/terraform-cloudflare-r2-run.sh", "/wf", bundle, proj)
	if filepath.ToSlash(got) != filepath.ToSlash(script) {
		t.Fatalf("got %q want %q", got, script)
	}
}
