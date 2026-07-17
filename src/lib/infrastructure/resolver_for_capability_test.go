package infrastructure

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSubstrateRuntimeFromDockpipeCapability(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"dockpipe.cli", "dockerimage"},
		{"dockpipe.cli.foo", "dockerimage"},
		{"dockpipe.docker", "dockerimage"},
		{"dockpipe.docker.alpine", "dockerimage"},
		{"dockpipe.dockerfile", "dockerfile"},
		{"dockpipe.package", "package"},
		{"dockpipe.kubepod", "dockerimage"},
		{"dockpipe.keystore", "dockerimage"},
		{"dockpipe.cloud.aws", "dockerimage"},
		{"dockpipe.powershell", "dockerimage"},
		{"cli.codex", ""},
		{"", ""},
	}
	for _, tc := range tests {
		if got := SubstrateRuntimeFromDockpipeCapability(tc.in); got != tc.want {
			t.Errorf("SubstrateRuntimeFromDockpipeCapability(%q) = %q want %q", tc.in, got, tc.want)
		}
	}
}

func TestResolverProfileNameForCapability_fromPackageYML(t *testing.T) {
	repo := t.TempDir()
	rsDir := filepath.Join(repo, "templates", "core", "resolvers", "mytool")
	if err := os.MkdirAll(rsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(rsDir, PackageManifestFilename), []byte("kind: resolver\nname: mytool\ncapability: app.mytool\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := ResolverProfileNameForCapability(".", repo, "app.mytool")
	if err != nil {
		t.Fatal(err)
	}
	if got != "mytool" {
		t.Fatalf("got %q want mytool", got)
	}
}

func TestResolverProfileNameForCapability_unknownId(t *testing.T) {
	repo := t.TempDir()
	rsDir := filepath.Join(repo, "templates", "core", "resolvers", "x")
	if err := os.MkdirAll(rsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(rsDir, PackageManifestFilename), []byte("kind: resolver\nname: x\ncapability: cli.x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := ResolverProfileNameForCapability(".", repo, "workflow.does-not-exist")
	if err != nil {
		t.Fatal(err)
	}
	if got != "" {
		t.Fatalf("got %q want empty", got)
	}
}

func TestResolverProfileNameForCapability_ambiguous(t *testing.T) {
	repo := t.TempDir()
	base := filepath.Join(repo, "templates", "core", "resolvers")
	for _, name := range []string{"a", "b"} {
		d := filepath.Join(base, name)
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(d, PackageManifestFilename), []byte("kind: resolver\nname: "+name+"\ncapability: dup.same\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	_, err := ResolverProfileNameForCapability(".", repo, "dup.same")
	if err == nil {
		t.Fatal("expected error for duplicate capability")
	}
}
