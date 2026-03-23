package domain

import "strings"

// MergeIfUnset adds keys from src into dst only when dst does not already have the key.
func MergeIfUnset(dst map[string]string, src map[string]string) {
	for k, v := range src {
		if _, ok := dst[k]; !ok {
			dst[k] = v
		}
	}
}

// EnvSliceToMap parses KEY=VAL from a slice (e.g. --env).
func EnvSliceToMap(lines []string) map[string]string {
	m := make(map[string]string)
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if ok {
			m[strings.TrimSpace(k)] = strings.TrimSpace(v)
		}
	}
	return m
}

// EnvironToMap converts KEY=VAL lines (e.g. os.Environ()) to a map.
func EnvironToMap(environ []string) map[string]string {
	m := make(map[string]string)
	for _, e := range environ {
		k, v, ok := strings.Cut(e, "=")
		if ok {
			m[k] = v
		}
	}
	return m
}

// EnvMapToSlice converts a map to KEY=VAL lines for subprocess env (order undefined).
func EnvMapToSlice(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k, v := range m {
		out = append(out, k+"="+v)
	}
	return out
}

// BranchPrefixForTemplate maps template/resolver flavor to git branch prefix.
func BranchPrefixForTemplate(t string) string {
	switch t {
	case "claude", "agent-dev":
		return "claude"
	case "codex":
		return "codex"
	default:
		return "dockpipe"
	}
}
