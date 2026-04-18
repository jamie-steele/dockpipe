package handoff

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"dorkpipe.orchestrator/statepaths"
)

func ComplianceSummary(workdir string) (string, error) {
	root, err := filepath.Abs(workdir)
	if err != nil {
		return "", err
	}

	var b strings.Builder
	b.WriteString("\n=== DockPipe — compliance & security posture handoff (signals only) ===\n")
	b.WriteString("Read: docs/artifacts.md\n\n")

	findingsPath := statepaths.CIFindingsPath(root)
	if fileExists(findingsPath) {
		b.WriteString("--- bin/.dockpipe/ci-analysis/ (CI-normalized signals) ---\n")
		b.WriteString(findingsSummary(findingsPath))
		b.WriteString("\n")
	} else {
		b.WriteString("[ ] bin/.dockpipe/ci-analysis/findings.json — run: bash src/scripts/ci-local.sh (or CI) to generate\n")
	}

	summaryPath := statepaths.CISummaryPath(root)
	if fileExists(summaryPath) {
		b.WriteString("\n--- SUMMARY.md (head) ---\n")
		for _, line := range headLines(summaryPath, 15) {
			b.WriteString(line + "\n")
		}
	}

	selfAnalysisDir, err := statepaths.SelfAnalysisDir(root)
	if err == nil && dirHasEntries(selfAnalysisDir) {
		b.WriteString("\n--- bin/.dockpipe/packages/dorkpipe/self-analysis/ (present) ---\n")
		for _, line := range dirSummary(selfAnalysisDir, 20) {
			b.WriteString(line + "\n")
		}
	}

	runPath, err := statepaths.RunPath(root)
	if err == nil && fileExists(runPath) {
		b.WriteString("\n--- bin/.dockpipe/packages/dorkpipe/run.json ---\n")
		b.WriteString(runSummary(runPath))
	}

	insightsPath := statepaths.InsightsPath(root)
	if fileExists(insightsPath) {
		b.WriteString("\n--- bin/.dockpipe/analysis/insights.json (user guidance signals; not verified facts) ---\n")
		b.WriteString(insightsSummary(insightsPath))
	}

	b.WriteString("\nAI: Answer compliance/security questions using AGENTS.md + artifacts above; do not claim certified compliance.\n")
	b.WriteString("See docs/artifacts.md\n")
	return b.String(), nil
}

func findingsSummary(path string) string {
	var doc struct {
		SchemaVersion string `json:"schema_version"`
		Provenance    struct {
			Commit string `json:"commit"`
			Source string `json:"source"`
		} `json:"provenance"`
		Findings []any `json:"findings"`
	}
	if body, err := os.ReadFile(path); err == nil && json.Unmarshal(body, &doc) == nil {
		return fmt.Sprintf("schema: %s | findings: %d | commit: %s | source: %s\n", doc.SchemaVersion, len(doc.Findings), doc.Provenance.Commit, doc.Provenance.Source)
	}
	lines := dirSummary(filepath.Dir(path), 20)
	if len(lines) == 0 {
		return ""
	}
	return strings.Join(lines, "\n") + "\n"
}

func runSummary(path string) string {
	var doc map[string]any
	if body, err := os.ReadFile(path); err == nil && json.Unmarshal(body, &doc) == nil {
		out := map[string]any{}
		for _, key := range []string{"name", "ts", "policy"} {
			if v, ok := doc[key]; ok {
				out[key] = v
			}
		}
		if body, err := json.MarshalIndent(out, "", "  "); err == nil {
			return string(body) + "\n"
		}
	}
	lines := headLines(path, 5)
	if len(lines) == 0 {
		return ""
	}
	return strings.Join(lines, "\n") + "\n"
}

func insightsSummary(path string) string {
	var doc struct {
		Kind     string `json:"kind"`
		Insights []struct {
			Category string `json:"category"`
		} `json:"insights"`
	}
	if body, err := os.ReadFile(path); err == nil && json.Unmarshal(body, &doc) == nil {
		categories := make(map[string]struct{})
		for _, insight := range doc.Insights {
			if insight.Category != "" {
				categories[insight.Category] = struct{}{}
			}
		}
		names := make([]string, 0, len(categories))
		for name := range categories {
			names = append(names, name)
		}
		sort.Strings(names)
		body, _ := json.MarshalIndent(map[string]any{
			"kind":          doc.Kind,
			"insight_count": len(doc.Insights),
			"categories":    names,
		}, "", "  ")
		return string(body) + "\n"
	}
	lines := headLines(path, 8)
	if len(lines) == 0 {
		return ""
	}
	return strings.Join(lines, "\n") + "\n"
}

func dirHasEntries(path string) bool {
	entries, err := os.ReadDir(path)
	return err == nil && len(entries) > 0
}

func dirSummary(path string, limit int) []string {
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil
	}
	lines := make([]string, 0, len(entries))
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue
		}
		mode := info.Mode().String()
		size := info.Size()
		lines = append(lines, fmt.Sprintf("%s %8d %s", mode, size, entry.Name()))
	}
	sort.Strings(lines)
	if len(lines) > limit {
		return lines[:limit]
	}
	return lines
}
