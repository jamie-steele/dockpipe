package mcpbridge

import (
	"context"
	"encoding/json"
	"io"
	"testing"
)

func TestEffectiveMCPTierPrecedence(t *testing.T) {
	t.Setenv("DOCKPIPE_MCP_TIER", "")
	t.Setenv("DOCKPIPE_MCP_ALLOW_EXEC", "")
	if g, w := EffectiveMCPTier(), TierValidate; g != w {
		t.Fatalf("default: got %v want %v", g, w)
	}

	t.Setenv("DOCKPIPE_MCP_ALLOW_EXEC", "1")
	if g, w := EffectiveMCPTier(), TierExec; g != w {
		t.Fatalf("allow_exec: got %v want %v", g, w)
	}

	t.Setenv("DOCKPIPE_MCP_TIER", "readonly")
	t.Setenv("DOCKPIPE_MCP_ALLOW_EXEC", "1")
	if g, w := EffectiveMCPTier(), TierReadonly; g != w {
		t.Fatalf("tier overrides allow_exec: got %v want %v", g, w)
	}
}

func TestToolAllowedByTier(t *testing.T) {
	t.Setenv("DOCKPIPE_MCP_ALLOWED_TOOLS", "")
	t.Setenv("DOCKPIPE_MCP_ALLOW_EXEC", "")
	t.Setenv("DOCKPIPE_MCP_TIER", "readonly")
	ctx := context.Background()
	if !ToolAllowed(ctx, "dockpipe.version") || !ToolAllowed(ctx, "capabilities.workflows") {
		t.Fatal("readonly should allow version + workflows")
	}
	if ToolAllowed(ctx, "dockpipe.validate_workflow") || ToolAllowed(ctx, "dockpipe.run") {
		t.Fatal("readonly should deny validate and exec tools")
	}

	t.Setenv("DOCKPIPE_MCP_TIER", "validate")
	if !ToolAllowed(ctx, "dockpipe.validate_workflow") {
		t.Fatal("validate tier should allow validate_workflow")
	}
	if ToolAllowed(ctx, "dockpipe.run") {
		t.Fatal("validate tier should deny run")
	}

	t.Setenv("DOCKPIPE_MCP_TIER", "exec")
	if !ToolAllowed(ctx, "dockpipe.run") || !ToolAllowed(ctx, "dorkpipe.run_spec") {
		t.Fatal("exec tier should allow run tools")
	}
}

func TestToolAllowedExplicitList(t *testing.T) {
	t.Setenv("DOCKPIPE_MCP_TIER", "validate")
	t.Setenv("DOCKPIPE_MCP_ALLOWED_TOOLS", "dockpipe.version,capabilities.workflows")
	ctx := context.Background()
	if !ToolAllowed(ctx, "dockpipe.version") {
		t.Fatal("allowlist should keep version")
	}
	if ToolAllowed(ctx, "dockpipe.validate_workflow") {
		t.Fatal("allowlist should drop validate_workflow when not listed")
	}
}

func TestToolsListFilteredByTier(t *testing.T) {
	t.Setenv("DOCKPIPE_MCP_TIER", "readonly")
	t.Setenv("DOCKPIPE_MCP_ALLOWED_TOOLS", "")
	t.Setenv("DOCKPIPE_MCP_ALLOW_EXEC", "")
	s := &Server{Version: "t"}
	raw := []byte(`{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}`)
	resp := s.handleMessage(context.Background(), raw, io.Discard)
	if resp == nil || resp.Error != nil {
		t.Fatalf("resp=%+v", resp)
	}
	var out struct {
		Tools []struct {
			Name string `json:"name"`
		} `json:"tools"`
	}
	if err := json.Unmarshal(resp.Result, &out); err != nil {
		t.Fatal(err)
	}
	want := expectedToolNamesForTier(TierReadonly)
	got := make([]string, 0, len(out.Tools))
	for _, tool := range out.Tools {
		got = append(got, tool.Name)
	}
	if !equalStringSlices(got, want) {
		t.Fatalf("readonly tier tools mismatch: got %v want %v", got, want)
	}
}

func TestToolAllowedContextOverridesEnv(t *testing.T) {
	t.Setenv("DOCKPIPE_MCP_TIER", "exec")
	t.Setenv("DOCKPIPE_MCP_ALLOWED_TOOLS", "")
	ctx := WithMCPTier(context.Background(), TierReadonly)
	if ToolAllowed(ctx, "dockpipe.run") {
		t.Fatal("context tier readonly should deny run even when env is exec")
	}
}
