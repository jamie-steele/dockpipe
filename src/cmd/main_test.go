package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDecideInitGitignoreFlagAppliesWithoutPrompt(t *testing.T) {
	var out bytes.Buffer
	argv, decision, err := decideInitGitignore([]string{"init", "--gitignore"}, true, strings.NewReader("n\n"), &out)
	if err != nil {
		t.Fatal(err)
	}
	if decision != gitignoreApply {
		t.Fatalf("decision=%v, want apply", decision)
	}
	if got := strings.Join(argv, " "); got != "init" {
		t.Fatalf("argv=%q, want %q", got, "init")
	}
	if out.Len() != 0 {
		t.Fatalf("expected no prompt output, got %q", out.String())
	}
}

func TestDecideInitGitignorePromptsDefaultYes(t *testing.T) {
	var out bytes.Buffer
	argv, decision, err := decideInitGitignore([]string{"init"}, true, strings.NewReader("\n"), &out)
	if err != nil {
		t.Fatal(err)
	}
	if decision != gitignoreApply {
		t.Fatalf("decision=%v, want apply", decision)
	}
	if got := strings.Join(argv, " "); got != "init" {
		t.Fatalf("argv=%q, want %q", got, "init")
	}
	if !strings.Contains(out.String(), "Add recommended DockPipe .gitignore? (Y/n)") {
		t.Fatalf("missing prompt, got %q", out.String())
	}
}

func TestDecideInitGitignorePromptNoDeclines(t *testing.T) {
	var out bytes.Buffer
	argv, decision, err := decideInitGitignore([]string{"init"}, true, strings.NewReader("n\n"), &out)
	if err != nil {
		t.Fatal(err)
	}
	if decision != gitignoreDeclined {
		t.Fatalf("decision=%v, want declined", decision)
	}
	if got := strings.Join(argv, " "); got != "init" {
		t.Fatalf("argv=%q, want %q", got, "init")
	}
}

func TestDecideInitGitignoreNonInteractiveSkipsPrompt(t *testing.T) {
	var out bytes.Buffer
	argv, decision, err := decideInitGitignore([]string{"init"}, false, strings.NewReader(""), &out)
	if err != nil {
		t.Fatal(err)
	}
	if decision != gitignoreNoop {
		t.Fatalf("decision=%v, want noop", decision)
	}
	if got := strings.Join(argv, " "); got != "init" {
		t.Fatalf("argv=%q, want %q", got, "init")
	}
	if out.Len() != 0 {
		t.Fatalf("expected no output, got %q", out.String())
	}
}

func TestDecideInitGitignoreInitWithArgsDoesNotPrompt(t *testing.T) {
	var out bytes.Buffer
	argv, decision, err := decideInitGitignore([]string{"init", "my-workflow"}, true, strings.NewReader(""), &out)
	if err != nil {
		t.Fatal(err)
	}
	if decision != gitignoreNoop {
		t.Fatalf("decision=%v, want noop", decision)
	}
	if got := strings.Join(argv, " "); got != "init my-workflow" {
		t.Fatalf("argv=%q, want %q", got, "init my-workflow")
	}
	if out.Len() != 0 {
		t.Fatalf("expected no prompt output, got %q", out.String())
	}
}

func TestEnsureDockpipeGitignoreCreatesFile(t *testing.T) {
	dir := t.TempDir()
	added, err := ensureDockpipeGitignore(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !added {
		t.Fatal("expected added=true on first run")
	}
	b, err := os.ReadFile(filepath.Join(dir, ".gitignore"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(b)
	for _, entry := range dockpipeGitignoreEntries {
		if !strings.Contains(text, entry+"\n") {
			t.Fatalf("missing entry %q in:\n%s", entry, text)
		}
	}
}

func TestEnsureDockpipeGitignoreAppendsOnlyMissingAndIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".gitignore")
	initial := "node_modules/\n.dockpipe/\n"
	if err := os.WriteFile(path, []byte(initial), 0o644); err != nil {
		t.Fatal(err)
	}

	added, err := ensureDockpipeGitignore(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !added {
		t.Fatal("expected added=true when some entries are missing")
	}
	once, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(once), "node_modules/\n") {
		t.Fatalf("existing content should be preserved, got:\n%s", once)
	}
	lines := strings.Split(strings.ReplaceAll(string(once), "\r\n", "\n"), "\n")
	count := 0
	for _, line := range lines {
		if line == ".dockpipe/" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("expected no duplicate .dockpipe/ line, count=%d\n%s", count, once)
	}

	added, err = ensureDockpipeGitignore(dir)
	if err != nil {
		t.Fatal(err)
	}
	if added {
		t.Fatal("expected added=false on second run")
	}
	twice, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(once) != string(twice) {
		t.Fatalf("second run should not change file")
	}
}
