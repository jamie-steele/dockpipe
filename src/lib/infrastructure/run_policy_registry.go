package infrastructure

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const RunPolicyRecordKind = "dockpipe-run-policy"

type RunPolicyRecord struct {
	Schema                int      `json:"schema"`
	Kind                  string   `json:"kind"`
	ID                    string   `json:"id"`
	CreatedAt             string   `json:"created_at"`
	Workdir               string   `json:"workdir,omitempty"`
	WorkflowName          string   `json:"workflow_name,omitempty"`
	WorkflowConfig        string   `json:"workflow_config,omitempty"`
	StepID                string   `json:"step_id,omitempty"`
	ImageRef              string   `json:"image_ref,omitempty"`
	ImageArtifactDecision string   `json:"image_artifact_decision,omitempty"`
	PolicyFingerprint     string   `json:"policy_fingerprint,omitempty"`
	PolicySummary         string   `json:"policy_summary,omitempty"`
	NetworkMode           string   `json:"network_mode,omitempty"`
	NetworkEnforcement    string   `json:"network_enforcement,omitempty"`
	AppliedRuleIDs        []string `json:"applied_rule_ids,omitempty"`
	AdvisoryNotes         []string `json:"advisory_notes,omitempty"`
	EnforcementNotes      []string `json:"enforcement_notes,omitempty"`
	BlockReasons          []string `json:"block_reasons,omitempty"`
}

type RunPolicyListOptions struct {
	WorkflowName string
	StepID       string
	JSON         bool
}

func RunPolicyRecordsDir(workdir string) string {
	return filepath.Join(HostRunsDir(workdir), "policy")
}

func legacyRunDecisionRecordsDir(workdir string) string {
	return filepath.Join(HostRunsDir(workdir), "decisions")
}

func WriteRunPolicyRecord(workdir string, rec *RunPolicyRecord) (string, error) {
	wd := strings.TrimSpace(workdir)
	if wd == "" || rec == nil {
		return "", nil
	}
	wd, err := absHostWorkdir(wd)
	if err != nil {
		return "", err
	}
	dir := RunPolicyRecordsDir(wd)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	if strings.TrimSpace(rec.ID) == "" {
		rec.ID, err = randomRunID()
		if err != nil {
			return "", err
		}
	}
	if rec.Schema <= 0 {
		rec.Schema = 1
	}
	if strings.TrimSpace(rec.Kind) == "" {
		rec.Kind = RunPolicyRecordKind
	}
	if strings.TrimSpace(rec.CreatedAt) == "" {
		rec.CreatedAt = time.Now().UTC().Format(time.RFC3339)
	}
	rec.Workdir = wd
	b, err := json.MarshalIndent(rec, "", "  ")
	if err != nil {
		return "", err
	}
	b = append(b, '\n')
	path := filepath.Join(dir, rec.ID+".json")
	if err := os.WriteFile(path, b, 0o644); err != nil {
		return "", err
	}
	return path, nil
}

func ListRunPolicyRecords(workdir string, w io.Writer, opts RunPolicyListOptions) error {
	wd := strings.TrimSpace(workdir)
	if wd == "" {
		return fmt.Errorf("workdir is empty")
	}
	wd, err := absHostWorkdir(wd)
	if err != nil {
		return err
	}
	rows, found, err := readRunPolicyRecords(wd, opts)
	if err != nil {
		return err
	}
	if len(rows) == 0 {
		if found {
			fmt.Fprintf(w, "No run policy records match in %s\n", RunPolicyRecordsDir(wd))
		} else {
			fmt.Fprintf(w, "No run policy records in %s\n", RunPolicyRecordsDir(wd))
		}
		return nil
	}
	if opts.JSON {
		b, err := json.MarshalIndent(rows, "", "  ")
		if err != nil {
			return err
		}
		b = append(b, '\n')
		_, err = w.Write(b)
		return err
	}
	fmt.Fprintf(w, "Run policy records (workdir=%s)\n", wd)
	fmt.Fprintf(w, "%-10s %-18s %-18s %-12s %-10s %s\n", "ID", "Workflow", "Step", "Network", "Enforce", "Image")
	for _, r := range rows {
		wf := truncateRunPolicyField(r.WorkflowName, 18)
		step := truncateRunPolicyField(r.StepID, 18)
		net := truncateRunPolicyField(r.NetworkMode, 12)
		enf := truncateRunPolicyField(r.NetworkEnforcement, 10)
		img := truncateRunPolicyField(r.ImageRef, 40)
		fmt.Fprintf(w, "%-10s %-18s %-18s %-12s %-10s %s\n",
			truncateRunPolicyField(r.ID, 10), wf, step, net, enf, img)
	}
	return nil
}

func readRunPolicyRecords(workdir string, opts RunPolicyListOptions) ([]RunPolicyRecord, bool, error) {
	dirs := []string{RunPolicyRecordsDir(workdir), legacyRunDecisionRecordsDir(workdir)}
	rows := make([]RunPolicyRecord, 0)
	foundAny := false
	seen := map[string]struct{}{}
	for _, dir := range dirs {
		ents, err := os.ReadDir(dir)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, false, err
		}
		foundAny = true
		for _, rec := range loadRunPolicyRecords(dir, ents) {
			if _, ok := seen[rec.ID]; ok {
				continue
			}
			seen[rec.ID] = struct{}{}
			if !matchRunPolicyRecord(rec, opts) {
				continue
			}
			rows = append(rows, rec)
		}
	}
	sort.Slice(rows, func(i, j int) bool {
		return rows[i].CreatedAt > rows[j].CreatedAt
	})
	return rows, foundAny, nil
}

func loadRunPolicyRecords(dir string, ents []os.DirEntry) []RunPolicyRecord {
	var rows []RunPolicyRecord
	for _, e := range ents {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		b, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		var rec RunPolicyRecord
		if json.Unmarshal(b, &rec) != nil {
			continue
		}
		rows = append(rows, rec)
	}
	return rows
}

func matchRunPolicyRecord(rec RunPolicyRecord, opts RunPolicyListOptions) bool {
	if wf := strings.TrimSpace(opts.WorkflowName); wf != "" && strings.TrimSpace(rec.WorkflowName) != wf {
		return false
	}
	if step := strings.TrimSpace(opts.StepID); step != "" && strings.TrimSpace(rec.StepID) != step {
		return false
	}
	return true
}

func truncateRunPolicyField(v string, n int) string {
	v = strings.TrimSpace(v)
	if len(v) <= n {
		return v
	}
	if n <= 3 {
		return v[:n]
	}
	return v[:n-3] + "..."
}
