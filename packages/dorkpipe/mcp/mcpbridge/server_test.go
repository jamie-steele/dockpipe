package mcpbridge

import (
	"bufio"
	"bytes"
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

func TestServerInitialize_echoesClientProtocolVersion(t *testing.T) {
	t.Parallel()
	s := &Server{Version: "1"}
	raw := []byte(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"t","version":"1"}}}`)
	resp := s.handleMessage(context.Background(), raw, io.Discard)
	if resp == nil || resp.Error != nil {
		t.Fatalf("resp=%+v", resp)
	}
	var out struct {
		ProtocolVersion string `json:"protocolVersion"`
	}
	if err := json.Unmarshal(resp.Result, &out); err != nil {
		t.Fatal(err)
	}
	if out.ProtocolVersion != "2025-11-25" {
		t.Fatalf("protocolVersion %q", out.ProtocolVersion)
	}
}

func TestServeStdio_repliesWithBatchWhenRequestWasSingleElementBatch(t *testing.T) {
	t.Parallel()
	s := &Server{Version: "9.9.9"}
	payload := `[{"jsonrpc":"2.0","id":99,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"c","version":"1"}}}]`
	var inBuf bytes.Buffer
	if err := WriteMessage(&inBuf, []byte(payload)); err != nil {
		t.Fatal(err)
	}
	var outBuf bytes.Buffer
	if err := s.ServeStdio(&inBuf, &outBuf, io.Discard); err != nil {
		t.Fatal(err)
	}
	br := bytes.NewReader(outBuf.Bytes())
	got, err := ReadMessage(bufio.NewReader(br))
	if err != nil {
		t.Fatalf("read response: %v", err)
	}
	if got[0] != '[' {
		t.Fatalf("expected JSON array response for batch request, got: %s", got)
	}
	var arr []json.RawMessage
	if err := json.Unmarshal(got, &arr); err != nil || len(arr) != 1 {
		t.Fatalf("expected one-element batch, got %v err=%v", arr, err)
	}
	var inner struct {
		Result struct {
			ProtocolVersion string `json:"protocolVersion"`
		} `json:"result"`
	}
	if err := json.Unmarshal(arr[0], &inner); err != nil {
		t.Fatal(err)
	}
	if inner.Result.ProtocolVersion != "2025-11-25" {
		t.Fatalf("protocolVersion %q", inner.Result.ProtocolVersion)
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
