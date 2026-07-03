package application

import (
	"strings"
	"time"

	"dockpipe/src/lib/infrastructure"
)

// hostSpinnerLabel is the status line for StartLineSpinner while a host script runs.
func hostSpinnerLabel(scriptPath string) string {
	if strings.Contains(scriptPath, "clone-worktree") {
		return "Preparing worktree…"
	}
	return "Running host setup…"
}

func hostSetupOperationOptions() infrastructure.OperationOptions {
	return infrastructure.OperationOptions{
		Spinner:       false,
		ProgressEvery: 5 * time.Second,
	}
}
