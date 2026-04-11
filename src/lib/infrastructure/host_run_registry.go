package infrastructure

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const hostRunIDBytes = 4 // 8 hex chars

// HostRunRecord is written to bin/.dockpipe/runs/<id>.json while a host script (skip_container step) runs.
type HostRunRecord struct {
	ID        string `json:"id"`
	PID       int    `json:"pid"`
	StartedAt string `json:"startedAt"`
	Workdir   string `json:"workdir"`
	Script    string `json:"script"`
	Container string `json:"container,omitempty"`
}

// HostRunsDir returns the per-project runs directory.
func HostRunsDir(workdir string) string {
	return filepath.Join(workdir, DockpipeDirRel, "runs")
}

// BeginHostRun adds DOCKPIPE_RUN_ID and DOCKPIPE_RUN_FILE to env and returns the JSON path to fill after Start.
// If workdir is empty, returns ("", "", env, nil) and does nothing.
func BeginHostRun(workdir string, env []string) (runID, runFile string, outEnv []string, err error) {
	wd := strings.TrimSpace(workdir)
	if wd == "" {
		return "", "", env, nil
	}
	wd, err = absHostWorkdir(wd)
	if err != nil {
		return "", "", env, err
	}
	rid, err := randomRunID()
	if err != nil {
		return "", "", env, err
	}
	dir := HostRunsDir(wd)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", "", env, err
	}
	rf := filepath.Join(dir, rid+".json")
	out := append([]string(nil), env...)
	out = append(out, "DOCKPIPE_RUN_ID="+rid, "DOCKPIPE_RUN_FILE="+rf)
	return rid, rf, out, nil
}

// WriteHostRunRecord writes the registry JSON after the bash child has started (PID known).
func WriteHostRunRecord(runFile, runID string, pid int, workdir, scriptPath string) error {
	if runFile == "" {
		return nil
	}
	wd := strings.TrimSpace(workdir)
	if wd != "" {
		var err error
		wd, err = absHostWorkdir(wd)
		if err != nil {
			return err
		}
	}
	rec := HostRunRecord{
		ID:        runID,
		PID:       pid,
		StartedAt: time.Now().UTC().Format(time.RFC3339),
		Workdir:   wd,
		Script:    filepath.Base(scriptPath),
	}
	b, err := json.MarshalIndent(rec, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	return os.WriteFile(runFile, b, 0o644)
}

// RemoveHostRunArtifacts removes the run JSON and optional .container sidecar for the same id.
func RemoveHostRunArtifacts(runFile string) {
	if runFile == "" {
		return
	}
	_ = os.Remove(runFile)
	dir := filepath.Dir(runFile)
	base := filepath.Base(runFile)
	if !strings.HasSuffix(base, ".json") {
		return
	}
	id := strings.TrimSuffix(base, ".json")
	if id == "" || strings.Contains(id, string(filepath.Separator)) || strings.Contains(id, "..") {
		return
	}
	_ = os.Remove(filepath.Join(dir, id+".container"))
}

// ReadHostRunContainerSidecar returns the container name from runs/<id>.container if present.
func ReadHostRunContainerSidecar(runFile string) string {
	if runFile == "" {
		return ""
	}
	dir := filepath.Dir(runFile)
	base := filepath.Base(runFile)
	if !strings.HasSuffix(base, ".json") {
		return ""
	}
	id := strings.TrimSuffix(base, ".json")
	if id == "" {
		return ""
	}
	b, err := os.ReadFile(filepath.Join(dir, id+".container"))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

// ListHostRuns prints a human-readable table of runs under workdir/bin/.dockpipe/runs/*.json
func ListHostRuns(workdir string, w io.Writer) error {
	wd := strings.TrimSpace(workdir)
	if wd == "" {
		return fmt.Errorf("workdir is empty")
	}
	wd, err := absHostWorkdir(wd)
	if err != nil {
		return err
	}
	dir := HostRunsDir(wd)
	ents, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Fprintf(w, "No host runs under %s\n", dir)
			return nil
		}
		return err
	}
	var rows []HostRunRecord
	for _, e := range ents {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		p := filepath.Join(dir, e.Name())
		b, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		var rec HostRunRecord
		if json.Unmarshal(b, &rec) != nil {
			continue
		}
		if c := ReadHostRunContainerSidecar(p); c != "" && rec.Container == "" {
			rec.Container = c
		}
		rows = append(rows, rec)
	}
	if len(rows) == 0 {
		fmt.Fprintf(w, "No host run records in %s\n", dir)
		return nil
	}
	fmt.Fprintf(w, "Host runs (workdir=%s)\n", wd)
	fmt.Fprintf(w, "%-10s %8s %-32s %-20s %s\n", "ID", "PID", "Started", "Script", "Container")
	for _, r := range rows {
		st := r.StartedAt
		if len(st) > 32 {
			st = st[:29] + "..."
		}
		sc := r.Script
		if len(sc) > 20 {
			sc = sc[:17] + "..."
		}
		co := r.Container
		if len(co) > 40 {
			co = co[:37] + "..."
		}
		fmt.Fprintf(w, "%-10s %8d %-32s %-20s %s\n", r.ID, r.PID, st, sc, co)
	}
	return nil
}

func absHostWorkdir(wd string) (string, error) {
	if filepath.IsAbs(wd) {
		return filepath.Clean(wd), nil
	}
	cwd, err := os.Getwd()
	if err != nil {
		return filepath.Clean(wd), nil
	}
	return filepath.Clean(filepath.Join(cwd, wd)), nil
}

func randomRunID() (string, error) {
	b := make([]byte, hostRunIDBytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
