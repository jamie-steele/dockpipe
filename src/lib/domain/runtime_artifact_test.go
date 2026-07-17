package domain

import "testing"

func TestValidateCompiledRuntimeManifestValid(t *testing.T) {
	m := &CompiledRuntimeManifest{
		Schema:        2,
		Kind:          RuntimeManifestKind,
		PolicyProfile: "sidecar-client",
		Security: CompiledSecurityPolicy{
			Preset: "sidecar-client",
			Network: CompiledNetworkPolicy{
				Mode:        "allowlist",
				Enforcement: "proxy",
				Allow:       []string{"api.openai.com"},
				InternalDNS: true,
			},
			FS: CompiledFilesystemPolicy{
				Root:          "readonly",
				Writes:        "workspace-only",
				WritablePaths: []string{"/tmp"},
			},
			Process: CompiledProcessPolicy{
				User:            "non-root",
				NoNewPrivileges: true,
				DropCaps:        []string{"ALL"},
				PIDLimit:        256,
			},
		},
		Image: CompiledImageSelection{
			Source:    "build",
			AutoBuild: "if-stale",
			Build: &CompiledImageBuildSpec{
				Context:    ".",
				Dockerfile: "Dockerfile",
			},
		},
	}
	if err := ValidateCompiledRuntimeManifest(m); err != nil {
		t.Fatal(err)
	}
}

func TestValidateCompiledRuntimeManifestRejectsOfflineAllowlistMix(t *testing.T) {
	m := &CompiledRuntimeManifest{
		Schema: 2,
		Kind:   RuntimeManifestKind,
		Security: CompiledSecurityPolicy{
			Network: CompiledNetworkPolicy{
				Mode:  "offline",
				Allow: []string{"api.openai.com"},
			},
		},
	}
	if err := ValidateCompiledRuntimeManifest(m); err == nil {
		t.Fatal("expected validation error")
	}
}

func TestValidateCompiledRuntimeManifestRejectsAllowlistWithoutRules(t *testing.T) {
	m := &CompiledRuntimeManifest{
		Schema: 2,
		Kind:   RuntimeManifestKind,
		Security: CompiledSecurityPolicy{
			Network: CompiledNetworkPolicy{
				Mode: "allowlist",
			},
		},
	}
	if err := ValidateCompiledRuntimeManifest(m); err == nil {
		t.Fatal("expected validation error")
	}
}

func TestValidateCompiledRuntimeManifestRejectsOfflineProxyEnforcement(t *testing.T) {
	m := &CompiledRuntimeManifest{
		Schema: 2,
		Kind:   RuntimeManifestKind,
		Security: CompiledSecurityPolicy{
			Network: CompiledNetworkPolicy{
				Mode:        "offline",
				Enforcement: "proxy",
			},
		},
	}
	if err := ValidateCompiledRuntimeManifest(m); err == nil {
		t.Fatal("expected validation error")
	}
}

func TestValidateCompiledRuntimeManifestRejectsRestrictedNativeEnforcement(t *testing.T) {
	m := &CompiledRuntimeManifest{
		Schema: 2,
		Kind:   RuntimeManifestKind,
		Security: CompiledSecurityPolicy{
			Network: CompiledNetworkPolicy{
				Mode:        "restricted",
				Enforcement: "native",
			},
		},
	}
	if err := ValidateCompiledRuntimeManifest(m); err == nil {
		t.Fatal("expected validation error")
	}
}

func TestValidateCompiledRuntimeManifestRejectsRegistryWithoutRef(t *testing.T) {
	m := &CompiledRuntimeManifest{
		Schema: 2,
		Kind:   RuntimeManifestKind,
		Image: CompiledImageSelection{
			Source: "registry",
		},
	}
	if err := ValidateCompiledRuntimeManifest(m); err == nil {
		t.Fatal("expected validation error")
	}
}

func TestValidateImageArtifactManifestValid(t *testing.T) {
	m := &ImageArtifactManifest{
		Schema:        3,
		Kind:          ImageArtifactManifestKind,
		Source:        "build",
		ArtifactState: "planned",
		Fingerprint:   "sha256:abc",
		Build: &CompiledImageBuildSpec{
			Context:    ".",
			Dockerfile: "Dockerfile",
		},
		Provenance: ImageArtifactProvenance{
			Resolver:       "codex",
			PackageVersion: "1.2.3",
		},
	}
	if err := ValidateImageArtifactManifest(m); err != nil {
		t.Fatal(err)
	}
}

func TestValidateImageArtifactManifestRejectsMissingFingerprint(t *testing.T) {
	m := &ImageArtifactManifest{
		Schema: 1,
		Kind:   ImageArtifactManifestKind,
		Source: "registry",
	}
	if err := ValidateImageArtifactManifest(m); err == nil {
		t.Fatal("expected validation error")
	}
}

func TestRuntimeManifestPathForStep(t *testing.T) {
	got := RuntimeManifestPathForStep("Fetch Docs / Step 1")
	want := "steps/fetch-docs-step-1.runtime.effective.json"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestImageArtifactPathForStep(t *testing.T) {
	got := ImageArtifactPathForStep("Fetch Docs / Step 1")
	want := "steps/fetch-docs-step-1.image-artifact.json"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}
