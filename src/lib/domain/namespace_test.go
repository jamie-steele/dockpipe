package domain

import "testing"

func TestValidateNamespaceEmptyOK(t *testing.T) {
	if err := ValidateNamespace(""); err != nil {
		t.Fatal(err)
	}
	if err := ValidateNamespace("   "); err != nil {
		t.Fatal(err)
	}
}

func TestValidateNamespaceReserved(t *testing.T) {
	for _, s := range []string{"dockpipe", "core", "system", "DorkPipe", "WORKFLOW"} {
		if err := ValidateNamespace(s); err == nil {
			t.Fatalf("expected error for %q", s)
		}
	}
}

func TestValidateNamespacePattern(t *testing.T) {
	for _, s := range []string{"Acme", "my.team", "-bad", "9x"} {
		if err := ValidateNamespace(s); err == nil {
			t.Fatalf("expected error for %q", s)
		}
	}
	if err := ValidateNamespace("acme"); err != nil {
		t.Fatal(err)
	}
	if err := ValidateNamespace("my-team"); err != nil {
		t.Fatal(err)
	}
}
