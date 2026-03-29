package domain

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultDockpipeProjectConfigBytesRoundTrip(t *testing.T) {
	b, err := DefaultDockpipeProjectConfigBytes()
	if err != nil {
		t.Fatal(err)
	}
	var c DockpipeProjectConfig
	if err := json.Unmarshal(b, &c); err != nil {
		t.Fatalf("default JSON: %v\n%s", err, string(b))
	}
	if c.Schema != 1 {
		t.Fatalf("schema: %d", c.Schema)
	}
	if c.Compile.Workflows == nil || len(*c.Compile.Workflows) < 1 {
		t.Fatal("expected compile.workflows")
	}
	if c.Compile.Resolvers != nil {
		t.Fatal("default config should not set compile.resolvers (workflows is the entry point)")
	}
	if c.Secrets.VaultTemplate == nil || *c.Secrets.VaultTemplate == "" {
		t.Fatal("expected secrets.vault_template in default")
	}
}

func TestLoadDockpipeProjectConfigMissing(t *testing.T) {
	c, err := LoadDockpipeProjectConfig(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if c != nil {
		t.Fatal("expected nil when file missing")
	}
}

func TestResolveVaultTemplatePathPrecedence(t *testing.T) {
	root := t.TempDir()
	vault := ".env.vault.template"
	legacy := ".env.op.template"
	cfg := &DockpipeProjectConfig{
		Secrets: DockpipeSecretsConfig{
			VaultTemplate:    &vault,
			OpInjectTemplate: &legacy,
		},
	}
	got, ok := ResolveVaultTemplatePath(cfg, root)
	if !ok {
		t.Fatal("expected ok")
	}
	want := filepath.Join(root, vault)
	if got != want {
		t.Fatalf("got %q want %q (vault_template should win)", got, want)
	}
	cfg2 := &DockpipeProjectConfig{
		Secrets: DockpipeSecretsConfig{
			OpInjectTemplate: &legacy,
		},
	}
	got2, ok2 := ResolveVaultTemplatePath(cfg2, root)
	if !ok2 {
		t.Fatal("expected ok for legacy only")
	}
	if got2 != filepath.Join(root, legacy) {
		t.Fatalf("got %q", got2)
	}
}

func TestFindProjectRootWithDockpipeConfig(t *testing.T) {
	root := t.TempDir()
	sub := filepath.Join(root, "a", "b", "c")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	cfgPath := filepath.Join(root, DockpipeProjectConfigFileName)
	if err := os.WriteFile(cfgPath, []byte(`{"schema":1}`), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(sub)
	got, err := FindProjectRootWithDockpipeConfig(sub)
	if err != nil {
		t.Fatal(err)
	}
	want, err := filepath.Abs(root)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}
