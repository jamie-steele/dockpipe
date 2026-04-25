package application

import (
	"strings"

	"dockpipe/src/lib/domain"
)

// EffectiveRuntimeProfileName returns the isolation **runtime** profile name (templates/core/runtimes/<name>).
// Precedence: CLI --runtime, then workflow runtime.
func EffectiveRuntimeProfileName(opts *CliOpts, wf *domain.Workflow, stepsMode bool) string {
	if opts != nil {
		if s := strings.TrimSpace(opts.Runtime); s != "" {
			return s
		}
	}
	if wf == nil {
		return ""
	}
	return strings.TrimSpace(wf.Runtime)
}

// EffectiveResolverProfileName returns the **resolver** (tool adapter) profile name (templates/core/resolvers/<name>).
// Precedence: CLI --resolver, then workflow resolver.
func EffectiveResolverProfileName(opts *CliOpts, wf *domain.Workflow, stepsMode bool) string {
	if opts != nil {
		if s := strings.TrimSpace(opts.Resolver); s != "" {
			return s
		}
	}
	if wf == nil {
		return ""
	}
	return strings.TrimSpace(wf.Resolver)
}

// EffectiveLegacyIsolateName returns workflow isolate: when no explicit runtime/resolver names were set.
// Used to pair runtimes/<name> + resolvers/<name> for legacy single-field workflows.
func EffectiveLegacyIsolateName(wf *domain.Workflow) string {
	if wf == nil {
		return ""
	}
	return strings.TrimSpace(wf.Isolate)
}

// ProfileLabelForEnv prefers resolver name for branch/env display, then runtime name.
func ProfileLabelForEnv(runtimeName, resolverName string) string {
	if s := strings.TrimSpace(resolverName); s != "" {
		return s
	}
	return strings.TrimSpace(runtimeName)
}

// ValidateRuntimeAllowlist is a no-op in the simplified authored workflow model.
func ValidateRuntimeAllowlist(wf *domain.Workflow, runtimeName string) error {
	return nil
}
