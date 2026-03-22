package engine

import (
	"encoding/json"
	"strings"
)

func parseBranchWinner(stdout string) string {
	s := strings.TrimSpace(stdout)
	var m struct {
		Winner string `json:"winner"`
	}
	if json.Unmarshal([]byte(s), &m) == nil && strings.TrimSpace(m.Winner) != "" {
		return strings.TrimSpace(m.Winner)
	}
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if json.Unmarshal([]byte(line), &m) == nil && strings.TrimSpace(m.Winner) != "" {
			return strings.TrimSpace(m.Winner)
		}
	}
	return ""
}
