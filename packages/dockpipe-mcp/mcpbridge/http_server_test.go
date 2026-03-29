package mcpbridge

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAcceptsJSONContentType(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		ct   string
		want bool
	}{
		{"", true},
		{"application/json", true},
		{"application/json; charset=utf-8", true},
		{"APPLICATION/JSON", true},
		{"application/ld+json", true},
		{"text/json", true},
		{"application/octet-stream", false},
		{"text/plain", false},
	} {
		if got := acceptsJSONContentType(tc.ct); got != tc.want {
			t.Errorf("%q: got %v want %v", tc.ct, got, tc.want)
		}
	}
}

func TestIsLoopbackBind(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		addr string
		want bool
	}{
		{"127.0.0.1:8443", true},
		{"[::1]:8443", true},
		{"localhost:9", true},
		{":8443", false},
		{"0.0.0.0:8443", false},
		{"10.0.0.1:8443", false},
	} {
		if got := isLoopbackBind(tc.addr); got != tc.want {
			t.Errorf("%q: got %v want %v", tc.addr, got, tc.want)
		}
	}
}

func TestServeHTTPRequiresAPIKeyAndTLSOrLoopback(t *testing.T) {
	t.Parallel()
	s := NewServer("test")
	ctx := context.Background()

	err := s.ServeHTTP(ctx, HTTPConfig{ListenAddr: "127.0.0.1:0", APIKey: ""})
	if err == nil || !strings.Contains(err.Error(), "MCP_HTTP") {
		t.Fatalf("expected HTTP auth config error, got %v", err)
	}

	err = s.ServeHTTP(ctx, HTTPConfig{ListenAddr: "127.0.0.1:0", APIKey: "k"})
	if err == nil || !strings.Contains(err.Error(), "HTTPS") && !strings.Contains(err.Error(), "TLS") {
		t.Fatalf("expected TLS / insecure hint, got %v", err)
	}

	err = s.ServeHTTP(ctx, HTTPConfig{ListenAddr: ":8443", APIKey: "k", AllowInsecurePlainHTTP: true})
	if err == nil || !strings.Contains(err.Error(), "loopback") {
		t.Fatalf("expected loopback error for :port, got %v", err)
	}
}

func TestAPIKeyGateUnauthorized(t *testing.T) {
	t.Parallel()
	s := NewServer("1")
	h := s.jsonRPCHandler(io.Discard)
	srv := httptest.NewServer(apiKeyGate("secret", h))
	defer srv.Close()

	resp, err := http.Post(srv.URL, "application/json", bytes.NewReader([]byte(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"t","version":"1"}}}`)))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status %d", resp.StatusCode)
	}
}

func TestAPIKeyGateOK(t *testing.T) {
	t.Parallel()
	s := NewServer("1")
	h := s.jsonRPCHandler(io.Discard)
	srv := httptest.NewServer(apiKeyGate("secret", h))
	defer srv.Close()

	req, err := http.NewRequest(http.MethodPost, srv.URL, bytes.NewReader([]byte(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"t","version":"1"}}}`)))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer secret")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status %d", resp.StatusCode)
	}
	var out json.RawMessage
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
}

func TestHTTPKeyTierGateReadonlyVsExec(t *testing.T) {
	t.Setenv("DOCKPIPE_MCP_TIER", "")
	t.Setenv("DOCKPIPE_MCP_ALLOW_EXEC", "")
	s := NewServer("1")
	entries := []keyTierEntry{
		{key: "ro", tier: TierReadonly},
		{key: "ex", tier: TierExec},
	}
	h := httpKeyTierGate(entries, s.jsonRPCHandler(io.Discard))
	srv := httptest.NewServer(h)
	defer srv.Close()

	body := []byte(`{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}`)
	reqRO, err := http.NewRequest(http.MethodPost, srv.URL, bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	reqRO.Header.Set("Content-Type", "application/json")
	reqRO.Header.Set("Authorization", "Bearer ro")
	respRO, err := http.DefaultClient.Do(reqRO)
	if err != nil {
		t.Fatal(err)
	}
	defer respRO.Body.Close()
	var wrapRO struct {
		Result struct {
			Tools []struct {
				Name string `json:"name"`
			} `json:"tools"`
		} `json:"result"`
	}
	if err := json.NewDecoder(respRO.Body).Decode(&wrapRO); err != nil {
		t.Fatal(err)
	}
	if len(wrapRO.Result.Tools) != 2 {
		t.Fatalf("readonly key: want 2 tools, got %d", len(wrapRO.Result.Tools))
	}

	reqEX, err := http.NewRequest(http.MethodPost, srv.URL, bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	reqEX.Header.Set("Content-Type", "application/json")
	reqEX.Header.Set("Authorization", "Bearer ex")
	respEX, err := http.DefaultClient.Do(reqEX)
	if err != nil {
		t.Fatal(err)
	}
	defer respEX.Body.Close()
	var wrapEX struct {
		Result struct {
			Tools []struct {
				Name string `json:"name"`
			} `json:"tools"`
		} `json:"result"`
	}
	if err := json.NewDecoder(respEX.Body).Decode(&wrapEX); err != nil {
		t.Fatal(err)
	}
	if len(wrapEX.Result.Tools) != 6 {
		t.Fatalf("exec key: want 6 tools, got %d", len(wrapEX.Result.Tools))
	}
}
