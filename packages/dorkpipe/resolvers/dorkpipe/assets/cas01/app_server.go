// CAS-01 is a disposable research harness, not a production App Server adapter.
package main

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

const (
	cas01Model    = "gpt-5.5"
	cas01Provider = "openai"
	cas01Effort   = "high"
)

type Event struct {
	Kind      string
	ThreadID  string
	TurnID    string
	ItemID    string
	RequestID string
	Status    string
	ErrorKind string
	WillRetry bool
}
type Policy struct {
	Sandbox  string
	Reviewer string
	Shell    bool
}
type Expect struct {
	Status            string
	Denied            int
	Rejected          int
	Terminal          string
	ErrorKinds        map[string]int
	RetriableErrors   int
	Warnings          int
	TerminalErrorKind string
}
type Fixture struct {
	Name   string
	Policy Policy
	Events []Event
	Expect Expect
}
type pending struct {
	thread string
	turn   string
	item   string
	denied bool
}
type state struct {
	status     string
	terminal   string
	thread     string
	turn       string
	items      map[string]bool
	pending    map[string]pending
	seen       map[string]bool
	denied     int
	rejected   int
	diagnostic diagnostic
}
type Audit struct {
	Sequence  int    `json:"sequence"`
	Direction string `json:"direction"`
	Method    string `json:"method"`
	ID        string `json:"id,omitempty"`
	Terminal  string `json:"terminal,omitempty"`
}
type Evidence struct {
	Harness                   string           `json:"harness"`
	Protocol                  string           `json:"protocol"`
	Outcome                   string           `json:"outcome"`
	StartedAt                 string           `json:"startedAt"`
	CompletedAt               string           `json:"completedAt"`
	Durations                 map[string]int64 `json:"durationsMs"`
	Events                    []Audit          `json:"events"`
	Redaction                 string           `json:"redaction"`
	Blocker                   string           `json:"blocker,omitempty"`
	MaterializationTerminal   string           `json:"materializationTerminal,omitempty"`
	MaterializationErrorKind  string           `json:"materializationErrorKind,omitempty"`
	ResumeTerminal            string           `json:"resumeTerminal,omitempty"`
	MaterializationDiagnostic *diagnostic      `json:"materializationDiagnostic,omitempty"`
}
type terminalState struct {
	classification string
	diagnostic     diagnostic
}
type diagnostic struct {
	ErrorKinds        map[string]int `json:"errorKinds,omitempty"`
	RetriableErrors   int            `json:"retriableErrors,omitempty"`
	Warnings          int            `json:"warnings,omitempty"`
	TerminalErrorKind string         `json:"terminalErrorKind,omitempty"`
}
type terminalError struct{ kind string }

func (e terminalError) Error() string { return e.kind }

type Client struct {
	stdin  io.WriteCloser
	scan   *bufio.Scanner
	next   int
	events *[]Audit
}

func main() {
	mode := flag.String("mode", "fixtures", "fixtures, diagnostics, policy, or live")
	fixtures := flag.String("fixtures", "", "absolute fixture file")
	sandbox := flag.String("sandbox", "workspace-write", "sandbox")
	reviewer := flag.String("reviewer", "user", "reviewer")
	method := flag.String("method", "", "method")
	artifacts := flag.String("artifacts", "", "absolute artifact directory")
	codex := flag.String("codex", "", "absolute Codex CLI path")
	workspace := flag.String("workspace", "", "absolute workspace")
	model := flag.String("model", cas01Model, "model")
	effort := flag.String("reasoning-effort", cas01Effort, "reasoning effort")
	turn := flag.Bool("start-turn", false, "start a cloud-backed turn")
	ack := flag.Bool("ack-cloud-spend", false, "acknowledge cloud-backed work")
	flag.Parse()
	var err error
	switch *mode {
	case "fixtures":
		err = validateFixtures(*fixtures)
	case "diagnostics":
		err = validateDiagnostics()
	case "policy":
		err = policy(*sandbox, *reviewer, *method, *model, *effort)
	case "live":
		err = live(*artifacts, *codex, *workspace, *turn, *ack)
	default:
		err = fmt.Errorf("unsupported mode %q", *mode)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "cas01:", err)
		os.Exit(1)
	}
}
func policy(sandbox, reviewer, method, model, effort string) error {
	if sandbox != "workspace-write" {
		return errors.New("only workspace-write is permitted")
	}
	if reviewer != "user" {
		return errors.New("only a human user reviewer is permitted")
	}
	if method == "thread/shellCommand" {
		return errors.New("thread shell command is forbidden")
	}
	if model != cas01Model || effort != cas01Effort {
		return errors.New("only gpt-5.5 with high reasoning is permitted")
	}
	if err := verifyModelCatalog(map[string]any{"data": []any{map[string]any{"id": cas01Model, "supportedReasoningEfforts": []any{map[string]any{"reasoningEffort": cas01Effort}}}}}); err != nil {
		return err
	}
	if err := validatePinnedRequests(); err != nil {
		return err
	}
	return nil
}
func validateFixtures(path string) error {
	if !filepath.IsAbs(path) {
		return errors.New("fixtures path must be absolute")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var fixtures []Fixture
	if err = json.Unmarshal(data, &fixtures); err != nil {
		return err
	}
	if len(fixtures) == 0 {
		return errors.New("no fixtures")
	}
	for _, f := range fixtures {
		if err = validate(f); err != nil {
			return fmt.Errorf("%s: %w", f.Name, err)
		}
	}
	fmt.Printf("cas01 fixtures OK (%d)\n", len(fixtures))
	return nil
}
func validateDiagnostics() error {
	notification := map[string]any{
		"threadId":  "thread-A",
		"turnId":    "turn-A",
		"willRetry": true,
		"error": map[string]any{
			"codexErrorInfo": map[string]any{"responseTooManyFailedAttempts": map[string]any{}},
		},
	}
	if !correlatedError(notification, "thread-A", "turn-A") {
		return errors.New("error notification correlation failed")
	}
	d := diagnostic{}
	recordError(&d, notificationErrorKind(notification), asBool(notification["willRetry"]))
	if !sameKinds(d.ErrorKinds, map[string]int{"responseTooManyFailedAttempts": 1}) || d.RetriableErrors != 1 {
		return errors.New("error notification diagnostic classification failed")
	}
	terminal := map[string]any{"turn": map[string]any{"error": map[string]any{"codexErrorInfo": "sandboxError"}}}
	if turnErrorKind(terminal) != "sandboxError" || safeCodexErrorKind(map[string]any{"unrecognized": map[string]any{}}) != "unknown" {
		return errors.New("terminal diagnostic classification failed")
	}
	fmt.Println("cas01 diagnostics OK")
	return nil
}
func validate(f Fixture) error {
	if f.Name == "" || len(f.Events) == 0 || f.Policy.Shell {
		return errors.New("fixture name, events, and no-shell policy are required")
	}
	if err := policy(f.Policy.Sandbox, f.Policy.Reviewer, "", cas01Model, cas01Effort); err != nil {
		return err
	}
	s := state{status: "Ready", items: map[string]bool{}, pending: map[string]pending{}, seen: map[string]bool{}, diagnostic: diagnostic{ErrorKinds: map[string]int{}}}
	for _, e := range f.Events {
		if e.Kind == "" {
			return errors.New("event kind is required")
		}
		key := e.Kind + ":" + e.ThreadID + ":" + e.TurnID + ":" + e.ItemID + ":" + e.RequestID
		if s.seen[key] && e.Kind != "approval_replay" && e.Kind != "error" && e.Kind != "warning" {
			closeState(&s)
			continue
		}
		s.seen[key] = true
		switch e.Kind {
		case "thread_started":
			if s.thread != "" || e.ThreadID == "" {
				return errors.New("invalid thread start")
			}
			s.thread = e.ThreadID
		case "turn_started":
			if e.ThreadID != s.thread || s.turn != "" || e.TurnID == "" {
				return errors.New("invalid turn start")
			}
			s.turn = e.TurnID
			s.status = "Running"
		case "item_started":
			if s.status != "Running" || e.ThreadID != s.thread || e.TurnID != s.turn || e.ItemID == "" {
				return errors.New("unordered item start")
			}
			s.items[e.ItemID] = true
		case "item_completed":
			if !s.items[e.ItemID] {
				return errors.New("item completed before start")
			}
		case "turn_completed":
			if s.status != "Running" || e.ThreadID != s.thread || e.TurnID != s.turn {
				return errors.New("unordered terminal turn")
			}
			terminal, ok := safeTurnTerminal(e.Status)
			if !ok {
				return errors.New("unknown terminal state")
			}
			s.terminal = terminal
			if e.Status == "failed" {
				s.diagnostic.TerminalErrorKind = safeCodexErrorKind(e.ErrorKind)
			}
			s.turn = ""
			if e.Status == "failed" {
				s.status = "Failed"
			} else {
				s.status = "Idle"
			}
		case "approval_requested":
			if s.status != "Running" || e.ThreadID != s.thread || e.TurnID != s.turn || e.ItemID == "" || e.RequestID == "" {
				return errors.New("invalid approval tuple")
			}
			s.pending[e.RequestID] = pending{e.ThreadID, e.TurnID, e.ItemID, false}
		case "error":
			if s.status != "Running" || e.ThreadID != s.thread || e.TurnID != s.turn {
				return errors.New("uncorrelated error notification")
			}
			recordError(&s.diagnostic, safeCodexErrorKind(e.ErrorKind), e.WillRetry)
		case "warning":
			if e.ThreadID != s.thread {
				return errors.New("uncorrelated warning notification")
			}
			s.diagnostic.Warnings++
		case "approval_denied":
			p, ok := s.pending[e.RequestID]
			if !ok || p.denied || p.thread != e.ThreadID || p.turn != e.TurnID || p.item != e.ItemID {
				return errors.New("denial tuple mismatch")
			}
			p.denied = true
			s.pending[e.RequestID] = p
			s.denied++
		case "approval_replay", "approval_cross_session":
			s.rejected++
		case "malformed", "duplicate", "reordered", "stale", "child_died", "shutdown", "transport_lost":
			closeState(&s)
		default:
			return fmt.Errorf("unsupported event %q", e.Kind)
		}
	}
	if s.status != f.Expect.Status || s.denied != f.Expect.Denied || s.rejected != f.Expect.Rejected || s.terminal != f.Expect.Terminal || !sameKinds(s.diagnostic.ErrorKinds, f.Expect.ErrorKinds) || s.diagnostic.RetriableErrors != f.Expect.RetriableErrors || s.diagnostic.Warnings != f.Expect.Warnings || s.diagnostic.TerminalErrorKind != f.Expect.TerminalErrorKind {
		return fmt.Errorf("got status=%s denied=%d rejected=%d terminal=%s diagnostics=%+v", s.status, s.denied, s.rejected, s.terminal, s.diagnostic)
	}
	return nil
}
func closeState(s *state) {
	for id, p := range s.pending {
		if !p.denied {
			p.denied = true
			s.pending[id] = p
			s.denied++
		}
	}
	if s.turn != "" {
		s.status = "Disconnected"
	}
}
func live(artifacts, codex, workspace string, turn, ack bool) error {
	if !filepath.IsAbs(artifacts) || !filepath.IsAbs(codex) || !filepath.IsAbs(workspace) {
		return errors.New("artifacts, codex, and workspace must be absolute")
	}
	if turn && !ack {
		return errors.New("--start-turn requires --ack-cloud-spend")
	}
	if err := os.MkdirAll(artifacts, 0755); err != nil {
		return err
	}
	e := Evidence{Harness: "cas-01", Protocol: "stdio", Outcome: "Blocked", StartedAt: time.Now().UTC().Format(time.RFC3339), Durations: map[string]int64{}, Redaction: "method, timing, SHA-256 request IDs, and allow-listed diagnostic classes/counts only; no raw RPC, prompt, command, diff, stderr, or credentials"}
	defer func() {
		e.CompletedAt = time.Now().UTC().Format(time.RFC3339)
		data, _ := json.MarshalIndent(e, "", "  ")
		_ = os.WriteFile(filepath.Join(artifacts, "cas01-live-evidence.json"), append(data, '\n'), 0600)
	}()
	cmd := exec.Command(codex, "app-server", "--stdio")
	in, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	out, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	cmd.Stderr = io.Discard
	if err = cmd.Start(); err != nil {
		return err
	}
	defer func() { _ = in.Close(); _ = cmd.Process.Kill(); _, _ = cmd.Process.Wait() }()
	scanner := bufio.NewScanner(out)
	scanner.Buffer(make([]byte, 4096), 2<<20)
	c := Client{stdin: in, scan: scanner, next: 1, events: &e.Events}
	started := time.Now()
	if _, err = c.call("initialize", map[string]any{"clientInfo": map[string]any{"name": "dockpipe-cas01", "version": "0.1"}, "capabilities": map[string]any{"experimentalApi": false}}); err != nil {
		e.Blocker = "initialize"
		return errors.New("initialize failed without retaining server payload")
	}
	e.Durations["cold_initialize"] = time.Since(started).Milliseconds()
	if err = c.send("initialized", map[string]any{}); err != nil {
		return err
	}
	models, err := c.call("model/list", map[string]any{})
	if err != nil {
		e.Blocker = "model_policy"
		return errors.New("model catalog failed without retaining server payload")
	}
	if err = verifyModelCatalog(models); err != nil {
		e.Blocker = "model_policy"
		return errors.New("gpt-5.5 high is unavailable; turn was not started")
	}
	started = time.Now()
	r, err := c.call("thread/start", threadStartParams(workspace))
	if err != nil {
		e.Blocker = "thread_start"
		return errors.New("thread start failed without retaining server payload")
	}
	threadID := nested(r, "thread", "id")
	if threadID == "" {
		return errors.New("thread start returned no ID")
	}
	e.Durations["thread_start"] = time.Since(started).Milliseconds()
	started = time.Now()
	if _, err = c.call("thread/read", map[string]any{"threadId": threadID, "includeTurns": false}); err != nil {
		e.Blocker = "thread_read"
		return errors.New("thread read failed without retaining server payload")
	}
	if !turn {
		e.Blocker = "materialization_turn"
		return errors.New("safe resume requires a completed materialization turn")
	}
	started = time.Now()
	r, err = c.call("turn/start", turnStartParams(threadID, workspace))
	if err != nil {
		e.Blocker = "turn_start"
		return errors.New("turn start failed without retaining server payload")
	}
	turnID := nested(r, "turn", "id")
	if turnID == "" {
		e.Blocker = "turn_start"
		return errors.New("turn start returned no ID")
	}
	terminal, err := c.waitTurn(threadID, turnID)
	e.MaterializationTerminal = terminal.classification
	e.MaterializationDiagnostic = &terminal.diagnostic
	if err != nil {
		e.Blocker = "turn_completion"
		e.MaterializationErrorKind = safeErrorKind(err)
		return errors.New("turn completion failed without retaining server payload")
	}
	e.Durations["materialization_turn"] = time.Since(started).Milliseconds()
	started = time.Now()
	if _, err = c.call("thread/resume", map[string]any{"threadId": threadID}); err != nil {
		e.Blocker = "thread_resume"
		e.ResumeTerminal = safeErrorKind(err)
		return errors.New("thread resume failed without retaining server payload")
	}
	e.ResumeTerminal = "result"
	e.Durations["read_resume"] = time.Since(started).Milliseconds()
	e.Outcome = "Ready"
	return nil
}
func threadStartParams(workspace string) map[string]any {
	return map[string]any{"cwd": workspace, "sandbox": "workspace-write", "approvalPolicy": "untrusted", "approvalsReviewer": "user", "model": cas01Model, "modelProvider": cas01Provider, "effort": cas01Effort}
}
func turnStartParams(threadID, workspace string) map[string]any {
	return map[string]any{"threadId": threadID, "cwd": workspace, "sandboxPolicy": map[string]any{"type": "workspaceWrite", "writableRoots": []string{workspace}, "networkAccess": false}, "approvalPolicy": "untrusted", "approvalsReviewer": "user", "model": cas01Model, "effort": cas01Effort, "input": []any{map[string]any{"type": "text", "text": "Reply only CAS01_OK. Do not use tools or change anything."}}}
}
func validatePinnedRequests() error {
	thread := threadStartParams("/workspace")
	turn := turnStartParams("thread", "/workspace")
	if asString(thread["model"]) != cas01Model || asString(thread["modelProvider"]) != cas01Provider || asString(thread["effort"]) != cas01Effort || asString(turn["model"]) != cas01Model || asString(turn["effort"]) != cas01Effort {
		return errors.New("pinned model policy is incomplete")
	}
	for _, params := range []map[string]any{thread, turn} {
		for key := range params {
			if key == "fallbackModel" || key == "fallbackModels" || key == "fallbackProvider" {
				return errors.New("fallback is forbidden")
			}
		}
	}
	return nil
}
func verifyModelCatalog(result map[string]any) error {
	models, ok := result["data"].([]any)
	if !ok {
		return errors.New("model catalog has no data")
	}
	for _, entry := range models {
		model, ok := entry.(map[string]any)
		if !ok || (asString(model["id"]) != cas01Model && asString(model["model"]) != cas01Model) {
			continue
		}
		efforts, ok := model["supportedReasoningEfforts"].([]any)
		if !ok {
			break
		}
		for _, entry := range efforts {
			effort, _ := entry.(map[string]any)
			if asString(effort["reasoningEffort"]) == cas01Effort {
				return nil
			}
		}
	}
	return errors.New("gpt-5.5 high is not available")
}
func (c *Client) call(method string, params map[string]any) (map[string]any, error) {
	id := c.next
	c.next++
	if err := c.write(map[string]any{"jsonrpc": "2.0", "id": id, "method": method, "params": params}); err != nil {
		return nil, err
	}
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		if !c.scan.Scan() {
			return nil, errors.New("transport closed")
		}
		var v map[string]any
		if json.Unmarshal(c.scan.Bytes(), &v) != nil {
			continue
		}
		if n, ok := v["id"].(float64); ok && int(n) == id {
			if _, failed := v["error"]; failed {
				c.recordTerminal(v["id"], "error")
				return nil, errors.New("server rejected request")
			}
			c.recordTerminal(v["id"], "result")
			r, _ := v["result"].(map[string]any)
			return r, nil
		}
		c.record("in", asString(v["method"]), fmt.Sprint(v["id"]))
	}
	return nil, errors.New("request timed out")
}
func (c *Client) waitTurn(threadID, turnID string) (terminalState, error) {
	deadline := time.Now().Add(2 * time.Minute)
	diagnostic := diagnostic{ErrorKinds: map[string]int{}}
	for time.Now().Before(deadline) {
		if !c.scan.Scan() {
			return terminalState{diagnostic: diagnostic}, terminalError{kind: "transport_closed"}
		}
		var v map[string]any
		if json.Unmarshal(c.scan.Bytes(), &v) != nil {
			continue
		}
		method := asString(v["method"])
		audit := c.record("in", method, fmt.Sprint(v["id"]))
		params, _ := v["params"].(map[string]any)
		switch method {
		case "error":
			if correlatedError(params, threadID, turnID) {
				recordError(&diagnostic, notificationErrorKind(params), asBool(params["willRetry"]))
			}
			continue
		case "model/rerouted":
			if params != nil && asString(params["threadId"]) == threadID && asString(params["turnId"]) == turnID {
				audit.Terminal = "model_rerouted"
				return terminalState{classification: "model_rerouted", diagnostic: diagnostic}, terminalError{kind: "model_rerouted"}
			}
			continue
		case "warning":
			if params != nil && asString(params["threadId"]) == threadID {
				diagnostic.Warnings++
			}
			continue
		case "turn/completed":
		default:
			continue
		}
		if params == nil {
			audit.Terminal = "malformed"
			return terminalState{classification: "malformed", diagnostic: diagnostic}, terminalError{kind: "malformed_terminal"}
		}
		if !correlated(params, threadID, turnID) {
			audit.Terminal = "correlation_mismatch"
			return terminalState{classification: "correlation_mismatch", diagnostic: diagnostic}, terminalError{kind: "correlation_mismatch"}
		}
		classification, known := safeTurnTerminal(nested(params, "turn", "status"))
		audit.Terminal = classification
		if classification == "failed" {
			diagnostic.TerminalErrorKind = turnErrorKind(params)
		}
		if !known || classification != "completed" {
			return terminalState{classification: classification, diagnostic: diagnostic}, terminalError{kind: "terminal_status_mismatch"}
		}
		return terminalState{classification: classification, diagnostic: diagnostic}, nil
	}
	return terminalState{diagnostic: diagnostic}, terminalError{kind: "timeout"}
}
func (c *Client) send(method string, params map[string]any) error {
	return c.write(map[string]any{"jsonrpc": "2.0", "method": method, "params": params})
}
func (c *Client) write(v map[string]any) error {
	c.record("out", asString(v["method"]), fmt.Sprint(v["id"]))
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	_, err = c.stdin.Write(append(data, '\n'))
	return err
}
func (c *Client) record(direction, method, id string) *Audit {
	*c.events = append(*c.events, Audit{Sequence: len(*c.events) + 1, Direction: direction, Method: method, ID: digest(id)})
	return &(*c.events)[len(*c.events)-1]
}
func (c *Client) recordTerminal(id any, terminal string) {
	*c.events = append(*c.events, Audit{Sequence: len(*c.events) + 1, Direction: "in", Method: "response", ID: digest(fmt.Sprint(id)), Terminal: terminal})
}
func safeErrorKind(err error) string {
	var terminal terminalError
	if errors.As(err, &terminal) {
		return terminal.kind
	}
	switch err.Error() {
	case "server rejected request":
		return "error"
	case "transport closed":
		return "transport_closed"
	case "request timed out":
		return "timeout"
	default:
		return "client_error"
	}
}
func safeTurnTerminal(status string) (string, bool) {
	switch status {
	case "completed", "interrupted", "failed":
		return status, true
	default:
		return "unexpected", false
	}
}
func correlated(params map[string]any, threadID, turnID string) bool {
	return params != nil && asString(params["threadId"]) == threadID && nested(params, "turn", "id") == turnID
}
func correlatedError(params map[string]any, threadID, turnID string) bool {
	return params != nil && asString(params["threadId"]) == threadID && asString(params["turnId"]) == turnID
}
func notificationErrorKind(params map[string]any) string {
	err, ok := params["error"].(map[string]any)
	if !ok {
		return "unknown"
	}
	return errorInfoKind(err)
}
func turnErrorKind(params map[string]any) string {
	turn, ok := params["turn"].(map[string]any)
	if !ok {
		return "unknown"
	}
	err, found := turn["error"]
	if !found || err == nil {
		return "none"
	}
	errorObject, ok := err.(map[string]any)
	if !ok {
		return "unknown"
	}
	return errorInfoKind(errorObject)
}
func errorInfoKind(err map[string]any) string {
	info, found := err["codexErrorInfo"]
	if !found || info == nil {
		return "none"
	}
	return safeCodexErrorKind(info)
}
func safeCodexErrorKind(value any) string {
	if value == nil || value == "" {
		return "none"
	}
	if kind, ok := value.(string); ok {
		switch kind {
		case "contextWindowExceeded", "sessionBudgetExceeded", "usageLimitExceeded", "serverOverloaded", "cyberPolicy", "internalServerError", "unauthorized", "badRequest", "threadRollbackFailed", "sandboxError", "other":
			return kind
		default:
			return "unknown"
		}
	}
	object, ok := value.(map[string]any)
	if !ok {
		return "unknown"
	}
	for _, kind := range []string{"httpConnectionFailed", "responseStreamConnectionFailed", "responseStreamDisconnected", "responseTooManyFailedAttempts", "activeTurnNotSteerable"} {
		if _, found := object[kind]; found {
			return kind
		}
	}
	return "unknown"
}
func recordError(d *diagnostic, kind string, willRetry bool) {
	if d.ErrorKinds == nil {
		d.ErrorKinds = map[string]int{}
	}
	d.ErrorKinds[kind]++
	if willRetry {
		d.RetriableErrors++
	}
}
func sameKinds(one, two map[string]int) bool {
	if len(one) != len(two) {
		return false
	}
	for kind, count := range one {
		if two[kind] != count {
			return false
		}
	}
	return true
}
func asBool(v any) bool { b, _ := v.(bool); return b }
func nested(v map[string]any, one, two string) string {
	m, _ := v[one].(map[string]any)
	return asString(m[two])
}
func asString(v any) string { s, _ := v.(string); return s }
func digest(v string) string {
	if v == "" || v == "<nil>" {
		return ""
	}
	sum := sha256.Sum256([]byte(v))
	return hex.EncodeToString(sum[:8])
}
