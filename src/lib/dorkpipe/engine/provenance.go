package engine

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"dockpipe/src/lib/dorkpipe/confidence"
	"dockpipe/src/lib/dorkpipe/spec"
	"dockpipe/src/lib/dorkpipe/workers"
)

type runDoc struct {
	Name         string       `json:"name"`
	TS           string       `json:"ts"`
	Policy       spec.Policy  `json:"policy"`
	BranchWinner string       `json:"branch_winner,omitempty"`
	EarlyStop    bool         `json:"early_stop,omitempty"`
	Phase1       phaseSection `json:"phase1"`
	Escal        *escSection  `json:"escalation,omitempty"`
}

type phaseSection struct {
	Nodes     []nodeOut         `json:"nodes"`
	Aggregate confidence.Vector `json:"aggregate"`
}

type escSection struct {
	Ran   bool      `json:"ran"`
	Nodes []nodeOut `json:"nodes,omitempty"`
}

type nodeOut struct {
	ID         string            `json:"id"`
	Kind       string            `json:"kind"`
	ExitCode   int               `json:"exit_code"`
	Vector     confidence.Vector `json:"vector"`
	StdoutLen  int               `json:"stdout_len"`
	Err        string            `json:"error,omitempty"`
	Skipped    bool              `json:"skipped,omitempty"`
	SkipReason string            `json:"skip_reason,omitempty"`
}

func writeProvenance(workdir string, d *spec.Doc, phase1, esc []*workers.Result, sum confidence.Vector, escalRan bool, subst map[string]string, earlyStop bool) error {
	dir := filepath.Join(workdir, ".dorkpipe")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	bw := ""
	if subst != nil {
		bw = subst["__branch_winner__"]
	}
	doc := runDoc{
		Name:         d.Name,
		TS:           time.Now().UTC().Format(time.RFC3339Nano),
		Policy:       d.Policy,
		BranchWinner: bw,
		EarlyStop:    earlyStop,
		Phase1: phaseSection{
			Nodes:     toNodes(phase1),
			Aggregate: sum,
		},
	}
	if escalRan {
		doc.Escal = &escSection{Ran: true, Nodes: toNodes(esc)}
	}
	b, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "run.json"), b, 0o644)
}

func toNodes(rs []*workers.Result) []nodeOut {
	var out []nodeOut
	for _, r := range rs {
		if r == nil {
			continue
		}
		no := nodeOut{
			ID:        r.NodeID,
			Kind:      r.Kind,
			ExitCode:  r.ExitCode,
			Vector:    r.Vector,
			StdoutLen: len(r.Stdout),
		}
		if r.Err != nil {
			no.Err = r.Err.Error()
		}
		if r.Skipped {
			no.Skipped = true
			no.SkipReason = r.SkipReason
		}
		out = append(out, no)
	}
	return out
}

func appendMetricsJSONL(workdir string, name string, calibrated float64, escalated bool, earlyStop bool, results []*workers.Result) error {
	dir := filepath.Join(workdir, ".dorkpipe")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	skipped := 0
	for _, r := range results {
		if r != nil && r.Skipped {
			skipped++
		}
	}
	line := map[string]any{
		"ts":             time.Now().UTC().Format(time.RFC3339Nano),
		"name":           name,
		"calibrated":     calibrated,
		"escalated":      escalated,
		"early_stop":     earlyStop,
		"skipped_nodes":  skipped,
		"schema":         "dorkpipe.metrics.v2",
		"schema_version": 2,
	}
	b, err := json.Marshal(line)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(filepath.Join(dir, "metrics.jsonl"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(append(b, '\n'))
	return err
}
