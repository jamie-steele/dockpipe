package domain

import (
	"encoding/json"
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
	if c.Compile.Resolvers == nil || len(*c.Compile.Resolvers) < 1 {
		t.Fatal("expected compile.resolvers")
	}
	if c.Secrets.OpInjectTemplate == nil || *c.Secrets.OpInjectTemplate == "" {
		t.Fatal("expected secrets.op_inject_template in default")
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
