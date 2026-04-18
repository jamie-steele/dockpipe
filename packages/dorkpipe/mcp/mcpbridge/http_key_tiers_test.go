package mcpbridge

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadHTTPKeyTierFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	p := filepath.Join(dir, "keys.json")
	if err := os.WriteFile(p, []byte(`[
  {"key":"a","tier":"readonly"},
  {"key":"b","tier":"exec"}
]`), 0o600); err != nil {
		t.Fatal(err)
	}
	entries, err := loadHTTPKeyTierFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("len %d", len(entries))
	}
	if entries[0].tier != TierReadonly || entries[1].tier != TierExec {
		t.Fatalf("%v %v", entries[0].tier, entries[1].tier)
	}
}

func TestLoadHTTPKeyTierFileDuplicateKey(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	p := filepath.Join(dir, "keys.json")
	if err := os.WriteFile(p, []byte(`[
  {"key":"same","tier":"readonly"},
  {"key":"same","tier":"exec"}
]`), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := loadHTTPKeyTierFile(p)
	if err == nil {
		t.Fatal("expected error")
	}
}
