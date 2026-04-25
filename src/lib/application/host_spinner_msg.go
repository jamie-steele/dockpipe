package application

import "strings"

// hostSpinnerLabel is the status line for StartLineSpinner while a host script runs.
func hostSpinnerLabel(scriptPath string) string {
	if strings.Contains(scriptPath, "clone-worktree") {
		return "Preparing worktree…"
	}
	return "Running host setup…"
}
