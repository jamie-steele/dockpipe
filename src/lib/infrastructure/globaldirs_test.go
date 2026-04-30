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
