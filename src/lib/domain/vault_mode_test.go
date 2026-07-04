package domain

import (
	"testing"
)

func TestEffectiveVaultString(t *testing.T) {
	wfOp := &Workflow{Vault: "op"}
	cfgNone := &DockpipeProjectConfig{Secrets: DockpipeSecretsConfig{Vault: ptr("none")}}
	if got := EffectiveVaultString(wfOp, cfgNone); got != "op" {
		t.Fatalf("workflow should win: got %q", got)
	}
	if got := EffectiveVaultString(&Workflow{}, cfgNone); got != "none" {
		t.Fatalf("project default: got %q want none", got)
	}
	if got := EffectiveVaultString(&Workflow{}, &DockpipeProjectConfig{}); got != "" {
		t.Fatalf("empty: got %q", got)
	}
	if got := EffectiveVaultString(nil, cfgNone); got != "none" {
		t.Fatalf("nil workflow: got %q want none", got)
	}
}

func ptr(s string) *string { return &s }
