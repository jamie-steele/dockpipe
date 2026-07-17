package mcpbridge

import "context"

type ctxKeyMcpTier struct{}

// WithMCPTier attaches an effective MCP tier for this request (HTTP per-key mode).
// Stdio callers use context.Background() and EffectiveMCPTier() applies.
func WithMCPTier(ctx context.Context, t MCPTier) context.Context {
	return context.WithValue(ctx, ctxKeyMcpTier{}, t)
}

// MCPTierFromContext returns the tier set by WithMCPTier, if any.
func MCPTierFromContext(ctx context.Context) (MCPTier, bool) {
	t, ok := ctx.Value(ctxKeyMcpTier{}).(MCPTier)
	return t, ok
}

func effectiveTierLabel(ctx context.Context) string {
	if t, ok := MCPTierFromContext(ctx); ok {
		return t.String()
	}
	return EffectiveMCPTier().String()
}
