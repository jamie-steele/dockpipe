package application

import (
	"strings"

	"dockpipe/src/lib/domain"
	"dockpipe/src/lib/infrastructure"
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
	if wf.DockerPreflight != nil && !*wf.DockerPreflight {
		return false
	}
	defaultRuntime := infrastructure.NormalizeRuntimeProfileName(strings.TrimSpace(wf.Runtime))
	defaultResolver := strings.TrimSpace(wf.Resolver)
	needsDockerForStep := func(step domain.Step) bool {
		if step.IsHostStep() {
			return false
		}
		rtName := infrastructure.NormalizeRuntimeProfileName(firstNonEmpty(strings.TrimSpace(step.Runtime), defaultRuntime))
		rsName := firstNonEmpty(strings.TrimSpace(step.Resolver), defaultResolver)
		if rtName == "" && rsName == "" {
			return true
		}
		rm, err := infrastructure.LoadIsolationProfile(repoRoot, rtName, rsName)
		if err != nil {
			return true
		}
		ra := domain.FromResolverMap(rm)
		if stepUsesResolverDelegate(&ra) {
			return hostDelegateRequiresDocker(rm)
		}
		return true
	}
	for _, step := range wf.Steps {
		if len(step.RunPaths()) > 0 {
			return true
		}
		if strings.TrimSpace(step.Resolver) != "" || strings.TrimSpace(step.Runtime) != "" || !step.IsHostStep() {
			if needsDockerForStep(step) {
				return true
			}
		}
	}
	if len(wf.Run) > 0 {
		return true
	}
	return false
}
