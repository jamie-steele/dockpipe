package mcpbridge

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
)

const maxHTTPKeyTiersFileBytes = 1 << 20 // 1 MiB

// keyTierEntry is one API key and its tier (HTTP per-key mode).
type keyTierEntry struct {
	key  string
	tier MCPTier
}

type httpKeyTierFileRow struct {
	Key  string `json:"key"`
	Tier string `json:"tier"`
}

// loadHTTPKeyTierFile reads [{"key":"...","tier":"readonly|validate|exec"}, ...].
// Keys must be unique. File should be chmod 600 on disk.
func loadHTTPKeyTierFile(path string) ([]keyTierEntry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, maxHTTPKeyTiersFileBytes+1))
	if err != nil {
		return nil, err
	}
	if len(b) > maxHTTPKeyTiersFileBytes {
		return nil, fmt.Errorf("mcpbridge: key tiers file too large (max %d bytes)", maxHTTPKeyTiersFileBytes)
	}
	var raw []httpKeyTierFileRow
	if err := json.Unmarshal(b, &raw); err != nil {
		return nil, fmt.Errorf("mcpbridge: key tiers JSON: %w", err)
	}
	if len(raw) == 0 {
		return nil, fmt.Errorf("mcpbridge: key tiers file has no entries")
	}
	seen := make(map[string]struct{})
	out := make([]keyTierEntry, 0, len(raw))
	for i, row := range raw {
		k := strings.TrimSpace(row.Key)
		if k == "" {
			return nil, fmt.Errorf("mcpbridge: key tiers entry %d: empty key", i)
		}
		if _, dup := seen[k]; dup {
			return nil, fmt.Errorf("mcpbridge: duplicate key in key tiers file")
		}
		seen[k] = struct{}{}
		t, err := parseTierName(strings.TrimSpace(row.Tier))
		if err != nil {
			return nil, fmt.Errorf("mcpbridge: key tiers entry %d: tier %q: %w", i, row.Tier, err)
		}
		out = append(out, keyTierEntry{key: k, tier: t})
	}
	return out, nil
}

func lookupKeyTier(entries []keyTierEntry, got string) (MCPTier, bool) {
	for _, e := range entries {
		if ConstantTimeEqualString(got, e.key) {
			return e.tier, true
		}
	}
	return TierValidate, false
}
