package domain

import (
	"reflect"
	"testing"
)

func TestFromStrategyMap(t *testing.T) {
	m := map[string]string{
		"DOCKPIPE_STRATEGY_BEFORE": "scripts/a.sh, scripts/b.sh",
		"DOCKPIPE_STRATEGY_AFTER":  "scripts/commit-worktree.sh",
		"DOCKPIPE_STRATEGY_KIND":   "git",
	}
	a := FromStrategyMap(m)
	wantBefore := []string{"scripts/a.sh", "scripts/b.sh"}
	if !reflect.DeepEqual(a.Before, wantBefore) {
		t.Fatalf("Before: got %#v want %#v", a.Before, wantBefore)
	}
	if len(a.After) != 1 || a.After[0] != "scripts/commit-worktree.sh" {
		t.Fatalf("After: got %#v", a.After)
	}
	if a.Kind != "git" {
		t.Fatalf("Kind: %q", a.Kind)
	}
}

func TestFromStrategyMapEmpty(t *testing.T) {
	a := FromStrategyMap(nil)
	if a.Before != nil || a.After != nil || a.Kind != "" {
		t.Fatalf("got %#v", a)
	}
}

func TestParseWorkflowYAMLStrategyFields(t *testing.T) {
	y := `
name: t
strategy: git-worktree
strategies:
  - git-worktree
  - git-commit
`
	w, err := ParseWorkflowYAML([]byte(y))
	if err != nil {
		t.Fatal(err)
	}
	if w.Strategy != "git-worktree" {
		t.Fatalf("strategy: %q", w.Strategy)
	}
	if len(w.Strategies) != 2 || w.Strategies[0] != "git-worktree" {
		t.Fatalf("strategies: %#v", w.Strategies)
	}
}
