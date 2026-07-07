package mcpbridge

import "testing"

func TestShouldDiscoverCodexSessionID(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		resumed  bool
		known    string
		expected bool
	}{
		{name: "new session discovers id", resumed: false, known: "", expected: true},
		{name: "resumed without known id discovers id", resumed: true, known: "", expected: true},
		{name: "resumed with known id skips discovery", resumed: true, known: "abc123", expected: false},
		{name: "resumed with whitespace known id discovers id", resumed: true, known: "  ", expected: true},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := shouldDiscoverCodexSessionID(tt.resumed, tt.known); got != tt.expected {
				t.Fatalf("shouldDiscoverCodexSessionID(%v, %q) = %v, want %v", tt.resumed, tt.known, got, tt.expected)
			}
		})
	}
}
