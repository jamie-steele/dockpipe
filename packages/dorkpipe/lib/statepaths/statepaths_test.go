package statepaths

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestProviderPoolScratchDirUsesPackageState(t *testing.T) {
	root := t.TempDir()
	got, err := ProviderPoolScratchDir(root)
	if err != nil {
		t.Fatal(err)
	}
	wantSuffix := filepath.Join("bin", ".dockpipe", "packages", "dorkpipe", "provider-pools", "scratch")
	if !strings.HasSuffix(filepath.Clean(got), wantSuffix) {
		t.Fatalf("scratch dir = %q, want suffix %q", got, wantSuffix)
	}
}
