package domain

import (
	"strings"
	"testing"
)

func TestMergeIfUnset(t *testing.T) {
	dst := map[string]string{"A": "1", "B": "2"}
	src := map[string]string{"B": "override", "C": "3"}
	MergeIfUnset(dst, src)
	if dst["A"] != "1" || dst["B"] != "2" || dst["C"] != "3" {
		t.Fatalf("unexpected merge result: %#v", dst)
	}
}

func TestEnvHelpers(t *testing.T) {
	m := EnvSliceToMap([]string{" A = x ", "", "B=y", "BROKEN"})
	if m["A"] != "x" || m["B"] != "y" {
		t.Fatalf("EnvSliceToMap unexpected: %#v", m)
	}

	env := EnvironToMap([]string{"K=V", "X=Y=Z", "BAD"})
	if env["K"] != "V" || env["X"] != "Y=Z" {
		t.Fatalf("EnvironToMap unexpected: %#v", env)
	}

	lines := EnvMapToSlice(map[string]string{"ONE": "1", "TWO": "2"})
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "ONE=1") || !strings.Contains(joined, "TWO=2") {
		t.Fatalf("EnvMapToSlice unexpected: %#v", lines)
	}
}

func TestBranchPrefixForTemplate(t *testing.T) {
	if got := BranchPrefixForTemplate("claude"); got != "claude" {
		t.Fatalf("claude prefix: %q", got)
	}
	if got := BranchPrefixForTemplate("agent-dev"); got != "claude" {
		t.Fatalf("agent-dev prefix: %q", got)
	}
	if got := BranchPrefixForTemplate("codex"); got != "codex" {
		t.Fatalf("codex prefix: %q", got)
	}
	if got := BranchPrefixForTemplate("something-else"); got != "dockpipe" {
		t.Fatalf("default prefix: %q", got)
	}
}

func TestFromResolverMap(t *testing.T) {
	r := FromResolverMap(map[string]string{
		"DOCKPIPE_RESOLVER_TEMPLATE":   "codex",
		"DOCKPIPE_RESOLVER_PRE_SCRIPT": "scripts/pre.sh",
		"DOCKPIPE_RESOLVER_ACTION":     "actions/do.sh",
		"DOCKPIPE_RESOLVER_CMD":        "codex",
		"DOCKPIPE_RESOLVER_ENV":        "OPENAI_API_KEY",
		"DOCKPIPE_RESOLVER_EXPERIMENTAL": "1",
	})
	if r.Template != "codex" || r.PreScript != "scripts/pre.sh" || r.Action != "actions/do.sh" {
		t.Fatalf("unexpected resolver assignments: %#v", r)
	}
	if r.Cmd != "codex" || r.EnvHint != "OPENAI_API_KEY" || !r.Experimental {
		t.Fatalf("unexpected extended fields: %#v", r)
	}
}
