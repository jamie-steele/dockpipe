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
			InputSchema: json.RawMessage(`{"type":"object","properties":{"path":{"type":"string","description":"optional path to config.yml, repo-relative (e.g. workflows/ci/test/config.yml); omit only for a flat single-workflow project"}},"additionalProperties":false}`),
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
		{
			Name:        "dorkpipe.request",
			Description: "Run dorkpipe request --execute through the MCP control plane. Tier: exec only.",
			InputSchema: json.RawMessage(`{"type":"object","properties":{"workdir":{"type":"string"},"message":{"type":"string"},"mode":{"type":"string"},"session_id":{"type":"string"},"provider_preset":{"type":"string"},"model_provider":{"type":"string"},"model":{"type":"string"},"active_file":{"type":"string"},"open_files":{"type":"array","items":{"type":"string"}},"selection_text":{"type":"string"},"attachment_files":{"type":"array","items":{"type":"string"}}},"required":["message"],"additionalProperties":false}`),
		},
		{
			Name:        "dorkpipe.provider_pool_catalog",
			Description: "Read the shared DorkPipe provider-pool catalog plus current provider states. Tier: readonly+.",
			InputSchema: json.RawMessage(`{"type":"object","properties":{"workdir":{"type":"string"}},"additionalProperties":false}`),
		},
		{
			Name:        "dorkpipe.provider_pool_status",
			Description: "Read current DorkPipe provider-pool status, optionally filtered to one provider. Tier: readonly+.",
			InputSchema: json.RawMessage(`{"type":"object","properties":{"workdir":{"type":"string"},"provider":{"type":"string","enum":["ollama","codex","claude"]}},"additionalProperties":false}`),
		},
		{
			Name:        "dorkpipe.provider_pool_chat",
			Description: "Route a direct prompt through the shared DorkPipe provider-pool contract. Tier: exec only.",
			InputSchema: json.RawMessage(`{"type":"object","properties":{"workdir":{"type":"string"},"message":{"type":"string"},"provider":{"type":"string","enum":["ollama","codex","claude"]},"model":{"type":"string"},"session_id":{"type":"string"},"active_file":{"type":"string"},"open_files":{"type":"array","items":{"type":"string"}},"selection_text":{"type":"string"}},"required":["message"],"additionalProperties":false}`),
		},
		{
			Name:        "dorkpipe.host_codex_chat",
			Description: "Host bridge for direct Codex chat. Runs codex exec with workspace sandboxing and the host Codex model config by default. Tier: exec only.",
			InputSchema: json.RawMessage(`{"type":"object","properties":{"workdir":{"type":"string"},"message":{"type":"string"},"model":{"type":"string"},"session_id":{"type":"string"},"active_file":{"type":"string"},"open_files":{"type":"array","items":{"type":"string"}},"selection_text":{"type":"string"}},"required":["message"],"additionalProperties":false}`),
		},
		{
			Name:        "dorkpipe.host_claude_chat",
			Description: "Host bridge for guarded Claude chat. Routes through DockPipe's Claude workflow boundary instead of raw host Claude. Tier: exec only.",
			InputSchema: json.RawMessage(`{"type":"object","properties":{"workdir":{"type":"string"},"message":{"type":"string"},"model":{"type":"string"},"session_id":{"type":"string"},"active_file":{"type":"string"},"open_files":{"type":"array","items":{"type":"string"}},"selection_text":{"type":"string"}},"required":["message"],"additionalProperties":false}`),
		},
		{
			Name:        "dorkpipe.host_claude_auth",
			Description: "Backward-compatible alias for dorkpipe.provider_auth_repair with provider=claude. Tier: exec only.",
			InputSchema: json.RawMessage(`{"type":"object","properties":{"workdir":{"type":"string"}},"additionalProperties":false}`),
		},
		{
			Name:        "dorkpipe.provider_auth_status",
			Description: "Check host provider auth state without launching a worker. Tier: readonly+.",
			InputSchema: json.RawMessage(`{"type":"object","properties":{"provider":{"type":"string","enum":["codex","claude"]},"workdir":{"type":"string"}},"required":["provider"],"additionalProperties":false}`),
		},
		{
			Name:        "dorkpipe.provider_auth_repair",
			Description: "Launch the provider's host authentication flow directly, then recheck provider status. Tier: exec only.",
			InputSchema: json.RawMessage(`{"type":"object","properties":{"provider":{"type":"string","enum":["claude"]},"workdir":{"type":"string"}},"required":["provider"],"additionalProperties":false}`),
		},
		{
			Name:        "dorkpipe.apply_edit",
			Description: "Run dorkpipe apply-edit for a prepared artifact directory. Tier: exec only.",
			InputSchema: json.RawMessage(`{"type":"object","properties":{"workdir":{"type":"string"},"artifact_dir":{"type":"string"}},"required":["artifact_dir"],"additionalProperties":false}`),
		},
	}
}
