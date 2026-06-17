package infrastructure

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestGlobalDockpipeDataDirOverride(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("DOCKPIPE_GLOBAL_ROOT", tmp)
	got, err := GlobalDockpipeDataDir()
	if err != nil {
		t.Fatal(err)
	}
	if got != tmp && filepath.Clean(got) != filepath.Clean(tmp) {
		t.Fatalf("got %q want %q", got, tmp)
	}
	sub, err := GlobalPackagesWorkflowsDir()
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(tmp, "packages", "workflows"); sub != want {
		t.Fatalf("workflows dir: got %q want %q", sub, want)
	}
	tc, err := GlobalTemplatesCoreDir()
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(tmp, "templates", "core"); tc != want {
		t.Fatalf("templates core: got %q want %q", tc, want)
	}
	img, err := GlobalImageArtifactByFingerprintDir()
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(tmp, "images", "by-fingerprint"); img != want {
		t.Fatalf("image artifacts: got %q want %q", img, want)
	}
	cache, err := GlobalCacheRoot()
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(tmp, "cache"); cache != want {
		t.Fatalf("cache root: got %q want %q", cache, want)
	}
}

func TestGlobalDockpipeDataDirNoOverrideHasSep(t *testing.T) {
	got, err := GlobalDockpipeDataDir()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "dockpipe") {
		t.Fatalf("expected path segment dockpipe, got %q", got)
	}
}

func TestSystemDockpipeDataDirsOverride(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("DOCKPIPE_SYSTEM_ROOT", tmp)

	dirs := SystemDockpipeDataDirs()
	if len(dirs) != 1 {
		t.Fatalf("expected 1 system dir, got %d (%v)", len(dirs), dirs)
	}
	if filepath.Clean(dirs[0]) != filepath.Clean(tmp) {
		t.Fatalf("got %q want %q", dirs[0], tmp)
	}

	coreDirs := SystemPackagesCoreDirs()
	if len(coreDirs) != 1 {
		t.Fatalf("expected 1 core dir, got %d (%v)", len(coreDirs), coreDirs)
	}
	if want := filepath.Join(tmp, "packages", "core"); filepath.Clean(coreDirs[0]) != filepath.Clean(want) {
		t.Fatalf("got %q want %q", coreDirs[0], want)
	}

	templatesDirs := SystemTemplatesCoreDirs()
	if len(templatesDirs) != 1 {
		t.Fatalf("expected 1 templates/core dir, got %d (%v)", len(templatesDirs), templatesDirs)
	}
	if want := filepath.Join(tmp, "templates", "core"); filepath.Clean(templatesDirs[0]) != filepath.Clean(want) {
		t.Fatalf("got %q want %q", templatesDirs[0], want)
	}
}
