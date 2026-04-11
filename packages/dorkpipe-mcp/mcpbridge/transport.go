package mcpbridge

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// ReadMessage reads one MCP message (Content-Length framed body) from r.
func ReadMessage(r *bufio.Reader) ([]byte, error) {
	var n int
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return nil, err
		}
		line = strings.TrimSuffix(line, "\r")
		if strings.TrimSpace(line) == "" {
			break
		}
		k, v, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(k), "Content-Length") {
			v = strings.TrimSpace(v)
			var err2 error
			n, err2 = strconv.Atoi(v)
			if err2 != nil {
				return nil, fmt.Errorf("mcpbridge: bad Content-Length %q", v)
			}
		}
	}
	if n <= 0 {
		return nil, fmt.Errorf("mcpbridge: missing or invalid Content-Length")
	}
	buf := make([]byte, n)
	if _, err := io.ReadFull(r, buf); err != nil {
		return nil, err
	}
	return buf, nil
}

// WriteMessage writes one MCP message with Content-Length framing.
func WriteMessage(w io.Writer, body []byte) error {
	if _, err := fmt.Fprintf(w, "Content-Length: %d\r\n\r\n", len(body)); err != nil {
		return err
	}
	if _, err := w.Write(body); err != nil {
		return err
	}
	type syncer interface{ Sync() error }
	if f, ok := w.(syncer); ok {
		_ = f.Sync()
	}
	return nil
}
