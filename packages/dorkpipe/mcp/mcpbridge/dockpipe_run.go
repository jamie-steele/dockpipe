package mcpbridge

import (
	"fmt"
	"strings"
)

// dockpipeRunInput is the generic MCP-to-CLI shape. Package selection stays
// generic: it lets any package-owned workflow use the same DockPipe boundary
// without adding product-specific MCP tools.
type dockpipeRunInput struct {
	Workflow   string   `json:"workflow"`
	Package    string   `json:"package"`
	Workdir    string   `json:"workdir"`
	Argv       []string `json:"argv"`
	ResultMode string   `json:"result_mode"`
}

func (in dockpipeRunInput) commandArgs() ([]string, error) {
	switch strings.TrimSpace(in.ResultMode) {
	case "", "summary", "stdout":
	default:
		return nil, fmt.Errorf("result_mode must be summary or stdout")
	}
	wf := strings.TrimSpace(in.Workflow)
	if wf == "" {
		return nil, fmt.Errorf("workflow required")
	}
	wd, err := resolveExecWorkdir(in.Workdir)
	if err != nil {
		return nil, err
	}
	args := []string{"--workflow", wf}
	if pkg := strings.TrimSpace(in.Package); pkg != "" {
		args = append(args, "--package", pkg)
	}
	args = append(args, "--workdir", wd, "--")
	return append(args, in.Argv...), nil
}

func (in dockpipeRunInput) formatResult(stdout, stderr string, code int) ([]byte, bool, error) {
	if strings.TrimSpace(in.ResultMode) == "stdout" {
		return []byte(stdout), code != 0, nil
	}
	summary := fmt.Sprintf("exit_code=%d\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	return []byte(summary), code != 0, nil
}
