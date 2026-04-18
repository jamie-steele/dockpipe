package infrastructure

import (
	"io/fs"
	"testing"

	"dockpipe"
)

func TestBundledVaultTemplateExampleExists(t *testing.T) {
	b, err := fs.ReadFile(dockpipe.BundledFS, VaultTemplateExampleEmbeddedPath)
	if err != nil {
		t.Fatalf("embed %s: %v", VaultTemplateExampleEmbeddedPath, err)
	}
	if len(b) < 50 {
		t.Fatalf("unexpected short embedded vault example: %d bytes", len(b))
	}
}
