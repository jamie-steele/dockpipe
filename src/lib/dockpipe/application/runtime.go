package application

import (
	"fmt"
	"strings"

	"dockpipe/src/lib/dockpipe/domain"
)

// EffectiveRuntimeProfileName returns the isolation **runtime** profile name (templates/core/runtimes/<name>).
// Precedence: CLI --runtime, then workflow runtime / default_runtime.
func EffectiveRuntimeProfileName(opts *CliOpts, wf *domain.Workflow, stepsMode bool) string {
	if opts != nil {
		if s := strings.TrimSpace(opts.Runtime); s != "" {
			return s
		}
	}
	if wf == nil {
		return ""
	}
	if stepsMode {
		if s := strings.TrimSpace(wf.Runtime); s != "" {
			return s
		}
		return strings.TrimSpace(wf.DefaultRuntime)
	}
	if s := strings.TrimSpace(wf.Runtime); s != "" {
		return s
	}
	return strings.TrimSpace(wf.DefaultRuntime)
}

// EffectiveResolverProfileName returns the **resolver** (tool adapter) profile name (templates/core/resolvers/<name>).
// Precedence: CLI --resolver, then workflow resolver / default_resolver.
func EffectiveResolverProfileName(opts *CliOpts, wf *domain.Workflow, stepsMode bool) string {
	if opts != nil {
		if s := strings.TrimSpace(opts.Resolver); s != "" {
			return s
		}
	}
	if wf == nil {
		return ""
	}
	if stepsMode {
		if s := strings.TrimSpace(wf.Resolver); s != "" {
			return s
		}
		return strings.TrimSpace(wf.DefaultResolver)
	}
	if s := strings.TrimSpace(wf.Resolver); s != "" {
		return s
	}
	return strings.TrimSpace(wf.DefaultResolver)
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

// ValidateRuntimeAllowlist errors if runtimes: is non-empty and the effective runtime profile name is not listed.
// Use the runtime substrate name (e.g. docker, cli), not ProfileLabelForEnv (which prefers resolver for display).
func ValidateRuntimeAllowlist(wf *domain.Workflow, runtimeName string) error {
	if wf == nil || len(wf.Runtimes) == 0 || runtimeName == "" {
		return nil
	}
	for _, s := range wf.Runtimes {
		if strings.TrimSpace(s) == strings.TrimSpace(runtimeName) {
			return nil
		}
	}
	return fmt.Errorf("runtime %q is not allowed by this workflow (runtimes: %v)", runtimeName, wf.Runtimes)
}
