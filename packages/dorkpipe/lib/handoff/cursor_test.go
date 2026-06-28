package handoff

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dorkpipe.orchestrator/statepaths"
)

func TestBuildCursor(t *testing.T) {
	root := t.TempDir()
	out, err := statepaths.SelfAnalysisDir(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(out, 0o755); err != nil {
		t.Fatal(err)
	}
	writeAbs := func(path, body string) {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	writeRel := func(rel, body string) {
		writeAbs(filepath.Join(root, rel), body)
	}
	writeAbs(filepath.Join(out, "dorkpipe_go_files.count"), "12\n")
	writeAbs(filepath.Join(out, "git.txt"), "commit abcdef\nAuthor: test\nDate: now\nextra\n")
	writeAbs(filepath.Join(out, "dorkpipe_packages.tsv"), "pkg\t3\n")
	writeAbs(filepath.Join(out, "workflow_configs.txt"), "workflow-a\n")
	writeAbs(filepath.Join(out, "signals_git_log.txt"), "fix thing\n")
	writeAbs(filepath.Join(out, "signals_engine_files.txt"), "packages/dorkpipe/lib/engine/run.go\n")
	writeAbs(filepath.Join(out, "key_file_wc.txt"), "120 packages/dorkpipe/lib/engine/run.go\n")
	writeAbs(filepath.Join(out, "signals_metrics_tail.txt"), "metric\n")
	metricsPath, err := statepaths.MetricsPath(root)
	if err != nil {
		t.Fatal(err)
	}
	writeAbs(metricsPath, "{}\n")
	writeRel("packages/dorkpipe/resolvers/dorkpipe-orchestrator/spec.example.yaml", "nodes:\n  - kind: worker\n")
	writeRel("packages/dorkpipe/lib/examples/full-bar.yaml", "name: demo\n")

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
