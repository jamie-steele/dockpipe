package application

import (
	"strings"
	"testing"
)

// TestWorkflowTaskLines formats stderr banner lines for single-step vs multi-step workflows and missing names.
func TestWorkflowTaskLines(t *testing.T) {
	got := workflowTaskLines("My flow", "Do the thing", 0)
	if !strings.Contains(got, "My flow") || strings.Contains(got, "steps") {
		t.Fatalf("single: %q", got)
	}
	if !strings.Contains(got, "Do the thing") {
		t.Fatalf("description: %q", got)
	}
	got = workflowTaskLines("Chain", "", 3)
	if !strings.Contains(got, "Chain") || !strings.Contains(got, "3 steps") {
		t.Fatalf("multi: %q", got)
	}
	got = workflowTaskLines("", "", 2)
	if !strings.Contains(got, "(workflow)") {
		t.Fatalf("fallback name: %q", got)
	}
}
