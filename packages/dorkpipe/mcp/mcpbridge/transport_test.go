package mcpbridge

import (
	"bufio"
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestReadWriteMessageRoundTrip(t *testing.T) {
	t.Parallel()
	payload := map[string]any{"jsonrpc": "2.0", "id": 1, "method": "ping"}
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := WriteMessage(&buf, body); err != nil {
		t.Fatal(err)
	}
	got, err := ReadMessage(bufio.NewReader(&buf))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(body) {
		t.Fatalf("got %q want %q", got, body)
	}
}

func TestReadMessageHeaders(t *testing.T) {
	t.Parallel()
	raw := "Content-Length: 11\r\n\r\n{\"hello\":1}"
	got, err := ReadMessage(bufio.NewReader(strings.NewReader(raw)))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != `{"hello":1}` {
		t.Fatalf("got %q", got)
	}
}
