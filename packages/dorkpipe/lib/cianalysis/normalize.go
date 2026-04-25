package cianalysis

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"dorkpipe.orchestrator/statepaths"
)

type Finding struct {
	Tool        string          `json:"tool"`
	RuleID      string          `json:"rule_id"`
	Title       string          `json:"title"`
	File        string          `json:"file"`
	Line        int             `json:"line"`
	Column      int             `json:"column"`
	Severity    string          `json:"severity"`
	Confidence  string          `json:"confidence"`
	Category    string          `json:"category"`
	Message     string          `json:"message"`
	Remediation *string         `json:"remediation"`
	Raw         json.RawMessage `json:"raw"`
	ID          string          `json:"id"`
}

type findingsEnvelope struct {
	SchemaVersion string `json:"schema_version"`
	Provenance    struct {
		Commit             string `json:"commit"`
		BranchOrRef        string `json:"branch_or_ref"`
		WorkflowRunID      string `json:"workflow_run_id"`
		WorkflowRunAttempt string `json:"workflow_run_attempt"`
		WorkflowName       string `json:"workflow_name"`
		Repository         string `json:"repository"`
		TimestampUTC       string `json:"timestamp_utc"`
		Source             string `json:"source"`
		Tools              struct {
			Gosec       string `json:"gosec"`
			Govulncheck string `json:"govulncheck"`
		} `json:"tools"`
	} `json:"provenance"`
	Findings []Finding `json:"findings"`
	RawPaths struct {
		Gosec       string `json:"gosec"`
		Govulncheck string `json:"govulncheck"`
	} `json:"raw_paths"`
}

type gosecDoc struct {
	Issues       []gosecIssue `json:"Issues"`
	GosecVersion string       `json:"GosecVersion"`
}

type gosecIssue struct {
	RuleID     string      `json:"rule_id"`
	Details    string      `json:"details"`
	File       string      `json:"file"`
	Line       interface{} `json:"line"`
	Column     interface{} `json:"column"`
	Severity   string      `json:"severity"`
	Confidence string      `json:"confidence"`
	CWE        *struct {
		ID interface{} `json:"id"`
	} `json:"cwe"`
}

type govulnDoc struct {
	Config *struct {
		ScannerVersion string `json:"scanner_version"`
	} `json:"config"`
	ScannerVersion string        `json:"ScannerVersion"`
	Vulns          []govulnEntry `json:"vulns"`
	VulnsAlt       []govulnEntry `json:"Vulns"`
}

type govulnEntry struct {
	OSV    *govOSV `json:"osv"`
	OSVAlt *govOSV `json:"OSV"`
}

type govOSV struct {
	ID       string `json:"id"`
	Summary  string `json:"summary"`
	Details  string `json:"details"`
	Severity []struct {
		Type  string      `json:"type"`
		Score interface{} `json:"score"`
	} `json:"severity"`
}

type Result struct {
	FindingsPath string
	SummaryPath  string
	Count        int
}

func Normalize(workdir string, env map[string]string) (Result, error) {
	root, err := filepath.Abs(workdir)
	if err != nil {
		return Result{}, err
	}
	rawDir := statepaths.CIRawDir(root)
	outDir := statepaths.CIAnalysisDir(root)
	rawOutDir := filepath.Join(outDir, "raw")
	gosecPath := filepath.Join(rawDir, "gosec.json")
	govPath := filepath.Join(rawDir, "govulncheck.json")

	if err := os.RemoveAll(outDir); err != nil {
		return Result{}, err
	}
	if err := os.MkdirAll(rawOutDir, 0o755); err != nil {
		return Result{}, err
	}
	if err := ensureJSONFile(gosecPath); err != nil {
		return Result{}, err
	}
	if err := ensureJSONFile(govPath); err != nil {
		return Result{}, err
	}
	if err := copyFile(gosecPath, filepath.Join(rawOutDir, "gosec.json")); err != nil {
		return Result{}, err
	}
	if err := copyFile(govPath, filepath.Join(rawOutDir, "govulncheck.json")); err != nil {
		return Result{}, err
	}

	gosecBytes, err := os.ReadFile(gosecPath)
	if err != nil {
		return Result{}, err
	}
	govBytes, err := os.ReadFile(govPath)
	if err != nil {
		return Result{}, err
	}

	var gosec gosecDoc
	if err := json.Unmarshal(gosecBytes, &gosec); err != nil {
		return Result{}, fmt.Errorf("parse gosec.json: %w", err)
	}
	var gov govulnDoc
	if err := json.Unmarshal(govBytes, &gov); err != nil {
		return Result{}, fmt.Errorf("parse govulncheck.json: %w", err)
	}

	findings := make([]Finding, 0, len(gosec.Issues)+len(gov.Vulns)+len(gov.VulnsAlt))
	for _, issue := range gosec.Issues {
		raw, _ := json.Marshal(issue)
		category := "sast"
		if issue.CWE != nil {
			if id := strings.TrimSpace(fmt.Sprint(issue.CWE.ID)); id != "" && id != "<nil>" {
				category = "CWE-" + id
			}
		}
		f := Finding{
			Tool:        "gosec",
			RuleID:      orDefault(issue.RuleID, "unknown"),
			Title:       issue.Details,
			File:        issue.File,
			Line:        toInt(issue.Line),
			Column:      toInt(issue.Column),
			Severity:    issue.Severity,
			Confidence:  issue.Confidence,
			Category:    category,
			Message:     issue.Details,
			Remediation: nil,
			Raw:         raw,
		}
		f.ID = fmt.Sprintf("%s|%s|%s|%d", f.Tool, f.RuleID, f.File, f.Line)
		findings = append(findings, f)
	}

	govEntries := gov.Vulns
	if len(govEntries) == 0 {
		govEntries = gov.VulnsAlt
	}
	for _, entry := range govEntries {
		osv := entry.OSV
		if osv == nil {
			osv = entry.OSVAlt
		}
		if osv == nil {
			osv = &govOSV{}
		}
		raw, _ := json.Marshal(entry)
		severity := "unknown"
		if len(osv.Severity) > 0 {
			if s := strings.TrimSpace(osv.Severity[0].Type); s != "" {
				severity = s
			} else if osv.Severity[0].Score != nil {
				severity = strings.TrimSpace(fmt.Sprint(osv.Severity[0].Score))
			}
		}
		msg := truncate(orDefault(osv.Details, osv.Summary), 2000)
		remediation := "Upgrade affected module(s); inspect raw.govulncheck and module traces."
		f := Finding{
			Tool:        "govulncheck",
			RuleID:      orDefault(osv.ID, "unknown"),
			Title:       orDefault(osv.Summary, orDefault(osv.ID, "vulnerability")),
			File:        "",
			Line:        0,
			Column:      0,
			Severity:    severity,
			Confidence:  "",
			Category:    "dependency-vuln",
			Message:     msg,
			Remediation: &remediation,
			Raw:         raw,
		}
		f.ID = fmt.Sprintf("%s|%s|%s|%d", f.Tool, f.RuleID, f.File, f.Line)
		findings = append(findings, f)
	}

	findingsPath := statepaths.CIFindingsPath(root)
	summaryPath := statepaths.CISummaryPath(root)
	now := time.Now().UTC().Format(time.RFC3339)
	commit := gitOutput(root, "rev-parse", "HEAD")
	branch := strings.TrimSpace(env["GITHUB_REF_NAME"])
	if branch == "" {
		branch = strings.TrimSpace(env["DOCKPIPE_GIT_BRANCH"])
	}
	if branch == "" {
		branch = gitOutput(root, "rev-parse", "--abbrev-ref", "HEAD")
	}
	if branch == "" {
		branch = "unknown"
	}
	runID := orDefault(env["GITHUB_RUN_ID"], "local")
	runAttempt := orDefault(env["GITHUB_RUN_ATTEMPT"], "1")
	workflowName := strings.TrimSpace(env["GITHUB_WORKFLOW"])
	if workflowName == "" {
		workflowName = strings.TrimSpace(env["DOCKPIPE_WORKFLOW_NAME"])
	}
	if workflowName == "" {
		workflowName = "local"
	}
	repository := orDefault(env["GITHUB_REPOSITORY"], "unknown")
	source := "local"
	if strings.TrimSpace(env["GITHUB_ACTIONS"]) != "" {
		source = "ci"
	}
	gosecVer := orDefault(gosec.GosecVersion, "unknown")
	govVer := "unknown"
	if gov.Config != nil && strings.TrimSpace(gov.Config.ScannerVersion) != "" {
		govVer = strings.TrimSpace(gov.Config.ScannerVersion)
	} else if strings.TrimSpace(gov.ScannerVersion) != "" {
		govVer = strings.TrimSpace(gov.ScannerVersion)
	}

	var doc findingsEnvelope
	doc.SchemaVersion = "1.0"
	doc.Provenance.Commit = commit
	doc.Provenance.BranchOrRef = branch
	doc.Provenance.WorkflowRunID = runID
	doc.Provenance.WorkflowRunAttempt = runAttempt
	doc.Provenance.WorkflowName = workflowName
	doc.Provenance.Repository = repository
	doc.Provenance.TimestampUTC = now
	doc.Provenance.Source = source
	doc.Provenance.Tools.Gosec = gosecVer
	doc.Provenance.Tools.Govulncheck = govVer
	doc.Findings = findings
	doc.RawPaths.Gosec = "raw/gosec.json"
	doc.RawPaths.Govulncheck = "raw/govulncheck.json"

	blob, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return Result{}, err
	}
	blob = append(blob, '\n')
	if err := os.WriteFile(findingsPath, blob, 0o644); err != nil {
		return Result{}, err
	}

	gosecCount := len(gosec.Issues)
	govCount := len(govEntries)
	summary := fmt.Sprintf(`# CI scan summary (DorkPipe signal bundle)

- **Schema:** %s — see %s
- **Commit:** %s · **ref:** %s · **time (UTC):** %s
- **Run:** %s attempt %s · **workflow:** %s
- **Normalized findings:** **%d** (gosec issues in raw: ~%d · govulncheck vulns in raw: ~%d)

**Artifacts:** %s (machine-readable), %s (original tool JSON), this file (human).

**DorkPipe:** load %s to classify, correlate with repo analysis, prioritize, and suggest fixes. Compare %s across runs for new/resolved/changed severity.

See **docs/artifacts.md** (CI bundle).
`,
		backtick("1.0")+"",
		backtick("src/schemas/dockpipe-ci-findings.schema.json"),
		backtick(commit),
		backtick(branch),
		now,
		backtick(runID),
		backtick(runAttempt),
		backtick(workflowName),
		len(findings),
		gosecCount,
		govCount,
		backtick("findings.json"),
		backtick("raw/"),
		backtick("bin/.dockpipe/ci-analysis/findings.json"),
		backtick("findings[].id"),
	)
	if err := os.WriteFile(summaryPath, []byte(summary), 0o644); err != nil {
		return Result{}, err
	}
	return Result{FindingsPath: findingsPath, SummaryPath: summaryPath, Count: len(findings)}, nil
}

func ensureJSONFile(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	return os.WriteFile(path, []byte("{}\n"), 0o644)
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0o644)
}

func gitOutput(root string, args ...string) string {
	cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
	out, err := cmd.Output()
	if err != nil {
		if len(args) == 2 && args[0] == "rev-parse" && args[1] == "HEAD" {
			return "unknown"
		}
		return "unknown"
	}
	v := strings.TrimSpace(string(out))
	if v == "" {
		return "unknown"
	}
	return v
}

func toInt(v interface{}) int {
	switch t := v.(type) {
	case float64:
		return int(t)
	case int:
		return t
	case json.Number:
		n, _ := t.Int64()
		return int(n)
	case string:
		n, err := strconv.Atoi(strings.TrimSpace(t))
		if err == nil {
			return n
		}
	}
	return 0
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max]
}

func orDefault(v, d string) string {
	if strings.TrimSpace(v) == "" {
		return d
	}
	return strings.TrimSpace(v)
}

func backtick(s string) string {
	return "`" + s + "`"
}
