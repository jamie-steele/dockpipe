package mcpbridge

import "encoding/json"

// mcpToolMeta is one row for tools/list (filtered by ToolAllowed(ctx, name)).
type mcpToolMeta struct {
	Name        string
	Description string
	InputSchema json.RawMessage
}

func mcpToolCatalog() []mcpToolMeta {
	return []mcpToolMeta{
		{
			Name:        "dockpipe.version",
			Description: "Run dockpipe --version. Tier: readonly+.",
			InputSchema: json.RawMessage(`{"type":"object","properties":{}}`),
		},
		{
			Name:        "capabilities.workflows",
			Description: "List workflow names (DOCKPIPE_REPO_ROOT / bundled cache). Tier: readonly+.",
			InputSchema: json.RawMessage(`{"type":"object","properties":{}}`),
		},
		{
			Name:        "dockpipe.validate_workflow",
			Description: "Validate workflow YAML (dockpipe workflow validate). Tier: validate+.",
			InputSchema: json.RawMessage(`{"type":"object","properties":{"path":{"type":"string","description":"defaults to dockpipe.yml"}},"additionalProperties":false}`),
		},
		{
			Name:        "dorkpipe.validate_spec",
			Description: "Validate a DorkPipe DAG spec (dorkpipe validate -f). Tier: validate+.",
			InputSchema: json.RawMessage(`{"type":"object","properties":{"spec_path":{"type":"string"}},"required":["spec_path"],"additionalProperties":false}`),
		},
		{
			Name:        "dockpipe.run",
			Description: "Run dockpipe with --workflow, --workdir, argv after --. Tier: exec only.",
			InputSchema: json.RawMessage(`{"type":"object","properties":{"workflow":{"type":"string"},"workdir":{"type":"string"},"argv":{"type":"array","items":{"type":"string"}}},"required":["workflow"],"additionalProperties":false}`),
		},
		{
			Name:        "dorkpipe.run_spec",
			Description: "Run dorkpipe run -f <spec>. Tier: exec only.",
			InputSchema: json.RawMessage(`{"type":"object","properties":{"spec_path":{"type":"string"},"workdir":{"type":"string"}},"required":["spec_path"],"additionalProperties":false}`),
		},
	}
}
