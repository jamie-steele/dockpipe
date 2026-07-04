package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

type mcpDiscovery struct {
	Connected bool
	URL       string
	APIKey    string
	ToolNames []string
	Version   string
	Workflows []string
}

type mcpRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type boundedMCPContextResult struct {
	Summary    string
	StepsUsed  int
	SearchHits []string
	ReadFiles  []string
	Refined    []string
}

func discoverMCPContext(ctx context.Context) (*mcpDiscovery, error) {
	baseURL := strings.TrimSpace(os.Getenv("MCP_HTTP_URL"))
	if baseURL == "" {
		return nil, nil
	}
	key := strings.TrimSpace(os.Getenv("MCP_HTTP_API_KEY"))
	if key == "" {
		if keyFile := strings.TrimSpace(os.Getenv("MCP_HTTP_API_KEY_FILE")); keyFile != "" {
			if b, err := os.ReadFile(keyFile); err == nil {
				key = strings.TrimSpace(string(b))
			}
		}
	}
	if key == "" {
		return nil, nil
	}

	disc := &mcpDiscovery{
		Connected: true,
		URL:       strings.TrimSpace(baseURL),
		APIKey:    key,
	}

	var toolsResp struct {
		Tools []struct {
			Name string `json:"name"`
		} `json:"tools"`
	}
	if err := mcpCall(ctx, baseURL, key, "tools/list", map[string]any{}, &toolsResp); err != nil {
		return nil, err
	}
	for _, tool := range toolsResp.Tools {
		if strings.TrimSpace(tool.Name) != "" {
			disc.ToolNames = append(disc.ToolNames, tool.Name)
		}
	}

	if hasMCPTool(disc.ToolNames, "dockpipe.version") {
		var versionResp struct {
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		}
		if err := mcpCall(ctx, baseURL, key, "tools/call", map[string]any{
			"name":      "dockpipe.version",
			"arguments": map[string]any{},
		}, &versionResp); err == nil {
			disc.Version = strings.TrimSpace(flattenMCPText(versionResp.Content))
		}
	}

	if hasMCPTool(disc.ToolNames, "capabilities.workflows") {
		var workflowsResp struct {
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		}
		if err := mcpCall(ctx, baseURL, key, "tools/call", map[string]any{
			"name":      "capabilities.workflows",
			"arguments": map[string]any{},
		}, &workflowsResp); err == nil {
			var workflows []string
			if raw := strings.TrimSpace(flattenMCPText(workflowsResp.Content)); raw != "" {
				_ = json.Unmarshal([]byte(raw), &workflows)
			}
			disc.Workflows = uniqueNonEmpty(workflows)
		}
	}

	return disc, nil
}

func mcpSummaryText(disc *mcpDiscovery) string {
	if disc == nil || !disc.Connected {
		return ""
	}
	var parts []string
	parts = append(parts, fmt.Sprintf("MCP bridge: connected to %s", disc.URL))
	if disc.Version != "" {
		parts = append(parts, fmt.Sprintf("dockpipe version: %s", disc.Version))
	}
	if len(disc.ToolNames) > 0 {
		preview := disc.ToolNames
		if len(preview) > 6 {
			preview = preview[:6]
		}
		parts = append(parts, fmt.Sprintf("available MCP tools: %s", strings.Join(preview, ", ")))
	}
	if len(disc.Workflows) > 0 {
		preview := disc.Workflows
		if len(preview) > 8 {
			preview = preview[:8]
		}
		parts = append(parts, fmt.Sprintf("known workflows: %s", strings.Join(preview, ", ")))
	}
	return clampString(strings.Join(parts, "\n"), 1400)
}

func mcpMetadata(disc *mcpDiscovery) map[string]any {
	if disc == nil || !disc.Connected {
		return nil
	}
	md := map[string]any{
		"mcp_connected":  true,
		"mcp_url":        disc.URL,
		"mcp_tool_count": len(disc.ToolNames),
	}
	if disc.Version != "" {
		md["mcp_dockpipe_version"] = disc.Version
	}
	if len(disc.Workflows) > 0 {
		md["mcp_workflow_count"] = len(disc.Workflows)
	}
	return md
}

func mcpSearchMatches(ctx context.Context, disc *mcpDiscovery, query string, limit int) ([]string, error) {
	if disc == nil || !disc.Connected || !hasMCPTool(disc.ToolNames, "repo.search_text") {
		return nil, nil
	}
	var resp struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := mcpCall(ctx, disc.URL, disc.APIKey, "tools/call", map[string]any{
		"name": "repo.search_text",
		"arguments": map[string]any{
			"query": query,
			"limit": limit,
		},
	}, &resp); err != nil {
		return nil, err
	}
	var out []string
	raw := strings.TrimSpace(flattenMCPText(resp.Content))
	if raw == "" {
		return nil, nil
	}
	_ = json.Unmarshal([]byte(raw), &out)
	return uniqueNonEmpty(out), nil
}

func mcpListFiles(ctx context.Context, disc *mcpDiscovery, query string, limit int) ([]string, error) {
	if disc == nil || !disc.Connected || !hasMCPTool(disc.ToolNames, "repo.list_files") {
		return nil, nil
	}
	var resp struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := mcpCall(ctx, disc.URL, disc.APIKey, "tools/call", map[string]any{
		"name": "repo.list_files",
		"arguments": map[string]any{
			"query": query,
			"limit": limit,
		},
	}, &resp); err != nil {
		return nil, err
	}
	var out []string
	raw := strings.TrimSpace(flattenMCPText(resp.Content))
	if raw == "" {
		return nil, nil
	}
	_ = json.Unmarshal([]byte(raw), &out)
	return uniqueNonEmpty(out), nil
}

func mcpReadFileText(ctx context.Context, disc *mcpDiscovery, relPath string, maxChars int) (string, error) {
	if disc == nil || !disc.Connected || !hasMCPTool(disc.ToolNames, "repo.read_file") {
		return "", nil
	}
	var resp struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := mcpCall(ctx, disc.URL, disc.APIKey, "tools/call", map[string]any{
		"name": "repo.read_file",
		"arguments": map[string]any{
			"path":      relPath,
			"max_chars": maxChars,
		},
	}, &resp); err != nil {
		return "", err
	}
	return strings.TrimSpace(flattenMCPText(resp.Content)), nil
}

func runBoundedMCPContextLoop(ctx context.Context, req routeRequest, disc *mcpDiscovery) *boundedMCPContextResult {
	if disc == nil || !disc.Connected {
		return nil
	}
	const maxSteps = 5
	const maxReads = 3
	result := &boundedMCPContextResult{}
	var sections []string
	seenFiles := map[string]struct{}{}
	var chosenFiles []string

	if strings.TrimSpace(req.ActiveFile) != "" {
		seenFiles[req.ActiveFile] = struct{}{}
		chosenFiles = append(chosenFiles, req.ActiveFile)
	}

	searchTerms := extractSearchTerms(req.Message)
	searchTerms = searchTerms[:minInt(len(searchTerms), 3)]
	for _, term := range searchTerms {
		if result.StepsUsed >= maxSteps {
			break
		}
		matches, err := mcpSearchMatches(ctx, disc, term, 4)
		result.StepsUsed++
		if err != nil || len(matches) == 0 {
			continue
		}
		result.SearchHits = append(result.SearchHits, matches...)
		for _, match := range matches {
			rel := relPathFromSearchHit(match)
			if rel == "" {
				continue
			}
			if _, ok := seenFiles[rel]; ok {
				continue
			}
			seenFiles[rel] = struct{}{}
			chosenFiles = append(chosenFiles, rel)
			if len(chosenFiles) >= maxReads {
				break
			}
		}
	}

	refinedTerms := deriveMCPRefinementTerms(searchTerms, result.SearchHits, chosenFiles)
	result.Refined = refinedTerms
	for _, term := range refinedTerms {
		if result.StepsUsed >= maxSteps || len(chosenFiles) >= maxReads {
			break
		}
		matches, err := mcpSearchMatches(ctx, disc, term, 3)
		result.StepsUsed++
		if err != nil || len(matches) == 0 {
			continue
		}
		result.SearchHits = append(result.SearchHits, matches...)
		for _, match := range matches {
			rel := relPathFromSearchHit(match)
			if rel == "" {
				continue
			}
			if _, ok := seenFiles[rel]; ok {
				continue
			}
			seenFiles[rel] = struct{}{}
			chosenFiles = append(chosenFiles, rel)
			if len(chosenFiles) >= maxReads {
				break
			}
		}
	}

	if len(chosenFiles) < maxReads && strings.TrimSpace(req.ActiveFile) != "" && result.StepsUsed < maxSteps {
		base := filepathBaseWithoutExt(req.ActiveFile)
		if base != "" {
			listed, err := mcpListFiles(ctx, disc, base, 5)
			result.StepsUsed++
			if err == nil {
				for _, rel := range listed {
					if len(chosenFiles) >= maxReads {
						break
					}
					if _, ok := seenFiles[rel]; ok {
						continue
					}
					seenFiles[rel] = struct{}{}
					chosenFiles = append(chosenFiles, rel)
				}
			}
		}
	}

	for _, rel := range chosenFiles {
		if result.StepsUsed >= maxSteps {
			break
		}
		text, err := mcpReadFileText(ctx, disc, rel, 1200)
		result.StepsUsed++
		if err != nil || strings.TrimSpace(text) == "" {
			continue
		}
		result.ReadFiles = append(result.ReadFiles, rel)
	}

	if len(result.SearchHits) > 0 {
		sections = append(sections, "## MCP bounded search hits\n\n- "+strings.Join(uniqueNonEmpty(result.SearchHits), "\n- "))
	}
	if len(result.ReadFiles) > 0 {
		sections = append(sections, "## MCP bounded file reads\n\n- "+strings.Join(uniqueNonEmpty(result.ReadFiles), "\n- "))
	}
	if result.StepsUsed > 0 {
		sections = append(sections, fmt.Sprintf("## MCP bounded loop summary\n\n- Steps used: %d\n- Search hits: %d\n- Files read: %d", result.StepsUsed, len(uniqueNonEmpty(result.SearchHits)), len(uniqueNonEmpty(result.ReadFiles))))
	}
	result.Summary = strings.Join(sections, "\n\n")
	return result
}

func deriveMCPRefinementTerms(seedTerms, searchHits, filePaths []string) []string {
	var out []string
	for _, path := range filePaths {
		base := filepathBaseWithoutExt(path)
		for _, term := range extractSearchTerms(strings.ReplaceAll(base, "-", " ")) {
			out = append(out, term)
		}
	}
	for _, hit := range searchHits {
		for _, part := range strings.Split(hit, ":") {
			for _, term := range extractSearchTerms(part) {
				out = append(out, term)
			}
		}
	}
	seen := map[string]struct{}{}
	var filtered []string
	for _, term := range uniqueNonEmpty(out) {
		if len(term) < 4 {
			continue
		}
		if _, ok := seen[term]; ok {
			continue
		}
		seen[term] = struct{}{}
		filtered = append(filtered, term)
	}
	for _, seed := range seedTerms {
		delete(seen, seed)
	}
	var final []string
	for _, term := range filtered {
		duplicateSeed := false
		for _, seed := range seedTerms {
			if term == seed {
				duplicateSeed = true
				break
			}
		}
		if duplicateSeed {
			continue
		}
		final = append(final, term)
		if len(final) >= 2 {
			break
		}
	}
	return final
}

func filepathBaseWithoutExt(rel string) string {
	base := strings.TrimSpace(rel)
	if base == "" {
		return ""
	}
	base = strings.TrimSuffix(base, filepathExt(base))
	base = strings.TrimSpace(base)
	if idx := strings.LastIndexAny(base, `/\`); idx >= 0 {
		base = base[idx+1:]
	}
	return base
}

func filepathExt(rel string) string {
	if idx := strings.LastIndex(rel, "."); idx >= 0 {
		return rel[idx:]
	}
	return ""
}

func mcpCall(ctx context.Context, baseURL, apiKey, method string, params any, out any) error {
	payload := map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  method,
		"params":  params,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(baseURL, "/")+"/mcp", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	client := &http.Client{Timeout: 8 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("mcp HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}
	var envelope struct {
		Result json.RawMessage `json:"result"`
		Error  *mcpRPCError    `json:"error"`
	}
	if err := json.Unmarshal(respBody, &envelope); err != nil {
		return err
	}
	if envelope.Error != nil {
		return fmt.Errorf("mcp RPC %d: %s", envelope.Error.Code, envelope.Error.Message)
	}
	if out == nil {
		return nil
	}
	return json.Unmarshal(envelope.Result, out)
}

func flattenMCPText(items []struct {
	Type string `json:"type"`
	Text string `json:"text"`
}) string {
	var parts []string
	for _, item := range items {
		if strings.EqualFold(strings.TrimSpace(item.Type), "text") && strings.TrimSpace(item.Text) != "" {
			parts = append(parts, strings.TrimSpace(item.Text))
		}
	}
	return strings.Join(parts, "\n")
}

func hasMCPTool(names []string, target string) bool {
	for _, name := range names {
		if strings.EqualFold(strings.TrimSpace(name), target) {
			return true
		}
	}
	return false
}
