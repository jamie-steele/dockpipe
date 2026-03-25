// Package promotion proposes promotions from repeated run artifacts (lightweight heuristic).
package promotion

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"dockpipe/src/lib/dorkpipe/eval"
)

// Suggestion is a human-readable promotion hint.
type Suggestion struct {
	Kind   string `json:"kind"`
	Detail string `json:"detail"`
}

// Analyze inspects .dorkpipe under workdir and returns suggestions.
func Analyze(workdir string) ([]Suggestion, error) {
	dir := filepath.Join(workdir, ".dorkpipe")
	metricsPath := filepath.Join(dir, "metrics.jsonl")
	var out []Suggestion
	st, err := eval.SummarizeFile(metricsPath)
	if err == nil && st.Lines > 0 {
		if st.EscalationRate > 0.4 {
			out = append(out, Suggestion{
				Kind:   "escalation_rate",
				Detail: fmt.Sprintf("escalation rate %.0f%% — consider stronger local verifiers or retrieve_if thresholds", st.EscalationRate*100),
			})
		}
		if st.EarlyStopRate > 0.2 {
			out = append(out, Suggestion{
				Kind:   "early_stop",
				Detail: fmt.Sprintf("early_stop used in %.0f%% of runs — good for cost; capture skipped patterns as cached transforms if stable", st.EarlyStopRate*100),
			})
		}
		if st.AvgSkipped > 2 {
			out = append(out, Suggestion{
				Kind:   "conditional_skip",
				Detail: fmt.Sprintf("avg %.1f skipped nodes/run — review retrieve_if / branch_not_selected; promote stable paths to workflow fragments", st.AvgSkipped),
			})
		}
	}
	runPath := filepath.Join(dir, "run.json")
	b, err := os.ReadFile(runPath)
	if err == nil {
		var doc struct {
			Phase1 struct {
				Nodes []struct {
					ID         string `json:"id"`
					SkipReason string `json:"skip_reason,omitempty"`
					Skipped    bool   `json:"skipped,omitempty"`
				} `json:"nodes"`
			} `json:"phase1"`
		}
		if json.Unmarshal(b, &doc) == nil {
			reasons := make(map[string]int)
			for _, n := range doc.Phase1.Nodes {
				if n.Skipped && n.SkipReason != "" {
					reasons[n.SkipReason]++
				}
			}
			for r, c := range reasons {
				if c >= 2 {
					out = append(out, Suggestion{
						Kind:   "repeated_skip",
						Detail: fmt.Sprintf("skip_reason %q appeared %d times in last run — candidate for asset or resolver template", r, c),
					})
				}
			}
		}
	}
	if len(out) == 0 {
		out = append(out, Suggestion{Kind: "none", Detail: "no strong promotion signals yet — accumulate more metrics.jsonl lines"})
	}
	return out, nil
}

// FormatSuggestions renders suggestions as text.
func FormatSuggestions(s []Suggestion) string {
	var b strings.Builder
	for _, x := range s {
		b.WriteString("- [")
		b.WriteString(x.Kind)
		b.WriteString("] ")
		b.WriteString(x.Detail)
		b.WriteByte('\n')
	}
	return b.String()
}
