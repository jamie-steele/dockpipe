package infrastructure

import (
	"bufio"
	"bytes"
	"io"
	"strings"
)

// ParseEnvBytes parses KEY=VAL lines (dotenv-style) from UTF-8 bytes. Skips comments and blanks.
func ParseEnvBytes(data []byte) (map[string]string, error) {
	return parseEnvReader(bytes.NewReader(data))
}

func parseEnvReader(r io.Reader) (map[string]string, error) {
	sc := bufio.NewScanner(r)
	out := make(map[string]string)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if !strings.Contains(line, "=") {
			continue
		}
		k, v, _ := strings.Cut(line, "=")
		k = strings.TrimSpace(k)
		v = strings.TrimSpace(v)
		if k == "" || strings.HasPrefix(k, "#") {
			continue
		}
		if len(v) >= 2 {
			if (v[0] == '"' && v[len(v)-1] == '"') || (v[0] == '\'' && v[len(v)-1] == '\'') {
				v = v[1 : len(v)-1]
			}
		}
		out[k] = v
	}
	return out, sc.Err()
}
