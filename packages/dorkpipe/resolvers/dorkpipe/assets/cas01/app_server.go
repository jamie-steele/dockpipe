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

type Event struct {
	Kind      string
	ThreadID  string
	TurnID    string
	ItemID    string
	RequestID string
	Status    string
}
type Policy struct {
	Sandbox  string
	Reviewer string
	Shell    bool
}
type Expect struct {
	Status   string
	Denied   int
	Rejected int
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
	status   string
	thread   string
	turn     string
	items    map[string]bool
	pending  map[string]pending
	seen     map[string]bool
	denied   int
	rejected int
}
type Audit struct {
	Sequence  int    `json:"sequence"`
	Direction string `json:"direction"`
	Method    string `json:"method"`
	ID        string `json:"id,omitempty"`
}
type Evidence struct {
	Harness     string           `json:"harness"`
	Protocol    string           `json:"protocol"`
	Outcome     string           `json:"outcome"`
	StartedAt   string           `json:"startedAt"`
	CompletedAt string           `json:"completedAt"`
	Durations   map[string]int64 `json:"durationsMs"`
	Events      []Audit          `json:"events"`
	Redaction   string           `json:"redaction"`
	Blocker     string           `json:"blocker,omitempty"`
}
type Client struct {
	stdin  io.WriteCloser
	scan   *bufio.Scanner
	next   int
	events *[]Audit
}

func main() {
	mode := flag.String("mode", "fixtures", "fixtures, policy, or live")
	fixtures := flag.String("fixtures", "", "absolute fixture file")
	sandbox := flag.String("sandbox", "workspace-write", "sandbox")
	reviewer := flag.String("reviewer", "user", "reviewer")
	method := flag.String("method", "", "method")
	artifacts := flag.String("artifacts", "", "absolute artifact directory")
	codex := flag.String("codex", "", "absolute Codex CLI path")
	workspace := flag.String("workspace", "", "absolute workspace")
	turn := flag.Bool("start-turn", false, "start a cloud-backed turn")
	ack := flag.Bool("ack-cloud-spend", false, "acknowledge cloud-backed work")
	flag.Parse()
	var err error
	switch *mode {
	case "fixtures":
		err = validateFixtures(*fixtures)
	case "policy":
		err = policy(*sandbox, *reviewer, *method)
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
func policy(sandbox, reviewer, method string) error {
	if sandbox != "workspace-write" {
		return errors.New("only workspace-write is permitted")
	}
	if reviewer != "user" {
		return errors.New("only a human user reviewer is permitted")
	}
	if method == "thread/shellCommand" {
		return errors.New("thread shell command is forbidden")
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
func validate(f Fixture) error {
	if f.Name == "" || len(f.Events) == 0 || f.Policy.Shell {
		return errors.New("fixture name, events, and no-shell policy are required")
	}
	if err := policy(f.Policy.Sandbox, f.Policy.Reviewer, ""); err != nil {
		return err
	}
	s := state{status: "Ready", items: map[string]bool{}, pending: map[string]pending{}, seen: map[string]bool{}}
	for _, e := range f.Events {
		if e.Kind == "" {
			return errors.New("event kind is required")
		}
		key := e.Kind + ":" + e.ThreadID + ":" + e.TurnID + ":" + e.ItemID + ":" + e.RequestID
		if s.seen[key] && e.Kind != "approval_replay" {
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
			if e.Status != "completed" && e.Status != "interrupted" && e.Status != "failed" {
				return errors.New("unknown terminal state")
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
	if s.status != f.Expect.Status || s.denied != f.Expect.Denied || s.rejected != f.Expect.Rejected {
		return fmt.Errorf("got status=%s denied=%d rejected=%d", s.status, s.denied, s.rejected)
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
	e := Evidence{Harness: "cas-01", Protocol: "stdio", Outcome: "Blocked", StartedAt: time.Now().UTC().Format(time.RFC3339), Durations: map[string]int64{}, Redaction: "method, timing, and SHA-256 request IDs only; no raw RPC, prompt, command, diff, stderr, or credentials"}
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
	started = time.Now()
	r, err := c.call("thread/start", map[string]any{"cwd": workspace, "sandbox": "workspace-write", "approvalPolicy": "untrusted", "approvalsReviewer": "user"})
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
	if _, err = c.call("thread/resume", map[string]any{"threadId": threadID, "cwd": workspace, "sandbox": "workspace-write", "approvalPolicy": "untrusted", "approvalsReviewer": "user"}); err != nil {
		e.Blocker = "thread_resume"
		return errors.New("thread resume failed without retaining server payload")
	}
	e.Durations["read_resume"] = time.Since(started).Milliseconds()
	if !turn {
		e.Outcome = "Ready"
		return nil
	}
	_, err = c.call("turn/start", map[string]any{"threadId": threadID, "cwd": workspace, "sandboxPolicy": map[string]any{"type": "workspaceWrite", "writableRoots": []string{workspace}, "networkAccess": false}, "approvalPolicy": "untrusted", "approvalsReviewer": "user", "input": []any{map[string]any{"type": "text", "text": "Reply only CAS01_OK. Do not use tools or change anything."}}})
	if err != nil {
		e.Blocker = "turn_start"
		return errors.New("turn start failed without retaining server payload")
	}
	e.Outcome = "Started"
	return nil
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
				return nil, errors.New("server rejected request")
			}
			r, _ := v["result"].(map[string]any)
			return r, nil
		}
		c.record("in", asString(v["method"]), fmt.Sprint(v["id"]))
	}
	return nil, errors.New("request timed out")
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
func (c *Client) record(direction, method, id string) {
	*c.events = append(*c.events, Audit{Sequence: len(*c.events) + 1, Direction: direction, Method: method, ID: digest(id)})
}
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
