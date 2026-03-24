package infrastructure

import (
	"strings"
	"testing"
)

func TestIsSafeDockerContainerName(t *testing.T) {
	t.Parallel()
	cases := []struct {
		s    string
		want bool
	}{
		{"dockpipe-cursor-dev-12345", true},
		{"a", true},
		{"", false},
		{"-bad", false},
		{"foo;bar", false},
		{strings.Repeat("a", 300), false},
	}
	for _, tc := range cases {
		got := isSafeDockerContainerName(tc.s)
		if got != tc.want {
			t.Errorf("%q: got %v want %v", tc.s, got, tc.want)
		}
	}
}

func TestHostCleanupSkip(t *testing.T) {
	t.Parallel()
	env := []string{"DOCKPIPE_SKIP_HOST_CLEANUP=1", "DOCKPIPE_WORKDIR=/tmp"}
	if !hostCleanupSkip(env) {
		t.Fatal("expected skip")
	}
}
