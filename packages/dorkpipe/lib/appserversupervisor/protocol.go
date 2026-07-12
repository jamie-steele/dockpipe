package appserversupervisor

// This file deliberately keeps provider JSON-RPC shapes inside the supervisor
// package. It never retains raw frames, error bodies, credentials, or IDs.

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"regexp"
	"strings"
	"sync"
	"time"
)

const (
	PinnedModel           = "gpt-5.6-terra"
	PinnedReasoningEffort = "high"
)

// InitializationConfig is the closed configuration surface used before a
// supervisor becomes ready. Model policy is retained here for later CAS-05
// work, but CAS-04 sends no thread, turn, catalog, or policy request.
type InitializationConfig struct {
	SchemaVersion        string
	RequiredCapabilities []string
	ClientName           string
	ClientVersion        string
	Model                string
	ReasoningEffort      string
}

func (c InitializationConfig) validate() error {
	if strings.TrimSpace(c.SchemaVersion) == "" || len(c.RequiredCapabilities) == 0 || strings.TrimSpace(c.ClientName) == "" || strings.TrimSpace(c.ClientVersion) == "" {
		return errors.New("schema, required capabilities, and client identity are required")
	}
	if c.Model != PinnedModel || c.ReasoningEffort != PinnedReasoningEffort {
		return errors.New("model and reasoning effort must use the CAS-01 pinned policy")
	}
	if len(c.RequiredCapabilities) > maxRequiredCapabilities || len(c.SchemaVersion) > maxInitializationValues || len(c.ClientName) > maxInitializationValues || len(c.ClientVersion) > maxInitializationValues || strings.TrimSpace(c.SchemaVersion) != c.SchemaVersion || strings.TrimSpace(c.ClientName) != c.ClientName || strings.TrimSpace(c.ClientVersion) != c.ClientVersion || !safeCapability(c.SchemaVersion) || !validID(c.ClientName) || !safeVersion(c.ClientVersion) {
		return errors.New("initialization values must be bounded safe identifiers")
	}
	seen := map[string]bool{}
	for _, capability := range c.RequiredCapabilities {
		if !safeCapability(capability) || seen[capability] {
			return errors.New("required capabilities must be unique safe names")
		}
		seen[capability] = true
	}
	return nil
}

// InitializationInfo is the only initialization result retained by CAS-04.
// Warning summaries are allow-listed classes rather than provider text.
type InitializationInfo struct {
	SchemaVersion         string
	ProviderVersion       string
	IdentityClass         string
	ConfigurationWarnings []string
	Model                 string
	ReasoningEffort       string
}

type protocolClient struct {
	stdin    io.WriteCloser
	failFunc func(DisconnectReason)
	liveness time.Duration

	mu            sync.Mutex
	nextID        uint64
	pending       map[uint64]chan json.RawMessage
	reason        DisconnectReason
	done          chan struct{}
	failOnce      sync.Once
	writeMu       sync.Mutex
	activity      chan struct{}
	eventNotify   func(string, json.RawMessage) DisconnectReason
	requestNotify func(uint64, string, json.RawMessage) DisconnectReason
}

func newProtocolClient(stdin io.WriteCloser, stdout io.ReadCloser, liveness time.Duration, failFunc func(DisconnectReason)) *protocolClient {
	return newProtocolClientWithNotifications(stdin, stdout, liveness, failFunc, nil)
}

func newProtocolClientWithNotifications(stdin io.WriteCloser, stdout io.ReadCloser, liveness time.Duration, failFunc func(DisconnectReason), notify func(string, json.RawMessage) DisconnectReason) *protocolClient {
	return newProtocolClientWithHandlers(stdin, stdout, liveness, failFunc, notify, nil)
}

func newProtocolClientWithHandlers(stdin io.WriteCloser, stdout io.ReadCloser, liveness time.Duration, failFunc func(DisconnectReason), notify func(string, json.RawMessage) DisconnectReason, requestNotify func(uint64, string, json.RawMessage) DisconnectReason) *protocolClient {
	c := &protocolClient{stdin: stdin, failFunc: failFunc, liveness: liveness, pending: map[uint64]chan json.RawMessage{}, done: make(chan struct{}), activity: make(chan struct{}, 1), eventNotify: notify, requestNotify: requestNotify}
	go c.read(stdout)
	go c.watchLiveness()
	return c
}

func (c *protocolClient) respond(ctx context.Context, id uint64, result map[string]any) error {
	if id == 0 {
		c.fail(DisconnectMalformedEnvelope)
		return errors.New("server request identifier is required")
	}
	return c.write(ctx, map[string]any{"jsonrpc": "2.0", "id": id, "result": result})
}

func (c *protocolClient) initialize(parent context.Context, deadline time.Duration, config InitializationConfig) (InitializationInfo, error) {
	ctx, cancel := context.WithTimeout(parent, deadline)
	defer cancel()
	result, err := c.request(ctx, "initialize", map[string]any{
		"clientInfo":   map[string]any{"name": config.ClientName, "version": config.ClientVersion},
		"capabilities": map[string]any{},
	})
	if err != nil {
		return InitializationInfo{}, err
	}
	info, reason := projectInitialization(result, config)
	if reason != "" {
		c.fail(reason)
		return InitializationInfo{}, errors.New("initialization gate rejected response")
	}
	if err := c.notify(ctx, "initialized", map[string]any{}); err != nil {
		return InitializationInfo{}, err
	}
	return info, nil
}

func (c *protocolClient) request(ctx context.Context, method string, params map[string]any) (json.RawMessage, error) {
	c.mu.Lock()
	c.nextID++
	id := c.nextID
	response := make(chan json.RawMessage, 1)
	c.pending[id] = response
	c.mu.Unlock()
	defer func() {
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
	}()
	if err := c.write(ctx, map[string]any{"jsonrpc": "2.0", "id": id, "method": method, "params": params}); err != nil {
		return nil, err
	}
	select {
	case result := <-response:
		return result, nil
	case <-c.done:
		return nil, errors.New("transport failed")
	case <-ctx.Done():
		c.fail(DisconnectRequestDeadline)
		return nil, ctx.Err()
	}
}

func (c *protocolClient) notify(ctx context.Context, method string, params map[string]any) error {
	return c.write(ctx, map[string]any{"jsonrpc": "2.0", "method": method, "params": params})
}

func (c *protocolClient) write(ctx context.Context, value map[string]any) error {
	data, err := json.Marshal(value)
	if err != nil {
		c.fail(DisconnectInitializationRejected)
		return err
	}
	result := make(chan error, 1)
	go func() {
		c.writeMu.Lock()
		defer c.writeMu.Unlock()
		_, err := c.stdin.Write(append(data, '\n'))
		result <- err
	}()
	select {
	case err := <-result:
		if err != nil {
			c.fail(DisconnectTransportClosed)
		}
		return err
	case <-c.done:
		return errors.New("transport failed")
	case <-ctx.Done():
		c.fail(DisconnectRequestDeadline)
		return ctx.Err()
	}
}

func (c *protocolClient) read(stdout io.ReadCloser) {
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 4096), 1<<20)
	for scanner.Scan() {
		c.touch()
		if !c.handle(scanner.Bytes()) {
			return
		}
	}
	if scanner.Err() != nil {
		c.fail(DisconnectMalformedInput)
		return
	}
	c.fail(DisconnectTransportClosed)
}

type rpcEnvelope struct {
	JSONRPC string           `json:"jsonrpc"`
	ID      *json.RawMessage `json:"id"`
	Method  *string          `json:"method"`
	Params  json.RawMessage  `json:"params"`
	Result  json.RawMessage  `json:"result"`
	Error   json.RawMessage  `json:"error"`
}

func (c *protocolClient) handle(frame []byte) bool {
	var envelope rpcEnvelope
	if json.Unmarshal(frame, &envelope) != nil || envelope.JSONRPC != "2.0" {
		c.fail(DisconnectMalformedEnvelope)
		return false
	}
	if envelope.ID != nil {
		var id uint64
		if json.Unmarshal(*envelope.ID, &id) != nil || id == 0 {
			c.fail(DisconnectMalformedEnvelope)
			return false
		}
		if envelope.Method != nil {
			if strings.TrimSpace(*envelope.Method) == "" || len(envelope.Result) != 0 || len(envelope.Error) != 0 || len(envelope.Params) == 0 || c.requestNotify == nil {
				c.fail(DisconnectMalformedEnvelope)
				return false
			}
			if containsModelReroute(envelope.Params) {
				c.fail(DisconnectModelRerouted)
				return false
			}
			if reason := c.requestNotify(id, *envelope.Method, append(json.RawMessage(nil), envelope.Params...)); reason != "" {
				c.fail(reason)
				return false
			}
			return true
		}
		if len(envelope.Result) == 0 && len(envelope.Error) == 0 || len(envelope.Result) != 0 && len(envelope.Error) != 0 {
			c.fail(DisconnectMalformedEnvelope)
			return false
		}
		if len(envelope.Error) != 0 {
			c.fail(DisconnectProviderError)
			return false
		}
		c.mu.Lock()
		response, found := c.pending[id]
		c.mu.Unlock()
		if !found {
			c.fail(DisconnectCorrelationMismatch)
			return false
		}
		response <- append(json.RawMessage(nil), envelope.Result...)
		return true
	}
	if envelope.Method == nil || strings.TrimSpace(*envelope.Method) == "" || len(envelope.Result) != 0 || len(envelope.Error) != 0 {
		c.fail(DisconnectMalformedEnvelope)
		return false
	}
	if strings.Contains(strings.ToLower(*envelope.Method), "model/rerout") || containsModelReroute(envelope.Params) {
		c.fail(DisconnectModelRerouted)
		return false
	}
	if c.eventNotify != nil {
		if reason := c.eventNotify(*envelope.Method, append(json.RawMessage(nil), envelope.Params...)); reason != "" {
			c.fail(reason)
			return false
		}
	}
	return true
}

func (c *protocolClient) watchLiveness() {
	timer := time.NewTimer(c.liveness)
	defer timer.Stop()
	for {
		select {
		case <-c.done:
			return
		case <-c.activity:
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			timer.Reset(c.liveness)
		case <-timer.C:
			c.fail(DisconnectLivenessDeadline)
			return
		}
	}
}

func (c *protocolClient) touch() {
	select {
	case c.activity <- struct{}{}:
	default:
	}
}

func (c *protocolClient) fail(reason DisconnectReason) {
	if reason == "" {
		reason = DisconnectInitializationRejected
	}
	c.failOnce.Do(func() {
		c.mu.Lock()
		c.reason = reason
		c.mu.Unlock()
		close(c.done)
		c.failFunc(reason)
	})
}

func (c *protocolClient) failureReason() DisconnectReason {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.reason == "" {
		return DisconnectInitializationRejected
	}
	return c.reason
}

type initializeResponse struct {
	ProtocolVersion string                         `json:"protocolVersion"`
	ServerInfo      struct{ Name, Version string } `json:"serverInfo"`
	Capabilities    map[string]json.RawMessage     `json:"capabilities"`
	ConfigWarnings  []json.RawMessage              `json:"configWarnings"`
}

func projectInitialization(result json.RawMessage, config InitializationConfig) (InitializationInfo, DisconnectReason) {
	fields, ok := objectFields(result, "protocolVersion", "serverInfo", "capabilities", "configWarnings")
	if !ok || !nestedObjectFields(fields["serverInfo"], "name", "version") || !nestedObjectFields(fields["capabilities"], config.RequiredCapabilities...) {
		return InitializationInfo{}, DisconnectUnsupportedSchema
	}
	var response initializeResponse
	if json.Unmarshal(result, &response) != nil || response.ProtocolVersion != config.SchemaVersion || response.Capabilities == nil {
		return InitializationInfo{}, DisconnectUnsupportedSchema
	}
	for _, required := range config.RequiredCapabilities {
		var enabled bool
		if raw, ok := response.Capabilities[required]; !ok || json.Unmarshal(raw, &enabled) != nil || !enabled {
			return InitializationInfo{}, DisconnectUnsupportedCapability
		}
	}
	if identityClass(response.ServerInfo.Name) == "" || !safeVersion(response.ServerInfo.Version) {
		return InitializationInfo{}, DisconnectInitializationRejected
	}
	if len(response.ConfigWarnings) > 4 {
		return InitializationInfo{}, DisconnectInitializationRejected
	}
	warnings := make([]string, 0, len(response.ConfigWarnings))
	for _, warning := range response.ConfigWarnings {
		if !warningShapeAllowed(warning) {
			return InitializationInfo{}, DisconnectInitializationRejected
		}
		if len(warnings) == 4 {
			break
		}
		warnings = append(warnings, classifyWarning(warning))
	}
	return InitializationInfo{SchemaVersion: response.ProtocolVersion, ProviderVersion: response.ServerInfo.Version, IdentityClass: identityClass(response.ServerInfo.Name), ConfigurationWarnings: warnings, Model: config.Model, ReasoningEffort: config.ReasoningEffort}, ""
}

func warningShapeAllowed(raw json.RawMessage) bool {
	var text string
	if json.Unmarshal(raw, &text) == nil {
		return true
	}
	return nestedObjectFields(raw, "code", "message")
}

func identityClass(name string) string {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "codex", "codex-app-server":
		return "codex_app_server"
	default:
		return ""
	}
}

var versionPattern = regexp.MustCompile(`^[0-9]+(?:\.[0-9]+){1,3}(?:-[a-z0-9]+)?$`)
var capabilityPattern = regexp.MustCompile(`^[a-z][A-Za-z0-9]{0,63}$`)

func safeVersion(version string) bool {
	return len(version) <= 32 && versionPattern.MatchString(version)
}
func safeCapability(capability string) bool { return capabilityPattern.MatchString(capability) }

func classifyWarning(raw json.RawMessage) string {
	var code string
	if json.Unmarshal(raw, &code) != nil {
		var object struct {
			Code string `json:"code"`
		}
		if json.Unmarshal(raw, &object) != nil {
			return "unclassified"
		}
		code = object.Code
	}
	switch code {
	case "config_deprecated", "config_ignored", "config_override":
		return code
	default:
		return "unclassified"
	}
}
