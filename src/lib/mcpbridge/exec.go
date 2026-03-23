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

	"dockpipe/src/lib/dockpipe/infrastructure"
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
