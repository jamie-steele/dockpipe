package application

import (
	"fmt"
	"strings"
)

// workflowTaskLines prints a short task-oriented header for --workflow runs.
// The ASCII banner carries the Run → Isolate → Act slogan; logs stay factual.
// displayName: wf.Name if set, else the template folder name (--workflow value).
// stepCount > 0 appends " (N steps)" for multi-step workflows.
func workflowTaskLines(displayName string, description string, stepCount int) string {
	name := strings.TrimSpace(displayName)
	if name == "" {
		name = "(workflow)"
	}
	var b strings.Builder
	if stepCount > 0 {
		fmt.Fprintf(&b, "[dockpipe] %s (%d steps)\n", name, stepCount)
	} else {
		fmt.Fprintf(&b, "[dockpipe] %s\n", name)
	}
	if d := strings.TrimSpace(description); d != "" {
		fmt.Fprintf(&b, "[dockpipe] %s\n", d)
	}
	return b.String()
}
