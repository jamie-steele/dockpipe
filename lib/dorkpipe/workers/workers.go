// Package workers executes DAG nodes using real processes and network I/O (no stubs).
package workers

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/lib/pq"

	"dockpipe/lib/dorkpipe/confidence"
	"dockpipe/lib/dorkpipe/spec"
)

// Result is one node's outcome.
type Result struct {
	NodeID   string
	Kind     string
	ExitCode int
	Stdout   string
	Stderr   string
	Vector   confidence.Vector
	Err      error
	// Skipped: node was not executed (conditional branch, early stop, etc.); excluded from harmonic aggregate.
	Skipped    bool
	SkipReason string
}

// Executor runs nodes; DockpipeBin defaults to PATH lookup.
type Executor struct {
	Workdir     string
	Env         []string
	DockpipeBin string
	HTTPClient  *http.Client
}

func (e *Executor) dockpipePath() string {
	if strings.TrimSpace(e.DockpipeBin) != "" {
		return e.DockpipeBin
	}
	p, err := exec.LookPath("dockpipe")
	if err != nil {
		return "dockpipe"
	}
	return p
}

// Run executes a single node. subst replaces {{key}} in prompts from prior outputs.
func (e *Executor) Run(ctx context.Context, n *spec.Node, subst map[string]string) *Result {
	r := &Result{NodeID: n.ID, Kind: n.Kind}
	k := strings.ToLower(strings.TrimSpace(n.Kind))
	if k == "deterministic" {
		k = "shell"
	}
	switch k {
	case "shell":
		r = e.runShell(ctx, n, subst)
	case "dockpipe":
		r = e.runDockpipe(ctx, n, subst, false)
	case "ollama":
		r = e.runOllama(ctx, n, subst)
	case "verifier":
		r = e.runVerifier(ctx, n, subst)
	case "pgvector":
		r = e.runPGVector(ctx, n)
	case "codex":
		r = e.runDockpipe(ctx, n, subst, true)
	default:
		r.Err = fmt.Errorf("workers: unknown kind %q", n.Kind)
	}
	return r
}

func (e *Executor) workdir(n *spec.Node) string {
	w := strings.TrimSpace(n.Workdir)
	if w != "" {
		return w
	}
	if strings.TrimSpace(e.Workdir) != "" {
		return e.Workdir
	}
	wd, err := os.Getwd()
	if err != nil {
		return "."
	}
	return wd
}

func applySubst(s string, subst map[string]string) string {
	out := s
	for k, v := range subst {
		out = strings.ReplaceAll(out, "{{"+k+"}}", v)
	}
	return out
}

func (e *Executor) runShell(ctx context.Context, n *spec.Node, subst map[string]string) *Result {
	r := &Result{NodeID: n.ID, Kind: "shell", Vector: confidence.Vector{
		NodeSelf:    confidence.F64(1),
		ToolSuccess: confidence.F64(1),
	}}
	script := applySubst(n.Script, subst)
	cmd := exec.CommandContext(ctx, "bash", "-c", script)
	wd := e.workdir(n)
	cmd.Dir = wd
	cmd.Env = append(append(os.Environ(), e.Env...), "DOCKPIPE_WORKDIR="+wd)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	r.Stdout = stdout.String()
	r.Stderr = stderr.String()
	if cmd.ProcessState != nil {
		r.ExitCode = cmd.ProcessState.ExitCode()
	}
	if err != nil {
		r.Err = err
		return r
	}
	return r
}

func (e *Executor) runDockpipe(ctx context.Context, n *spec.Node, subst map[string]string, isCodex bool) *Result {
	kind := "dockpipe"
	if isCodex {
		kind = "codex"
	}
	r := &Result{NodeID: n.ID, Kind: kind, Vector: confidence.Vector{
		NodeSelf:    confidence.F64(1),
		ToolSuccess: confidence.F64(1),
	}}
	args := append([]string{}, n.DockpipeArgs...)
	for i := range args {
		args[i] = applySubst(args[i], subst)
	}
	cmd := exec.CommandContext(ctx, e.dockpipePath(), args...)
	cmd.Dir = e.workdir(n)
	cmd.Env = append(os.Environ(), e.Env...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	r.Stdout = stdout.String()
	r.Stderr = stderr.String()
	if cmd.ProcessState != nil {
		r.ExitCode = cmd.ProcessState.ExitCode()
	}
	if err != nil {
		r.Err = err
		return r
	}
	return r
}

func (e *Executor) http() *http.Client {
	if e.HTTPClient != nil {
		return e.HTTPClient
	}
	return &http.Client{Timeout: 15 * time.Minute}
}

func (e *Executor) runOllama(ctx context.Context, n *spec.Node, subst map[string]string) *Result {
	r := &Result{NodeID: n.ID, Kind: "ollama"}
	host := strings.TrimSpace(n.OllamaHost)
	if host == "" {
		host = os.Getenv("OLLAMA_HOST")
	}
	if host == "" {
		host = "http://127.0.0.1:11434"
	}
	host = strings.TrimSuffix(host, "/")
	prompt := applySubst(n.Prompt, subst)
	body := map[string]any{
		"model":  n.Model,
		"prompt": prompt,
		"stream": false,
	}
	b, err := json.Marshal(body)
	if err != nil {
		r.Err = err
		return r
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, host+"/api/generate", bytes.NewReader(b))
	if err != nil {
		r.Err = err
		return r
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := e.http().Do(req)
	if err != nil {
		r.Err = err
		return r
	}
	defer resp.Body.Close()
	out, _ := io.ReadAll(resp.Body)
	r.Stdout = string(out)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		r.Err = fmt.Errorf("ollama: HTTP %d: %s", resp.StatusCode, r.Stdout)
		return r
	}
	var parsed struct {
		Response string `json:"response"`
	}
	if err := json.Unmarshal(out, &parsed); err == nil && strings.TrimSpace(parsed.Response) != "" {
		r.Stdout = parsed.Response
	}
	ns := ollamaNodeSelf(r.Stdout)
	r.Vector = confidence.Vector{
		NodeSelf:    confidence.F64(ns),
		ToolSuccess: confidence.F64(1),
	}
	return r
}

func (e *Executor) runVerifier(ctx context.Context, n *spec.Node, subst map[string]string) *Result {
	r := e.runOllama(ctx, n, subst)
	if r.Err != nil {
		r.Kind = "verifier"
		return r
	}
	r.Kind = "verifier"
	r.Vector = verifierVectorFromStdout(r.Stdout)
	return r
}

func verifierVectorFromStdout(s string) confidence.Vector {
	text := strings.TrimSpace(s)
	var outer struct {
		Response string `json:"response"`
	}
	if json.Unmarshal([]byte(text), &outer) == nil && strings.TrimSpace(outer.Response) != "" {
		text = strings.TrimSpace(outer.Response)
	}
	sc := parseVerifierScore(text)
	return confidence.Vector{
		NodeSelf:    confidence.F64(0.55),
		Verifier:    confidence.F64(sc),
		ToolSuccess: confidence.F64(1),
	}
}

func parseVerifierScore(s string) float64 {
	var m struct {
		Verifier      float64 `json:"verifier"`
		VerifierScore float64 `json:"verifier_score"`
		Score         float64 `json:"score"`
		Pass          *bool   `json:"pass"`
		NeedsMoreCtx  *bool   `json:"needs_more_context"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(s)), &m); err != nil {
		return 0.5
	}
	if m.Verifier > 0 {
		return confidence.Clamp01(m.Verifier)
	}
	if m.VerifierScore > 0 {
		return confidence.Clamp01(m.VerifierScore)
	}
	if m.Score > 0 {
		return confidence.Clamp01(m.Score)
	}
	if m.Pass != nil {
		if *m.Pass {
			return 0.85
		}
		return 0.25
	}
	if m.NeedsMoreCtx != nil && *m.NeedsMoreCtx {
		return 0.4
	}
	return 0.5
}

// ollamaNodeSelf is a heuristic self-score for local LLM output length/shape (not calibrated research-grade).
func ollamaNodeSelf(s string) float64 {
	t := strings.TrimSpace(s)
	if len(t) < 20 {
		return 0.4
	}
	if len(t) > 400 {
		return 0.85
	}
	return 0.65
}

func (e *Executor) runPGVector(ctx context.Context, n *spec.Node) *Result {
	r := &Result{NodeID: n.ID, Kind: "pgvector"}
	dsn := strings.TrimSpace(n.DatabaseURL)
	if dsn == "" && strings.TrimSpace(n.DatabaseURLEnv) != "" {
		dsn = os.Getenv(n.DatabaseURLEnv)
	}
	if dsn == "" {
		r.Err = fmt.Errorf("pgvector: empty database DSN")
		return r
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		r.Err = err
		return r
	}
	defer db.Close()
	if err := db.PingContext(ctx); err != nil {
		r.Err = fmt.Errorf("pgvector ping: %w", err)
		return r
	}
	rows, err := db.QueryContext(ctx, n.SQL)
	if err != nil {
		r.Err = err
		return r
	}
	defer rows.Close()
	cols, err := rows.Columns()
	if err != nil {
		r.Err = err
		return r
	}
	var buf strings.Builder
	buf.WriteString(strings.Join(cols, "\t"))
	buf.WriteByte('\n')
	for rows.Next() {
		raw := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range raw {
			ptrs[i] = &raw[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			r.Err = err
			return r
		}
		for i, v := range raw {
			if i > 0 {
				buf.WriteByte('\t')
			}
			buf.WriteString(fmt.Sprint(v))
		}
		buf.WriteByte('\n')
	}
	if err := rows.Err(); err != nil {
		r.Err = err
		return r
	}
	r.Stdout = buf.String()
	rs := 0.35
	if strings.TrimSpace(r.Stdout) != "" {
		rs = 1.0
	}
	r.Vector = confidence.Vector{
		RetrievalSupport: confidence.F64(rs),
		ToolSuccess:      confidence.F64(1),
	}
	return r
}

// WriteOutputs writes node stdout to .dorkpipe/nodes/<id>.txt under workdir (for downstream subst).
func WriteOutputs(workdir, nodeID, stdout string) error {
	dir := filepath.Join(workdir, ".dorkpipe", "nodes")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, nodeID+".txt"), []byte(stdout), 0o644)
}
