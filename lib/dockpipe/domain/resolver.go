package domain

import "strings"

// ResolverAssignments holds merged fields from runtime + resolver profiles (see infrastructure.LoadIsolationProfile).
// Runtime file: templates/core/runtimes/<name> (DOCKPIPE_RUNTIME_*). Resolver file: templates/core/resolvers/<name> (DOCKPIPE_RESOLVER_*).
// Normative model: docs/architecture-model.md.
// See also docs/isolation-layer.md, docs/runtime-architecture.md.
// Contract (not beside a workflow folder):
//   - DOCKPIPE_RUNTIME_IMAGE_TEMPLATE or DOCKPIPE_RESOLVER_TEMPLATE: built-in template name for docker image (see infrastructure.TemplateBuild).
//     Omit when DOCKPIPE_RUNTIME_WORKFLOW / DOCKPIPE_RESOLVER_WORKFLOW or DOCKPIPE_RUNTIME_HOST_SCRIPT / DOCKPIPE_RESOLVER_HOST_ISOLATE is set.
//   - DOCKPIPE_RUNTIME_WORKFLOW or DOCKPIPE_RESOLVER_WORKFLOW: optional; delegate YAML under templates/core/resolvers/<name>/config.yml (or workflows / legacy templates path).
//     After pre-scripts, runs that workflow via the same multi-step runner — preferred over duplicating script paths.
//   - DOCKPIPE_RUNTIME_HOST_SCRIPT or DOCKPIPE_RESOLVER_HOST_ISOLATE: optional; repo-relative script run on the host after pre-scripts instead of docker run.
//     Prefer *_WORKFLOW when a bundled workflow already exists.
//   - DOCKPIPE_RUNTIME_PRE_SCRIPT / DOCKPIPE_RESOLVER_PRE_SCRIPT, DOCKPIPE_RUNTIME_ACTION / DOCKPIPE_RESOLVER_ACTION: optional; override workflow run/act when using --runtime/--resolver without --workflow.
//   - DOCKPIPE_RUNTIME_CMD / DOCKPIPE_RESOLVER_CMD: optional; default CLI binary name for docs / user prompts (not executed by the runner).
//   - DOCKPIPE_RUNTIME_ENV / DOCKPIPE_RESOLVER_ENV: optional; comma-separated env var names typically required for auth (documentation).
//   - DOCKPIPE_RUNTIME_EXPERIMENTAL / DOCKPIPE_RESOLVER_EXPERIMENTAL=1: optional; marks profile as experimental in docs.
//   - DOCKPIPE_RUNTIME_TYPE: runtime.type — execution | ide | agent (classification only; see runtime_kind.go).
type ResolverAssignments struct {
	Template     string
	Workflow     string
	HostIsolate  string
	PreScript    string
	Action       string
	Cmd          string
	EnvHint      string
	Experimental bool
	// RuntimeKind is runtime.type (execution / ide / agent) from DOCKPIPE_RUNTIME_TYPE.
	RuntimeKind string
}

func firstNonEmptyKV(m map[string]string, keys ...string) string {
	for _, k := range keys {
		if s := strings.TrimSpace(m[k]); s != "" {
			return s
		}
	}
	return ""
}

func experimentalKV(m map[string]string, keys ...string) bool {
	for _, k := range keys {
		v := strings.TrimSpace(strings.ToLower(m[k]))
		if v == "1" || v == "true" {
			return true
		}
	}
	return false
}

// FromResolverMap extracts dockpipe isolation profile fields from a parsed assignment map.
// DOCKPIPE_RUNTIME_* keys take precedence over DOCKPIPE_RESOLVER_* when both are set.
func FromResolverMap(m map[string]string) ResolverAssignments {
	exp := experimentalKV(m, "DOCKPIPE_RUNTIME_EXPERIMENTAL", "DOCKPIPE_RESOLVER_EXPERIMENTAL")
	raw := strings.TrimSpace(firstNonEmptyKV(m, "DOCKPIPE_RUNTIME_TYPE"))
	kind := ""
	if raw != "" {
		low := strings.ToLower(raw)
		if IsValidRuntimeKind(low) {
			kind = low
		} else {
			kind = raw // unknown value preserved for forward compatibility
		}
	}
	return ResolverAssignments{
		Template:     firstNonEmptyKV(m, "DOCKPIPE_RUNTIME_IMAGE_TEMPLATE", "DOCKPIPE_RESOLVER_TEMPLATE"),
		Workflow:     firstNonEmptyKV(m, "DOCKPIPE_RUNTIME_WORKFLOW", "DOCKPIPE_RESOLVER_WORKFLOW"),
		HostIsolate:  firstNonEmptyKV(m, "DOCKPIPE_RUNTIME_HOST_SCRIPT", "DOCKPIPE_RESOLVER_HOST_ISOLATE"),
		PreScript:    firstNonEmptyKV(m, "DOCKPIPE_RUNTIME_PRE_SCRIPT", "DOCKPIPE_RESOLVER_PRE_SCRIPT"),
		Action:       firstNonEmptyKV(m, "DOCKPIPE_RUNTIME_ACTION", "DOCKPIPE_RESOLVER_ACTION"),
		Cmd:          firstNonEmptyKV(m, "DOCKPIPE_RUNTIME_CMD", "DOCKPIPE_RESOLVER_CMD"),
		EnvHint:      firstNonEmptyKV(m, "DOCKPIPE_RUNTIME_ENV", "DOCKPIPE_RESOLVER_ENV"),
		Experimental: exp,
		RuntimeKind:  kind,
	}
}

// EnvVarNamesFromHint parses DOCKPIPE_RUNTIME_ENV / DOCKPIPE_RESOLVER_ENV values: comma-separated
// names, trimmed; empty segments are dropped.
func EnvVarNamesFromHint(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	var out []string
	for _, part := range strings.Split(s, ",") {
		k := strings.TrimSpace(part)
		if k != "" {
			out = append(out, k)
		}
	}
	return out
}
