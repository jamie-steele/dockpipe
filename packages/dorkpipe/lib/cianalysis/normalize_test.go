package cianalysis

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNormalizeWritesFindingsAndSummary(t *testing.T) {
	tmp := t.TempDir()
	rawDir := filepath.Join(tmp, "bin", ".dockpipe", "ci-raw")
	if err := os.MkdirAll(rawDir, 0o755); err != nil {
		t.Fatal(err)
	}
	gosec := `{"Issues":[{"rule_id":"G101","details":"hardcoded secret","file":"main.go","line":"7","column":"3","severity":"HIGH","confidence":"HIGH","cwe":{"id":"798"}}],"GosecVersion":"fixture-gosec"}`
	gov := `{"config":{"scanner_version":"fixture-govuln"},"vulns":[{"osv":{"id":"GO-2024-0001","summary":"summary","details":"details","severity":[{"type":"CVSS_V3","score":"9.8"}]}}]}`
	if err := os.WriteFile(filepath.Join(rawDir, "gosec.json"), []byte(gosec), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(rawDir, "govulncheck.json"), []byte(gov), 0o644); err != nil {
		t.Fatal(err)
	}

	res, err := Normalize(tmp, map[string]string{
		"GITHUB_WORKFLOW":   "ci",
		"GITHUB_REPOSITORY": "dockpipe/dockpipe",
	})
	if err != nil {
		t.Fatalf("Normalize() error = %v", err)
	}
	if res.Count != 2 {
		t.Fatalf("Normalize() count = %d, want 2", res.Count)
	}

	findingsBytes, err := os.ReadFile(res.FindingsPath)
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		SchemaVersion string `json:"schema_version"`
		Provenance    struct {
			WorkflowName string `json:"workflow_name"`
			Repository   string `json:"repository"`
			Tools        struct {
				Gosec       string `json:"gosec"`
				Govulncheck string `json:"govulncheck"`
			} `json:"tools"`
		} `json:"provenance"`
		Findings []Finding `json:"findings"`
	}
	if err := json.Unmarshal(findingsBytes, &doc); err != nil {
		t.Fatal(err)
	}
	if doc.SchemaVersion != "1.0" {
		t.Fatalf("schema_version = %q, want 1.0", doc.SchemaVersion)
	}
	if doc.Provenance.WorkflowName != "ci" {
		t.Fatalf("workflow_name = %q, want ci", doc.Provenance.WorkflowName)
	}
	if doc.Provenance.Repository != "dockpipe/dockpipe" {
		t.Fatalf("repository = %q", doc.Provenance.Repository)
	}
	if doc.Provenance.Tools.Gosec != "fixture-gosec" || doc.Provenance.Tools.Govulncheck != "fixture-govuln" {
		t.Fatalf("tool versions = %#v", doc.Provenance.Tools)
	}
	if len(doc.Findings) != 2 {
		t.Fatalf("findings length = %d", len(doc.Findings))
	}
	if doc.Findings[0].ID == "" || !strings.Contains(doc.Findings[0].ID, "gosec|G101|main.go|7") {
		t.Fatalf("first finding id = %q", doc.Findings[0].ID)
	}
	if doc.Findings[0].Category != "CWE-798" {
		t.Fatalf("first finding category = %q", doc.Findings[0].Category)
	}
	if doc.Findings[1].Remediation == nil || *doc.Findings[1].Remediation == "" {
		t.Fatalf("second finding remediation missing")
	}

	summaryBytes, err := os.ReadFile(res.SummaryPath)
	if err != nil {
		t.Fatal(err)
	}
	summary := string(summaryBytes)
	if !strings.Contains(summary, "Normalized findings:** **2**") {
		t.Fatalf("summary missing findings count: %s", summary)
	}
	if !strings.Contains(summary, "`ci`") {
		t.Fatalf("summary missing workflow name: %s", summary)
	}
}
