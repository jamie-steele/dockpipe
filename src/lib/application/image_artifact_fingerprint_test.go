package application

import (
	"os"
	"path/filepath"
	"testing"

	"dockpipe/src/lib/domain"
)

func TestBuildImageArtifactFingerprintSeparatesSourceFromPolicy(t *testing.T) {
	wd := t.TempDir()
	buildDir := filepath.Join(wd, "templates", "core", "assets", "images", "codex")
	if err := os.MkdirAll(buildDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(buildDir, "Dockerfile"), []byte("FROM alpine:3.20\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	prov := domain.ImageArtifactProvenance{
		Isolate:         "codex",
		PackageVersion:  "1.2.3",
		DockpipeVersion: "1.2.3",
	}
	a, err := buildImageArtifactManifest(wd, "wf", "pkg", "codex", "dockpipe-codex:1.2.3", buildDir, wd, "sha256:policy-a", prov)
	if err != nil {
		t.Fatal(err)
	}
	b, err := buildImageArtifactManifest(wd, "wf", "pkg", "codex", "dockpipe-codex:1.2.3", buildDir, wd, "sha256:policy-b", prov)
	if err != nil {
		t.Fatal(err)
	}
	if a.SourceFingerprint != b.SourceFingerprint {
		t.Fatalf("source fingerprint should ignore runtime policy: %q != %q", a.SourceFingerprint, b.SourceFingerprint)
	}
	if a.Fingerprint != b.Fingerprint {
		t.Fatalf("artifact fingerprint should ignore runtime-only policy: %q != %q", a.Fingerprint, b.Fingerprint)
	}
	if a.SecurityManifestFingerprint == b.SecurityManifestFingerprint {
		t.Fatalf("security manifest fingerprint should keep policy distinction")
	}
	if a.ArtifactState != "planned" {
		t.Fatalf("compiled build artifact should be planned, got %q", a.ArtifactState)
	}
	if a.Provenance.Isolate != "codex" || a.Provenance.PackageVersion != "1.2.3" {
		t.Fatalf("unexpected provenance: %+v", a.Provenance)
	}
}
