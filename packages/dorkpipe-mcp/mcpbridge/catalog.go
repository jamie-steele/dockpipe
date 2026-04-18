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
			Description: "List workflow names for the current project or bundled cache. Tier: readonly+.",
			InputSchema: json.RawMessage(`{"type":"object","properties":{}}`),
		},
		{
			Name:        "repo.list_files",
			Description: "List repo-relative files, optionally filtered by a path substring. Tier: readonly+.",
			InputSchema: json.RawMessage(`{"type":"object","properties":{"query":{"type":"string"},"limit":{"type":"integer","minimum":1,"maximum":100}},"additionalProperties":false}`),
		},
		{
			Name:        "repo.read_file",
			Description: "Read a UTF-8 text file under repo root. Tier: readonly+.",
			InputSchema: json.RawMessage(`{"type":"object","properties":{"path":{"type":"string"},"max_chars":{"type":"integer","minimum":1,"maximum":20000}},"required":["path"],"additionalProperties":false}`),
		},
		{
			Name:        "repo.search_text",
			Description: "Search UTF-8 text files under repo root and return matching lines. Tier: readonly+.",
			InputSchema: json.RawMessage(`{"type":"object","properties":{"query":{"type":"string"},"limit":{"type":"integer","minimum":1,"maximum":100}},"required":["query"],"additionalProperties":false}`),
		},
		{
			Name:        "dockpipe.validate_workflow",
			Description: "Validate workflow YAML (dockpipe workflow validate). Tier: validate+.",
			InputSchema: json.RawMessage(`{"type":"object","properties":{"path":{"type":"string","description":"optional path to config.yml, repo-relative (e.g. workflows/test/config.yml); omit when exactly one workflows/*/config.yml exists"}},"additionalProperties":false}`),
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
