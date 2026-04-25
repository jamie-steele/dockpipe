package application

import (
	"dockpipe/src/lib/domain"
)

// applyWorkflowCapabilityIsolation is a no-op: workflows declare runtime/resolver explicitly (no capability indirection).
func applyWorkflowCapabilityIsolation(workdir, repoRoot string, wf *domain.Workflow, rtName, rsName string) (string, string, error) {
	return rtName, rsName, nil
}

// WorkflowNeedsDockerReachableResolved extends domain.NeedsDockerReachable with optional capability-based preflight (disabled).
func WorkflowNeedsDockerReachableResolved(wf *domain.Workflow, workdir, repoRoot string) bool {
	if wf == nil {
		return false
	}
	if wf.NeedsDockerReachable() {
		return true
	}
	if wf.DockerPreflight != nil && !*wf.DockerPreflight {
		return false
	}
	return false
}
