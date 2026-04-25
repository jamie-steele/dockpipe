package handoff

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildCursor(t *testing.T) {
	root := t.TempDir()
	out := filepath.Join(root, "bin", ".dockpipe", "packages", "dorkpipe", "self-analysis")
	if err := os.MkdirAll(out, 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(rel, body string) {
		path := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("bin/.dockpipe/packages/dorkpipe/self-analysis/dorkpipe_go_files.count", "12\n")
	write("bin/.dockpipe/packages/dorkpipe/self-analysis/git.txt", "commit abcdef\nAuthor: test\nDate: now\nextra\n")
	write("bin/.dockpipe/packages/dorkpipe/self-analysis/dorkpipe_packages.tsv", "pkg\t3\n")
	write("bin/.dockpipe/packages/dorkpipe/self-analysis/workflow_configs.txt", "workflow-a\n")
	write("bin/.dockpipe/packages/dorkpipe/self-analysis/signals_git_log.txt", "fix thing\n")
	write("bin/.dockpipe/packages/dorkpipe/self-analysis/signals_engine_files.txt", "packages/dorkpipe/lib/engine/run.go\n")
	write("bin/.dockpipe/packages/dorkpipe/self-analysis/key_file_wc.txt", "120 packages/dorkpipe/lib/engine/run.go\n")
	write("bin/.dockpipe/packages/dorkpipe/self-analysis/signals_metrics_tail.txt", "metric\n")
	write("bin/.dockpipe/packages/dorkpipe/metrics.jsonl", "{}\n")
	write("packages/dorkpipe/resolvers/dorkpipe-orchestrator/spec.example.yaml", "nodes:\n  - kind: worker\n")
	write("packages/dorkpipe/lib/examples/full-bar.yaml", "name: demo\n")

	res, err := BuildCursor(root, map[string]string{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Bytes == 0 {
		t.Fatal("expected non-zero document size")
	}
	doc, err := os.ReadFile(res.DocumentPath)
	if err != nil {
		t.Fatal(err)
	}
	text := string(doc)
	if !strings.Contains(text, "- **Go files in `lib/dorkpipe`**: 12") {
		t.Fatalf("missing go count in document: %s", text)
	}
	if !strings.Contains(text, "independent verifier") {
		t.Fatalf("expected verifier recommendation when spec lacks verifier: %s", text)
	}
	paste, err := os.ReadFile(res.PastePath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(paste), "Priority: orchestrator spec.example.yaml still lacks a kind: verifier node") {
		t.Fatalf("missing verifier priority in paste prompt: %s", string(paste))
	}
}
