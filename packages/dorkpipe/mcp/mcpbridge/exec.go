package mcpbridge

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"dockpipe/src/lib/infrastructure"
)

func dockpipePath() string {
	if v := strings.TrimSpace(os.Getenv("DOCKPIPE_BIN")); v != "" {
		return v
	}
	p, err := exec.LookPath("dockpipe")
	if err != nil {
		return "dockpipe"
	}
	return p
}

func dorkpipePath() string {
	if v := strings.TrimSpace(os.Getenv("DORKPIPE_BIN")); v != "" {
		return v
	}
	p, err := exec.LookPath("dorkpipe")
	if err != nil {
		return "dorkpipe"
	}
	return p
}

func runDockpipe(ctx context.Context, args []string) (stdout, stderr string, exitCode int, err error) {
	if err := CheckMCPBinPathsAreAbsolute(); err != nil {
		return "", "", -1, err
	}
	cmd := exec.CommandContext(ctx, dockpipePath(), args...)
	var outb, errb bytes.Buffer
	cmd.Stdout = &outb
	cmd.Stderr = &errb
	runErr := cmd.Run()
	stdout = outb.String()
	stderr = errb.String()
	if runErr != nil {
		if ee, ok := runErr.(*exec.ExitError); ok {
			return stdout, stderr, ee.ExitCode(), nil
		}
		return stdout, stderr, -1, runErr
	}
	return stdout, stderr, 0, nil
}

func runDorkpipe(ctx context.Context, args []string) (stdout, stderr string, exitCode int, err error) {
	if err := CheckMCPBinPathsAreAbsolute(); err != nil {
		return "", "", -1, err
	}
	cmd := exec.CommandContext(ctx, dorkpipePath(), args...)
	var outb, errb bytes.Buffer
	cmd.Stdout = &outb
	cmd.Stderr = &errb
	runErr := cmd.Run()
	stdout = outb.String()
	stderr = errb.String()
	if runErr != nil {
		if ee, ok := runErr.(*exec.ExitError); ok {
			return stdout, stderr, ee.ExitCode(), nil
		}
		return stdout, stderr, -1, runErr
	}
	return stdout, stderr, 0, nil
}

// absWorkdir ensures workdir is absolute; if relative, resolves from cwd.
func absWorkdir(wd string) (string, error) {
	wd = strings.TrimSpace(wd)
	if wd == "" {
		return os.Getwd()
	}
	if filepath.IsAbs(wd) {
		return filepath.Clean(wd), nil
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return filepath.Abs(filepath.Join(cwd, wd))
}

func mcpEnvOptOut(name string) bool {
	v := strings.TrimSpace(strings.ToLower(os.Getenv(name)))
	return v == "0" || v == "false" || v == "no" || v == "off"
}

// restrictExecWorkdirToRepo is ON by default so dockpipe.run / dorkpipe.run_spec workdirs
// stay under the resolved repo root. Set DOCKPIPE_MCP_RESTRICT_WORKDIR=0 to disable.
func restrictExecWorkdirToRepo() bool {
	return !mcpEnvOptOut("DOCKPIPE_MCP_RESTRICT_WORKDIR")
}

// resolveExecWorkdir is used for MCP exec tools when restriction is on (default):
// empty workdir defaults to repo root; any other path must lie under that root.
func resolveExecWorkdir(inWorkdir string) (string, error) {
	if !restrictExecWorkdirToRepo() {
		return absWorkdir(inWorkdir)
	}
	rr, err := infrastructure.RepoRoot()
	if err != nil {
		return "", err
	}
	root := filepath.Clean(rr)
	if strings.TrimSpace(inWorkdir) == "" {
		return root, nil
	}
	awd, err := absWorkdir(inWorkdir)
	if err != nil {
		return "", err
	}
	if err := CheckAbsolutePathUnderRepoRoot(awd); err != nil {
		return "", fmt.Errorf("workdir must stay under repo root (default; set DOCKPIPE_MCP_RESTRICT_WORKDIR=0 to allow any path): %w", err)
	}
	return awd, nil
}

// CheckMCPBinPathsAreAbsolute is ON by default: DOCKPIPE_BIN / DORKPIPE_BIN must be absolute.
// Set DOCKPIPE_MCP_REQUIRE_ABSOLUTE_BIN=0 to allow PATH lookup to a non-absolute name (weaker).
func CheckMCPBinPathsAreAbsolute() error {
	if mcpEnvOptOut("DOCKPIPE_MCP_REQUIRE_ABSOLUTE_BIN") {
		return nil
	}
	dp := dockpipePath()
	if !filepath.IsAbs(dp) {
		return fmt.Errorf("DOCKPIPE_MCP_REQUIRE_ABSOLUTE_BIN=1: dockpipe path must be absolute (got %q); set DOCKPIPE_BIN=/path/to/dockpipe", dp)
	}
	dr := dorkpipePath()
	if !filepath.IsAbs(dr) {
		return fmt.Errorf("dorkpipe path must be absolute (default; set DOCKPIPE_MCP_REQUIRE_ABSOLUTE_BIN=0 to relax): got %q — set DORKPIPE_BIN=/path/to/dorkpipe", dr)
	}
	return nil
}

func toolResultJSON(text string, isError bool) []byte {
	type contentBlock struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	type callResult struct {
		Content []contentBlock `json:"content"`
		IsError bool           `json:"isError"`
	}
	out, _ := json.Marshal(callResult{
		Content: []contentBlock{{Type: "text", Text: text}},
		IsError: isError,
	})
	return out
}

type dorkpipeExecSummary struct {
	ExitCode     int            `json:"exit_code"`
	FinalEvent   map[string]any `json:"final_event,omitempty"`
	ReadyToApply map[string]any `json:"ready_to_apply,omitempty"`
	StreamedText string         `json:"streamed_text,omitempty"`
	Events       []string       `json:"events,omitempty"`
	Stderr       []string       `json:"stderr,omitempty"`
}

func runDorkpipeEventStream(ctx context.Context, args []string) (*dorkpipeExecSummary, error) {
	stdout, stderr, exitCode, err := runDorkpipe(ctx, args)
	if err != nil {
		return nil, err
	}
	summary := &dorkpipeExecSummary{
		ExitCode: exitCode,
		Stderr:   compactNonEmptyLines(stderr),
	}
	for _, rawLine := range strings.Split(stdout, "\n") {
		line := strings.TrimSpace(rawLine)
		if line == "" {
			continue
		}
		var event map[string]any
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			summary.Events = append(summary.Events, line)
			continue
		}
		if display, _ := event["display_text"].(string); strings.TrimSpace(display) != "" {
			summary.Events = append(summary.Events, strings.TrimSpace(display))
		}
		eventType, _ := event["type"].(string)
		switch strings.TrimSpace(eventType) {
		case "model_stream":
			if metadata, _ := event["metadata"].(map[string]any); metadata != nil {
				if piece, _ := metadata["text"].(string); piece != "" {
					summary.StreamedText += piece
				}
			}
		case "ready_to_apply":
			if metadata, _ := event["metadata"].(map[string]any); metadata != nil {
				summary.ReadyToApply = metadata
			}
		case "done", "error":
			summary.FinalEvent = event
		}
	}
	return summary, nil
}

func compactNonEmptyLines(text string) []string {
	var lines []string
	for _, raw := range strings.Split(text, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		lines = append(lines, line)
	}
	return lines
}

func normalizeRepoHintPath(userPath string) (string, error) {
	userPath = strings.TrimSpace(userPath)
	if userPath == "" {
		return "", nil
	}
	absPath, err := ResolvePathUnderRepoRoot(userPath)
	if err != nil {
		return "", err
	}
	repoRoot, err := infrastructure.RepoRoot()
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(filepath.Clean(repoRoot), filepath.Clean(absPath))
	if err != nil {
		return "", err
	}
	return filepath.ToSlash(rel), nil
}

func normalizeRepoHintPaths(paths []string) ([]string, error) {
	var out []string
	seen := map[string]struct{}{}
	for _, raw := range paths {
		rel, err := normalizeRepoHintPath(raw)
		if err != nil {
			return nil, err
		}
		if rel == "" {
			continue
		}
		if _, ok := seen[rel]; ok {
			continue
		}
		seen[rel] = struct{}{}
		out = append(out, rel)
	}
	return out, nil
}

func normalizeAttachmentPaths(paths []string) ([]string, error) {
	var out []string
	seen := map[string]struct{}{}
	for _, raw := range paths {
		target := strings.TrimSpace(raw)
		if target == "" {
			continue
		}
		absPath, err := resolveRepoBoundAbsolutePath(target)
		if err != nil {
			return nil, err
		}
		if _, ok := seen[absPath]; ok {
			continue
		}
		seen[absPath] = struct{}{}
		out = append(out, absPath)
	}
	return out, nil
}

func resolveRepoBoundAbsolutePath(userPath string) (string, error) {
	userPath = strings.TrimSpace(userPath)
	if userPath == "" {
		return "", fmt.Errorf("path is empty")
	}
	if filepath.IsAbs(userPath) {
		cleaned := filepath.Clean(userPath)
		if err := CheckAbsolutePathUnderRepoRoot(cleaned); err != nil {
			return "", err
		}
		return cleaned, nil
	}
	return ResolvePathUnderRepoRoot(userPath)
}
