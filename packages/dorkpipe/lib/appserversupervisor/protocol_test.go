package appserversupervisor

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func beginInitialize(t *testing.T, s *Supervisor, child *fakeChild) (<-chan error, uint64) {
	t.Helper()
	done := make(chan error, 1)
	go func() { done <- s.Start(context.Background()) }()
	scanner := bufio.NewScanner(child.stdinR)
	if !scanner.Scan() {
		t.Fatal("expected initialize request")
	}
	var request struct {
		ID     uint64 `json:"id"`
		Method string `json:"method"`
	}
	if err := json.Unmarshal(scanner.Bytes(), &request); err != nil || request.ID != 1 || request.Method != "initialize" {
		t.Fatalf("initialize request = %s, err=%v", scanner.Text(), err)
	}
	return done, request.ID
}

func response(id uint64, result string) string {
	return `{"jsonrpc":"2.0","id":` + strconv.FormatUint(id, 10) + `,"result":` + result + "}\n"
}

func TestInitializeProjectsOnlyAllowlistedInformation(t *testing.T) {
	child := newFakeChild()
	s := newTestSupervisor(t, fakeLauncher{start: func(context.Context) (Child, error) { return child, nil }}, testDeadlines())
	if err := startInitialized(t, s, child, ""); err != nil {
		t.Fatal(err)
	}
	info := s.Initialization()
	if info.SchemaVersion != "v2" || info.ProviderVersion != "0.144.1" || info.IdentityClass != "codex_app_server" || len(info.ConfigurationWarnings) != 1 || info.ConfigurationWarnings[0] != "config_deprecated" || info.Model != PinnedModel || info.ReasoningEffort != PinnedReasoningEffort {
		t.Fatalf("unsafe or incomplete initialization projection: %+v", info)
	}
	if event := nextEvent(t, s); event.State != "ready" || event.Summary != "initialized" {
		t.Fatalf("ready event = %+v", event)
	}
}

func TestCorrelationMismatchFailsClosed(t *testing.T) {
	child := newFakeChild()
	s := newTestSupervisor(t, fakeLauncher{start: func(context.Context) (Child, error) { return child, nil }}, testDeadlines())
	done, id := beginInitialize(t, s, child)
	_, _ = child.stdoutW.Write([]byte(response(id+1, `{}`)))
	if err := <-done; err == nil {
		t.Fatal("expected initialize failure")
	}
	event := nextEvent(t, s)
	if event.State != "disconnected" || event.Summary != string(DisconnectCorrelationMismatch) {
		t.Fatalf("event = %+v", event)
	}
}

func TestRequestIDsAdvanceMonotonically(t *testing.T) {
	child := newFakeChild()
	failures := make(chan DisconnectReason, 1)
	client := newProtocolClient(child.stdinW, child.stdoutR, time.Second, func(reason DisconnectReason) { failures <- reason })
	scanner := bufio.NewScanner(child.stdinR)
	for want := uint64(1); want <= 2; want++ {
		completed := make(chan error, 1)
		go func() {
			_, err := client.request(context.Background(), "test/request", map[string]any{})
			completed <- err
		}()
		if !scanner.Scan() {
			t.Fatal("expected request")
		}
		var request struct {
			ID uint64 `json:"id"`
		}
		if err := json.Unmarshal(scanner.Bytes(), &request); err != nil || request.ID != want {
			t.Fatalf("request ID = %d, want %d (err=%v)", request.ID, want, err)
		}
		_, _ = child.stdoutW.Write([]byte(response(request.ID, `{}`)))
		if err := <-completed; err != nil {
			t.Fatal(err)
		}
	}
	select {
	case reason := <-failures:
		t.Fatalf("unexpected protocol failure: %s", reason)
	default:
	}
	_ = child.stdoutW.Close()
}

func TestMalformedEnvelopeAndProviderErrorFailClosed(t *testing.T) {
	for name, fixture := range map[string]struct {
		frame  string
		reason DisconnectReason
	}{
		"malformed":      {frame: "not-json\n", reason: DisconnectMalformedEnvelope},
		"provider_error": {frame: `{"jsonrpc":"2.0","id":1,"error":{"message":"do not retain"}}` + "\n", reason: DisconnectProviderError},
	} {
		t.Run(name, func(t *testing.T) {
			child := newFakeChild()
			s := newTestSupervisor(t, fakeLauncher{start: func(context.Context) (Child, error) { return child, nil }}, testDeadlines())
			done, _ := beginInitialize(t, s, child)
			_, _ = child.stdoutW.Write([]byte(fixture.frame))
			if err := <-done; err == nil {
				t.Fatal("expected initialize failure")
			}
			event := nextEvent(t, s)
			if event.State != "disconnected" || event.Summary != string(fixture.reason) {
				t.Fatalf("event = %+v", event)
			}
		})
	}
}

func TestInitializeDeadlineFailsClosed(t *testing.T) {
	child := newFakeChild()
	deadlines := testDeadlines()
	deadlines.Request = 20 * time.Millisecond
	s := newTestSupervisor(t, fakeLauncher{start: func(context.Context) (Child, error) { return child, nil }}, deadlines)
	done, _ := beginInitialize(t, s, child)
	if err := <-done; err == nil {
		t.Fatal("expected request deadline")
	}
	event := nextEvent(t, s)
	if event.State != "disconnected" || event.Summary != string(DisconnectRequestDeadline) {
		t.Fatalf("event = %+v", event)
	}
}

func TestSchemaAndCapabilityRejection(t *testing.T) {
	for name, fixture := range map[string]struct {
		result string
		reason DisconnectReason
	}{
		"schema":     {result: `{"protocolVersion":"v1","serverInfo":{"name":"codex","version":"0.144.1"},"capabilities":{"stableV2":true}}`, reason: DisconnectUnsupportedSchema},
		"capability": {result: `{"protocolVersion":"v2","serverInfo":{"name":"codex","version":"0.144.1"},"capabilities":{"stableV2":false}}`, reason: DisconnectUnsupportedCapability},
	} {
		t.Run(name, func(t *testing.T) {
			child := newFakeChild()
			s := newTestSupervisor(t, fakeLauncher{start: func(context.Context) (Child, error) { return child, nil }}, testDeadlines())
			done, id := beginInitialize(t, s, child)
			_, _ = child.stdoutW.Write([]byte(response(id, fixture.result)))
			if err := <-done; err == nil {
				t.Fatal("expected initialization rejection")
			}
			event := nextEvent(t, s)
			if event.State != "disconnected" || event.Summary != string(fixture.reason) {
				t.Fatalf("event = %+v", event)
			}
		})
	}
}

func TestModelRerouteIndicationFailsClosed(t *testing.T) {
	child := newFakeChild()
	s := newTestSupervisor(t, fakeLauncher{start: func(context.Context) (Child, error) { return child, nil }}, testDeadlines())
	if err := startInitialized(t, s, child, `{"protocolVersion":"v2","serverInfo":{"name":"codex","version":"0.144.1"},"capabilities":{"stableV2":true}}`); err != nil {
		t.Fatal(err)
	}
	_, _ = child.stdoutW.Write([]byte(`{"jsonrpc":"2.0","method":"model/rerouted","params":{}}` + "\n"))
	if ready := nextEvent(t, s); ready.State != "ready" {
		t.Fatalf("event = %+v", ready)
	}
	event := nextEvent(t, s)
	if event.State != "disconnected" || event.Summary != string(DisconnectModelRerouted) {
		t.Fatalf("event = %+v", event)
	}
}

func TestProtocolBoundaryContainsNoGenericOrPipeonLeak(t *testing.T) {
	forbidden := []string{"jsonrpc", "rawmessage", "appserversupervisor", "threadid", "turnid", "itemid"}
	paths := []string{filepath.Join("..", "providersession", "contract.go")}
	if err := filepath.WalkDir(filepath.Join("..", "..", "..", "pipeon"), func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() || !strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, ".ts") {
			return err
		}
		paths = append(paths, path)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for _, path := range paths[:1] {
		contents, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		lower := strings.ToLower(string(contents))
		for _, token := range append(forbidden, "requestapproval", "requestuserinput", "turn/interrupt") {
			if strings.Contains(lower, token) {
				t.Fatalf("provider protocol boundary leak %q in %s", token, path)
			}
		}
	}
	for _, path := range paths[1:] {
		contents, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		lower := strings.ToLower(string(contents))
		for _, token := range []string{"appserversupervisor", "model/rerouted", "thread/start", "turn/start", "turn/interrupt", "requestapproval", "requestuserinput", "json.rawmessage"} {
			if strings.Contains(lower, token) {
				t.Fatalf("app server protocol boundary leak %q in %s", token, path)
			}
		}
	}
}

func TestProtocolClientOffersNoRetryResumeReplayOrFallback(t *testing.T) {
	contents, err := os.ReadFile("protocol.go")
	if err != nil {
		t.Fatal(err)
	}
	lower := strings.ToLower(string(contents))
	for _, forbidden := range []string{"retry", "resume", "replay", "fallback"} {
		if strings.Contains(lower, forbidden) {
			t.Fatalf("forbidden recovery operation %q found in protocol client", forbidden)
		}
	}
}
