package handoff

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestComplianceSummary(t *testing.T) {
	root := t.TempDir()
	write := func(rel, body string) {
		path := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	write("bin/.dockpipe/ci-analysis/findings.json", `{"schema_version":"1.0","provenance":{"commit":"abc123","source":"ci"},"findings":[{},{}]}`)
	write("bin/.dockpipe/ci-analysis/SUMMARY.md", "# Summary\nline2\n")
	write("bin/.dockpipe/packages/dorkpipe/self-analysis/git.txt", "commit abc\n")
	write("bin/.dockpipe/packages/dorkpipe/run.json", `{"name":"demo","ts":"now","policy":{"mode":"strict"},"extra":"x"}`)
	write("bin/.dockpipe/analysis/insights.json", `{"kind":"dockpipe_user_insights","insights":[{"category":"risk"},{"category":"compliance"},{"category":"risk"}]}`)

	out, err := ComplianceSummary(root)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "schema: 1.0 | findings: 2 | commit: abc123 | source: ci") {
		t.Fatalf("missing findings summary: %s", out)
	}
	if !strings.Contains(out, "\"name\": \"demo\"") {
		t.Fatalf("missing run summary: %s", out)
	}
	if !strings.Contains(out, "\"insight_count\": 3") || !strings.Contains(out, "\"compliance\"") {
		t.Fatalf("missing insights summary: %s", out)
	}
}
