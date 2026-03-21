package domain

import (
	"regexp"
	"testing"
)

// TestRandomWorkBranchSlugFormat checks generated slugs match the expected hyphenated word pattern.
func TestRandomWorkBranchSlugFormat(t *testing.T) {
	t.Parallel()
	re := regexp.MustCompile(`^[a-z]+(-[a-z]+){3}$`)
	for range 30 {
		s, err := RandomWorkBranchSlug()
		if err != nil {
			t.Fatalf("RandomWorkBranchSlug: %v", err)
		}
		if !re.MatchString(s) {
			t.Fatalf("unexpected slug format: %q", s)
		}
	}
}

// TestRandomWorkBranchSlugVaries checks two successive slugs are usually different (rare flake if equal).
func TestRandomWorkBranchSlugVaries(t *testing.T) {
	t.Parallel()
	a, err := RandomWorkBranchSlug()
	if err != nil {
		t.Fatal(err)
	}
	b, err := RandomWorkBranchSlug()
	if err != nil {
		t.Fatal(err)
	}
	if a == b {
		t.Fatalf("expected two slugs to differ (rare flake if equal): %q", a)
	}
}
