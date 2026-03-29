package application

import (
	"fmt"
	"strings"

	"dockpipe/src/lib/domain"
	"dockpipe/src/lib/infrastructure"
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

// effectiveRuntimesAllowlist returns the substrate names allowed for --runtime overrides.
// Explicit runtimes: wins; otherwise, if runtime and/or default_runtime are set, the allowlist is
// those non-empty values (deduped), so you do not need runtimes: [dockerimage] when runtime: dockerimage alone (legacy YAML may still say runtime: cli; it normalizes to dockerimage).
func effectiveRuntimesAllowlist(wf *domain.Workflow) []string {
	if wf == nil {
		return nil
	}
	if len(wf.Runtimes) > 0 {
		return wf.Runtimes
	}
	var out []string
	seen := map[string]struct{}{}
	add := func(s string) {
		s = strings.TrimSpace(s)
		if s == "" {
			return
		}
		if _, ok := seen[s]; ok {
			return
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	add(wf.Runtime)
	add(wf.DefaultRuntime)
	if len(out) == 0 {
		return nil
	}
	return out
}

// ValidateRuntimeAllowlist errors if the effective runtime profile name is not in the allowlist.
// The allowlist is explicit runtimes:, or implicit from runtime / default_runtime when runtimes: is omitted.
// Names are compared after NormalizeRuntimeProfileName (docker → dockerimage, legacy host names → dockerimage, etc.).
func ValidateRuntimeAllowlist(wf *domain.Workflow, runtimeName string) error {
	if wf == nil || runtimeName == "" {
		return nil
	}
	allow := effectiveRuntimesAllowlist(wf)
	if len(allow) == 0 {
		return nil
	}
	want := infrastructure.NormalizeRuntimeProfileName(runtimeName)
	for _, s := range allow {
		if infrastructure.NormalizeRuntimeProfileName(strings.TrimSpace(s)) == want {
			return nil
		}
	}
	return fmt.Errorf("runtime %q is not allowed by this workflow (runtimes: %v)", runtimeName, allow)
}
