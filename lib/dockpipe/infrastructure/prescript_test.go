package infrastructure

import "testing"

func TestParseEnv0(t *testing.T) {
	data := []byte("A=1\x00BROKEN\x00B=two=parts\x00\x00")
	m := parseEnv0(data)
	if m["A"] != "1" {
		t.Fatalf("A mismatch: %#v", m)
	}
	if m["B"] != "two=parts" {
		t.Fatalf("B mismatch: %#v", m)
	}
	if _, ok := m["BROKEN"]; ok {
		t.Fatalf("BROKEN should be ignored: %#v", m)
	}
}
