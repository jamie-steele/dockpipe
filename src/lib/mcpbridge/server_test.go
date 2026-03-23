package mcpbridge

import (
	"context"
	"encoding/json"
	"io"
	"testing"
)

func TestServerInitialize(t *testing.T) {
	t.Parallel()
	s := &Server{Version: "1.2.3"}
	raw := []byte(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"t","version":"1"}}}`)
	resp := s.handleMessage(context.Background(), raw, io.Discard)
	if resp == nil || resp.Error != nil {
		t.Fatalf("resp=%+v", resp)
	}
	var out struct {
		ServerInfo struct {
			Version string `json:"version"`
		} `json:"serverInfo"`
	}
	if err := json.Unmarshal(resp.Result, &out); err != nil {
		t.Fatal(err)
	}
	if out.ServerInfo.Version != "1.2.3" {
		t.Fatalf("version %q", out.ServerInfo.Version)
	}
}

func TestServerNotificationNoResponse(t *testing.T) {
	t.Parallel()
	s := &Server{Version: "1"}
	raw := []byte(`{"jsonrpc":"2.0","method":"notifications/initialized","params":{}}`)
	resp := s.handleMessage(context.Background(), raw, io.Discard)
	if resp != nil {
		t.Fatalf("expected nil for notification, got %+v", resp)
	}
}
