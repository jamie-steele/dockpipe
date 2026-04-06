package mcpbridge

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
)

// Server holds MCP server state (version string for initialize).
type Server struct {
	Version string
}

// NewServer builds a server; version falls back to DOCKPIPE_MCP_SERVER_VERSION or "0.0.0-dev".
func NewServer(version string) *Server {
	v := strings.TrimSpace(version)
	if v == "" {
		v = strings.TrimSpace(os.Getenv("DOCKPIPE_MCP_SERVER_VERSION"))
	}
	if v == "" {
		v = "0.0.0-dev"
	}
	return &Server{Version: v}
}

// ServeStdio runs the MCP JSON-RPC loop over Content-Length–framed messages.
func (s *Server) ServeStdio(in io.Reader, out io.Writer, log io.Writer) error {
	br := bufio.NewReader(in)
	for {
		raw, err := ReadMessage(br)
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
		reqBody := raw
		replyAsBatch := false
		if trim := bytes.TrimSpace(raw); len(trim) > 0 && trim[0] == '[' {
			var arr []json.RawMessage
			if err := json.Unmarshal(trim, &arr); err == nil && len(arr) == 1 {
				reqBody = []byte(arr[0])
				replyAsBatch = true
			}
		}
		resp := s.handleMessage(context.Background(), reqBody, log)
		if resp == nil {
			continue
		}
		body, err := json.Marshal(resp)
		if err != nil {
			fmt.Fprintf(log, "mcpbridge: marshal response: %v\n", err)
			continue
		}
		if replyAsBatch {
			wrapped, err := json.Marshal([]json.RawMessage{json.RawMessage(body)})
			if err != nil {
				fmt.Fprintf(log, "mcpbridge: marshal batch response: %v\n", err)
				continue
			}
			body = wrapped
		}
		if err := WriteMessage(out, body); err != nil {
			return err
		}
	}
}

func (s *Server) handleMessage(ctx context.Context, raw []byte, log io.Writer) *rpcResponse {
	var req rpcRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		return errResponse(nil, -32700, "parse error")
	}
	if req.JSONRPC != "2.0" {
		return errResponse(req.ID, -32600, "invalid request")
	}
	if req.ID == nil {
		if strings.HasPrefix(req.Method, "notifications/") {
			return nil
		}
		return nil
	}
	switch req.Method {
	case "initialize":
		return s.handleInitialize(&req)
	case "tools/list":
		return s.handleToolsList(ctx, &req)
	case "tools/call":
		return s.handleToolsCall(ctx, &req, log)
	case "ping":
		return okResponse(req.ID, json.RawMessage(`{}`))
	default:
		return errResponse(req.ID, -32601, fmt.Sprintf("method not found: %s", req.Method))
	}
}

func (s *Server) handleInitialize(req *rpcRequest) *rpcResponse {
	type result struct {
		ProtocolVersion string `json:"protocolVersion"`
		Capabilities    struct {
			Tools struct{} `json:"tools"`
		} `json:"capabilities"`
		ServerInfo struct {
			Name    string `json:"name"`
			Version string `json:"version"`
		} `json:"serverInfo"`
	}
	type initParams struct {
		ProtocolVersion string `json:"protocolVersion"`
	}
	var p initParams
	_ = json.Unmarshal(req.Params, &p)
	ver := strings.TrimSpace(p.ProtocolVersion)
	if ver == "" {
		ver = "2024-11-05"
	}
	// Cursor and other hosts send newer MCP protocol lines; echo the client's version so
	// the handshake completes (we only implement tools over stdio; wire format matches).
	var r result
	r.ProtocolVersion = ver
	r.Capabilities.Tools = struct{}{}
	r.ServerInfo.Name = "dorkpipe-mcp"
	r.ServerInfo.Version = s.Version
	b, err := json.Marshal(r)
	if err != nil {
		return errResponse(req.ID, -32603, "internal error")
	}
	return okResponse(req.ID, b)
}

func (s *Server) handleToolsList(ctx context.Context, req *rpcRequest) *rpcResponse {
	type tool struct {
		Name        string          `json:"name"`
		Description string          `json:"description"`
		InputSchema json.RawMessage `json:"inputSchema"`
	}
	var tools []tool
	for _, m := range mcpToolCatalog() {
		if !ToolAllowed(ctx, m.Name) {
			continue
		}
		tools = append(tools, tool{Name: m.Name, Description: m.Description, InputSchema: m.InputSchema})
	}
	out := struct {
		Tools []tool `json:"tools"`
	}{Tools: tools}
	b, err := json.Marshal(out)
	if err != nil {
		return errResponse(req.ID, -32603, "internal error")
	}
	return okResponse(req.ID, b)
}

func (s *Server) handleToolsCall(ctx context.Context, req *rpcRequest, log io.Writer) *rpcResponse {
	var params struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return errResponse(req.ID, -32602, "invalid params")
	}
	raw, isErr, err := s.dispatchTool(ctx, params.Name, params.Arguments)
	if err != nil {
		fmt.Fprintf(log, "mcpbridge tool %s: %v\n", params.Name, err)
		return errResponse(req.ID, -32000, err.Error())
	}
	tr := toolResultJSON(string(raw), isErr)
	return okResponse(req.ID, json.RawMessage(tr))
}

func (s *Server) dispatchTool(ctx context.Context, name string, args json.RawMessage) ([]byte, bool, error) {
	if !ToolAllowed(ctx, name) {
		return nil, true, fmt.Errorf("mcp: tool %q not allowed (effective tier %s; see DOCKPIPE_MCP_TIER or HTTP key tiers file)", name, effectiveTierLabel(ctx))
	}
	switch name {
	case "dockpipe.version":
		out, stderr, code, err := runDockpipe(ctx, []string{"--version"})
		if err != nil {
			return nil, true, err
		}
		text := strings.TrimSpace(out + stderr)
		if code != 0 {
			return []byte(text), true, nil
		}
		return []byte(text), false, nil

	case "capabilities.workflows":
		names, err := listWorkflowNames()
		if err != nil {
			return nil, true, err
		}
		b, err := json.MarshalIndent(names, "", "  ")
		if err != nil {
			return nil, true, err
		}
		return b, false, nil

	case "dockpipe.validate_workflow":
		var in struct {
			Path string `json:"path"`
		}
		_ = json.Unmarshal(args, &in)
		path := strings.TrimSpace(in.Path)
		var cmdArgs []string
		if path == "" {
			cmdArgs = []string{"workflow", "validate"}
		} else {
			absPath, err := ResolvePathUnderRepoRoot(path)
			if err != nil {
				return nil, true, err
			}
			cmdArgs = []string{"workflow", "validate", absPath}
		}
		_, stderr, code, err := runDockpipe(ctx, cmdArgs)
		msg := strings.TrimSpace(stderr)
		if err != nil {
			return nil, true, err
		}
		if code != 0 {
			return []byte(msg), true, nil
		}
		return []byte(msg), false, nil

	case "dorkpipe.validate_spec":
		var in struct {
			SpecPath string `json:"spec_path"`
		}
		if err := json.Unmarshal(args, &in); err != nil {
			return nil, true, err
		}
		sp := strings.TrimSpace(in.SpecPath)
		if sp == "" {
			return nil, true, fmt.Errorf("spec_path required")
		}
		absSpec, err := ResolvePathUnderRepoRoot(sp)
		if err != nil {
			return nil, true, err
		}
		_, stderr, code, err := runDorkpipe(ctx, []string{"validate", "-f", absSpec})
		msg := strings.TrimSpace(stderr)
		if err != nil {
			return nil, true, err
		}
		if code != 0 {
			return []byte(msg), true, nil
		}
		return []byte(msg), false, nil

	case "dockpipe.run":
		var in struct {
			Workflow string   `json:"workflow"`
			Workdir  string   `json:"workdir"`
			Argv     []string `json:"argv"`
		}
		if err := json.Unmarshal(args, &in); err != nil {
			return nil, true, err
		}
		wf := strings.TrimSpace(in.Workflow)
		if wf == "" {
			return nil, true, fmt.Errorf("workflow required")
		}
		wd, err := resolveExecWorkdir(in.Workdir)
		if err != nil {
			return nil, true, err
		}
		cmdArgs := []string{"--workflow", wf, "--workdir", wd, "--"}
		cmdArgs = append(cmdArgs, in.Argv...)
		stdout, stderr, code, err := runDockpipe(ctx, cmdArgs)
		if err != nil {
			return nil, true, err
		}
		summary := fmt.Sprintf("exit_code=%d\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
		return []byte(summary), code != 0, nil

	case "dorkpipe.run_spec":
		var in struct {
			SpecPath string `json:"spec_path"`
			Workdir  string `json:"workdir"`
		}
		if err := json.Unmarshal(args, &in); err != nil {
			return nil, true, err
		}
		sp := strings.TrimSpace(in.SpecPath)
		if sp == "" {
			return nil, true, fmt.Errorf("spec_path required")
		}
		absSpec, err := ResolvePathUnderRepoRoot(sp)
		if err != nil {
			return nil, true, err
		}
		dargs := []string{"run", "-f", absSpec}
		if restrictExecWorkdirToRepo() {
			awd, err := resolveExecWorkdir(in.Workdir)
			if err != nil {
				return nil, true, err
			}
			dargs = append(dargs, "--workdir", awd)
		} else if wd := strings.TrimSpace(in.Workdir); wd != "" {
			awd, err := absWorkdir(wd)
			if err != nil {
				return nil, true, err
			}
			dargs = append(dargs, "--workdir", awd)
		}
		stdout, stderr, code, err := runDorkpipe(ctx, dargs)
		if err != nil {
			return nil, true, err
		}
		summary := fmt.Sprintf("exit_code=%d\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
		return []byte(summary), code != 0, nil

	default:
		return nil, true, fmt.Errorf("unknown tool %q", name)
	}
}
