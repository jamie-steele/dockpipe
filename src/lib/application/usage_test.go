package application

import (
	"strings"
	"testing"
)

func TestRenderUsageSectionsWrapsNarrowWidth(t *testing.T) {
	out := renderUsageSections(mainUsageSections, 72)
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	for _, line := range lines {
		if len(line) > 72 {
			t.Fatalf("line exceeds width 72: %q (%d chars)", line, len(line))
		}
	}
	if !strings.Contains(out, "Commands:") {
		t.Fatal("expected Commands section")
	}
}

func TestRenderUsageSectionsStacksColumnsWhenNarrow(t *testing.T) {
	sections := []usageSection{{
		title: "Flags",
		entries: []usageEntry{
			{"--very-long-flag-name <value>", "Description that should wrap cleanly on narrow terminals."},
		},
	}}
	out := renderUsageSections(sections, 60)
	if !strings.Contains(out, "  --very-long-flag-name <value>\n") {
		t.Fatalf("expected left column on its own line, got:\n%s", out)
	}
	if !strings.Contains(out, "      Description that should wrap cleanly on narrow") {
		t.Fatalf("expected indented wrapped description, got:\n%s", out)
	}
}
