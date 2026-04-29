package modelclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type Options struct {
	NumCtx      int
	KeepAlive   string
	Temperature *float64
	NumPredict  int
	TopP        *float64
	Extra       map[string]any
}

func (o Options) OllamaPayload() map[string]any {
	out := map[string]any{}
	if o.NumCtx > 0 {
		out["num_ctx"] = o.NumCtx
	}
	if o.Temperature != nil {
		out["temperature"] = *o.Temperature
	}
	if o.NumPredict != 0 {
		out["num_predict"] = o.NumPredict
	}
	if o.TopP != nil {
		out["top_p"] = *o.TopP
	}
	for k, v := range o.Extra {
		if strings.TrimSpace(k) == "" {
			continue
		}
		out[k] = v
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

type Request struct {
	Model    string
	Prompt   string
	Messages []Message
	Options  Options
	Stream   bool
}

type CallRecord struct {
	Provider             string         `json:"provider,omitempty"`
	BaseURL              string         `json:"base_url,omitempty"`
	Endpoint             string         `json:"endpoint,omitempty"`
	Model                string         `json:"model,omitempty"`
	Stream               bool           `json:"stream,omitempty"`
	PromptChars          int            `json:"prompt_chars,omitempty"`
	MessageCount         int            `json:"message_count,omitempty"`
	Options              map[string]any `json:"options,omitempty"`
	KeepAlive            string         `json:"keep_alive,omitempty"`
	TotalDurationNS      int64          `json:"total_duration_ns,omitempty"`
	LoadDurationNS       int64          `json:"load_duration_ns,omitempty"`
	PromptEvalCount      int            `json:"prompt_eval_count,omitempty"`
	PromptEvalDurationNS int64          `json:"prompt_eval_duration_ns,omitempty"`
	EvalCount            int            `json:"eval_count,omitempty"`
	EvalDurationNS       int64          `json:"eval_duration_ns,omitempty"`
	DoneReason           string         `json:"done_reason,omitempty"`
}

type Response struct {
	Text string
	Call *CallRecord
}

type Client interface {
	Generate(ctx context.Context, req Request) (*Response, error)
	Chat(ctx context.Context, req Request, onToken func(string)) (*Response, error)
}

func DefaultProvider() string {
	if provider := strings.TrimSpace(os.Getenv("DORKPIPE_MODEL_PROVIDER")); provider != "" {
		return provider
	}
	return "ollama"
}

func New(provider, baseURL string) (Client, error) {
	provider = strings.ToLower(strings.TrimSpace(provider))
	if provider == "" {
		provider = DefaultProvider()
	}
	switch provider {
	case "ollama":
		return &ollamaClient{
			baseURL: strings.TrimSpace(baseURL),
			http:    &http.Client{Timeout: 15 * time.Minute},
		}, nil
	default:
		return nil, fmt.Errorf("modelclient: unsupported provider %q", provider)
	}
}

var gateOnce sync.Once
var gate chan struct{}

func acquireGate(ctx context.Context) error {
	gateOnce.Do(func() {
		raw := strings.TrimSpace(os.Getenv("DORKPIPE_MODEL_MAX_PARALLEL"))
		if raw == "" {
			return
		}
		n, err := strconv.Atoi(raw)
		if err != nil || n <= 0 {
			return
		}
		gate = make(chan struct{}, n)
	})
	if gate == nil {
		return nil
	}
	select {
	case gate <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func releaseGate() {
	if gate == nil {
		return
	}
	select {
	case <-gate:
	default:
	}
}

type ollamaClient struct {
	baseURL string
	http    *http.Client
}

func (c *ollamaClient) Generate(ctx context.Context, req Request) (*Response, error) {
	if err := acquireGate(ctx); err != nil {
		return nil, err
	}
	defer releaseGate()

	u, err := buildURL(c.baseURL, "/api/generate")
	if err != nil {
		return nil, err
	}
	payload := map[string]any{
		"model":  req.Model,
		"prompt": req.Prompt,
		"stream": false,
	}
	if opts := req.Options.OllamaPayload(); len(opts) > 0 {
		payload["options"] = opts
	}
	if strings.TrimSpace(req.Options.KeepAlive) != "" {
		payload["keep_alive"] = strings.TrimSpace(req.Options.KeepAlive)
	}
	call := baseCallRecord("ollama", c.baseURL, "/api/generate", req, payload)
	b, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	out, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("ollama: HTTP %d: %s", resp.StatusCode, string(out))
	}
	var parsed struct {
		Response           string `json:"response"`
		Model              string `json:"model"`
		TotalDuration      int64  `json:"total_duration"`
		LoadDuration       int64  `json:"load_duration"`
		PromptEvalCount    int    `json:"prompt_eval_count"`
		PromptEvalDuration int64  `json:"prompt_eval_duration"`
		EvalCount          int    `json:"eval_count"`
		EvalDuration       int64  `json:"eval_duration"`
		DoneReason         string `json:"done_reason"`
	}
	if err := json.Unmarshal(out, &parsed); err != nil {
		return nil, err
	}
	applyUsage(call, parsed.Model, parsed.TotalDuration, parsed.LoadDuration, parsed.PromptEvalCount, parsed.PromptEvalDuration, parsed.EvalCount, parsed.EvalDuration, parsed.DoneReason)
	return &Response{Text: parsed.Response, Call: call}, nil
}

func (c *ollamaClient) Chat(ctx context.Context, req Request, onToken func(string)) (*Response, error) {
	if err := acquireGate(ctx); err != nil {
		return nil, err
	}
	defer releaseGate()

	u, err := buildURL(c.baseURL, "/api/chat")
	if err != nil {
		return nil, err
	}
	payload := map[string]any{
		"model":    req.Model,
		"stream":   req.Stream,
		"messages": req.Messages,
	}
	if opts := req.Options.OllamaPayload(); len(opts) > 0 {
		payload["options"] = opts
	}
	if strings.TrimSpace(req.Options.KeepAlive) != "" {
		payload["keep_alive"] = strings.TrimSpace(req.Options.KeepAlive)
	}
	call := baseCallRecord("ollama", c.baseURL, "/api/chat", req, payload)
	b, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ollama: HTTP %d: %s", resp.StatusCode, string(body))
	}
	if !req.Stream {
		out, _ := io.ReadAll(resp.Body)
		var parsed struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
			Response           string `json:"response"`
			Model              string `json:"model"`
			TotalDuration      int64  `json:"total_duration"`
			LoadDuration       int64  `json:"load_duration"`
			PromptEvalCount    int    `json:"prompt_eval_count"`
			PromptEvalDuration int64  `json:"prompt_eval_duration"`
			EvalCount          int    `json:"eval_count"`
			EvalDuration       int64  `json:"eval_duration"`
			DoneReason         string `json:"done_reason"`
		}
		if err := json.Unmarshal(out, &parsed); err != nil {
			return nil, err
		}
		text := strings.TrimSpace(parsed.Message.Content)
		if text == "" {
			text = parsed.Response
		}
		applyUsage(call, parsed.Model, parsed.TotalDuration, parsed.LoadDuration, parsed.PromptEvalCount, parsed.PromptEvalDuration, parsed.EvalCount, parsed.EvalDuration, parsed.DoneReason)
		return &Response{Text: text, Call: call}, nil
	}

	decoder := json.NewDecoder(resp.Body)
	var full strings.Builder
	for {
		var obj map[string]any
		if err := decoder.Decode(&obj); err != nil {
			if err == io.EOF {
				break
			}
			return &Response{Text: full.String(), Call: call}, err
		}
		piece := nestedString(obj, "message", "content")
		if piece == "" {
			piece = stringValue(obj["response"])
		}
		if piece != "" {
			full.WriteString(piece)
			if onToken != nil {
				onToken(piece)
			}
		}
		if done, _ := obj["done"].(bool); done {
			applyUsage(
				call,
				stringValue(obj["model"]),
				int64Value(obj["total_duration"]),
				int64Value(obj["load_duration"]),
				intValue(obj["prompt_eval_count"]),
				int64Value(obj["prompt_eval_duration"]),
				intValue(obj["eval_count"]),
				int64Value(obj["eval_duration"]),
				stringValue(obj["done_reason"]),
			)
		}
	}
	return &Response{Text: full.String(), Call: call}, nil
}

func buildURL(rawBase, path string) (*url.URL, error) {
	s := strings.TrimSpace(rawBase)
	s = strings.TrimSuffix(s, "/")
	if s == "" {
		return nil, fmt.Errorf("modelclient: empty base URL")
	}
	if !strings.Contains(s, "://") {
		s = "http://" + s
	}
	u, err := url.Parse(s)
	if err != nil {
		return nil, fmt.Errorf("modelclient: parse base URL: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("modelclient: only http and https URLs are allowed")
	}
	if strings.TrimSpace(u.Host) == "" {
		return nil, fmt.Errorf("modelclient: URL must include a host")
	}
	return &url.URL{Scheme: u.Scheme, Host: u.Host, Path: path}, nil
}

func baseCallRecord(provider, baseURL, endpoint string, req Request, payload map[string]any) *CallRecord {
	call := &CallRecord{
		Provider:     provider,
		BaseURL:      strings.TrimSpace(baseURL),
		Endpoint:     endpoint,
		Model:        req.Model,
		Stream:       req.Stream,
		PromptChars:  len(req.Prompt),
		MessageCount: len(req.Messages),
		KeepAlive:    strings.TrimSpace(req.Options.KeepAlive),
	}
	if opts, ok := payload["options"].(map[string]any); ok && len(opts) > 0 {
		call.Options = opts
	}
	return call
}

func applyUsage(call *CallRecord, model string, totalDuration, loadDuration int64, promptEvalCount int, promptEvalDuration int64, evalCount int, evalDuration int64, doneReason string) {
	call.Model = firstNonEmpty(strings.TrimSpace(model), call.Model)
	call.TotalDurationNS = totalDuration
	call.LoadDurationNS = loadDuration
	call.PromptEvalCount = promptEvalCount
	call.PromptEvalDurationNS = promptEvalDuration
	call.EvalCount = evalCount
	call.EvalDurationNS = evalDuration
	call.DoneReason = strings.TrimSpace(doneReason)
}

func nestedString(obj map[string]any, outer, inner string) string {
	v, ok := obj[outer]
	if !ok {
		return ""
	}
	m, ok := v.(map[string]any)
	if !ok {
		return ""
	}
	return stringValue(m[inner])
}

func stringValue(v any) string {
	s, _ := v.(string)
	return s
}

func intValue(v any) int {
	switch x := v.(type) {
	case int:
		return x
	case int64:
		return int(x)
	case float64:
		return int(x)
	default:
		return 0
	}
}

func int64Value(v any) int64 {
	switch x := v.(type) {
	case int:
		return int64(x)
	case int64:
		return x
	case float64:
		return int64(x)
	default:
		return 0
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
