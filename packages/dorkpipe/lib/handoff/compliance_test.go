package handoff

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dorkpipe.orchestrator/statepaths"
)

func TestComplianceSummary(t *testing.T) {
	root := t.TempDir()

	findingsPath, err := statepaths.PackageCIFindingsPath(root)
	if err != nil {
		t.Fatal(err)
	}
	summaryPath, err := statepaths.PackageCISummaryPath(root)
	if err != nil {
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
	writeAbs(findingsPath, `{"schema_version":"1.0","provenance":{"commit":"abc123","source":"ci"},"findings":[{},{}]}`)
	writeAbs(summaryPath, "# Summary\nline2\n")
	selfAnalysisDir, err := statepaths.SelfAnalysisDir(root)
	if err != nil {
		t.Fatal(err)
	}
	runPath, err := statepaths.RunPath(root)
	if err != nil {
		t.Fatal(err)
	}
	writeAbs(filepath.Join(selfAnalysisDir, "git.txt"), "commit abc\n")
	writeAbs(runPath, `{"name":"demo","ts":"now","policy":{"mode":"strict"},"extra":"x"}`)
	writeAbs(statepaths.InsightsPath(root), `{"kind":"dockpipe_user_insights","insights":[{"category":"risk"},{"category":"compliance"},{"category":"risk"}]}`)

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
