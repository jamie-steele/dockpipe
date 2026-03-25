package scheduler

import (
	"reflect"
	"testing"

	"dockpipe/src/lib/dorkpipe/spec"
)

func TestLevelsParallel(t *testing.T) {
	d := &spec.Doc{
		Name: "t",
		Nodes: []spec.Node{
			{ID: "root", Kind: "shell", Script: "true"},
			{ID: "p1", Kind: "shell", Script: "true", Needs: []string{"root"}},
			{ID: "p2", Kind: "shell", Script: "true", Needs: []string{"root"}},
			{ID: "join", Kind: "shell", Script: "true", Needs: []string{"p1", "p2"}},
		},
	}
	levels, err := Levels(d, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(levels) != 3 {
		t.Fatalf("levels: %#v", levels)
	}
	if !reflect.DeepEqual(levels[1], []string{"p1", "p2"}) {
		t.Fatalf("expected p1 p2 parallel, got %#v", levels[1])
	}
}

func TestLevelsSkipsEscalateCodex(t *testing.T) {
	d := &spec.Doc{
		Name: "t",
		Nodes: []spec.Node{
			{ID: "a", Kind: "shell", Script: "true"},
			{ID: "c", Kind: "codex", DockpipeArgs: []string{"--version"}, EscalateOnly: true, Needs: []string{"a"}},
		},
	}
	levels, err := Levels(d, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(levels) != 1 || levels[0][0] != "a" {
		t.Fatalf("got %#v", levels)
	}
}
