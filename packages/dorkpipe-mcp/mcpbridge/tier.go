package mcpbridge

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
)

// MCPTier is the coarse IAM level for which MCP tools may run in this process.
// Higher tiers include all tools from lower tiers.
type MCPTier int

const (
	TierReadonly MCPTier = iota // dockpipe.version, capabilities.workflows
	TierValidate                // + validate_workflow, validate_spec
	TierExec                    // + dockpipe.run, dorkpipe.run_spec
)

func (t MCPTier) String() string {
	switch t {
	case TierReadonly:
		return "readonly"
	case TierValidate:
		return "validate"
	case TierExec:
		return "exec"
	default:
		return fmt.Sprintf("MCPTier(%d)", int(t))
	}
}

var (
	tierParseWarnOnce sync.Once
	allowlistWarnOnce sync.Once
)

// EffectiveMCPTier reads DOCKPIPE_MCP_TIER and DOCKPIPE_MCP_ALLOW_EXEC (legacy).
//
// Precedence:
//  1. DOCKPIPE_MCP_TIER (readonly | validate | exec) when non-empty — authoritative.
//  2. Else DOCKPIPE_MCP_ALLOW_EXEC=1 → exec (backward compatible).
//  3. Else → validate (same default as before: validate tools on, exec off).
func EffectiveMCPTier() MCPTier {
	if v := strings.TrimSpace(os.Getenv("DOCKPIPE_MCP_TIER")); v != "" {
		t, err := parseTierName(v)
		if err != nil {
			tierParseWarnOnce.Do(func() {
				fmt.Fprintf(os.Stderr, "mcpd: invalid DOCKPIPE_MCP_TIER=%q (%v); using validate\n", v, err)
			})
			return TierValidate
		}
		return t
	}
	if strings.TrimSpace(os.Getenv("DOCKPIPE_MCP_ALLOW_EXEC")) == "1" {
		return TierExec
	}
	return TierValidate
}

func parseTierName(s string) (MCPTier, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "readonly", "read-only", "0":
		return TierReadonly, nil
	case "validate", "validation", "1":
		return TierValidate, nil
	case "exec", "execute", "2":
		return TierExec, nil
	default:
		return TierValidate, fmt.Errorf("unknown tier name")
	}
}

// toolMinTier is the minimum tier required to invoke a tool.
var toolMinTier = map[string]MCPTier{
	"dockpipe.version":           TierReadonly,
	"capabilities.workflows":     TierReadonly,
	"repo.list_files":            TierReadonly,
	"repo.read_file":             TierReadonly,
	"repo.search_text":           TierReadonly,
	"dockpipe.validate_workflow": TierValidate,
	"dorkpipe.validate_spec":     TierValidate,
	"dockpipe.run":               TierExec,
	"dorkpipe.run_spec":          TierExec,
}

func minTierForTool(name string) (MCPTier, bool) {
	t, ok := toolMinTier[name]
	return t, ok
}

// allowedToolsExplicit parses DOCKPIPE_MCP_ALLOWED_TOOLS (comma-separated).
// If empty, the second return is false and the caller should use tier only.
func allowedToolsExplicit() (map[string]struct{}, bool, error) {
	raw := strings.TrimSpace(os.Getenv("DOCKPIPE_MCP_ALLOWED_TOOLS"))
	if raw == "" {
		return nil, false, nil
	}
	out := make(map[string]struct{})
	var unknown []string
	for _, p := range strings.Split(raw, ",") {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if _, ok := toolMinTier[p]; !ok {
			unknown = append(unknown, p)
			continue
		}
		out[p] = struct{}{}
	}
	if len(unknown) > 0 {
		allowlistWarnOnce.Do(func() {
			fmt.Fprintf(os.Stderr, "mcpd: ignoring unknown tool names in DOCKPIPE_MCP_ALLOWED_TOOLS: %s\n", strings.Join(unknown, ", "))
		})
	}
	if len(out) == 0 {
		return nil, true, fmt.Errorf("DOCKPIPE_MCP_ALLOWED_TOOLS produced an empty allowlist")
	}
	return out, true, nil
}

// ToolAllowed reports whether the named tool may run for this request.
// If ctx carries a tier from HTTP per-key auth, that tier is used; otherwise EffectiveMCPTier().
func ToolAllowed(ctx context.Context, name string) bool {
	mt, ok := minTierForTool(name)
	if !ok {
		return false
	}
	tier := EffectiveMCPTier()
	if t, ok := MCPTierFromContext(ctx); ok {
		tier = t
	}
	if tier < mt {
		return false
	}
	explicit, hasExplicit, err := allowedToolsExplicit()
	if err != nil {
		// Misconfigured allowlist: deny all tools (safe default).
		return false
	}
	if !hasExplicit {
		return true
	}
	_, in := explicit[name]
	return in
}
